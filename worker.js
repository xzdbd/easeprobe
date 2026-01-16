// Import sockets from cloudflare:sockets
// Note: This requires the "cloudflare:sockets" capability.
import { connect } from 'cloudflare:sockets';

export default {
  async fetch(request, env, ctx) {
    return handleRequest(request, env);
  },
  async scheduled(event, env, ctx) {
    ctx.waitUntil(handleScheduled(event, env));
  }
};

// Polyfill process.env for Go's os.Environ()
if (!globalThis.process) {
  globalThis.process = {
    env: {}
  };
} else if (!globalThis.process.env) {
  globalThis.process.env = {};
}

// Implement tcp check function for WASM
globalThis.easeprobe_tcp_check = async function(host, timeout) {
  console.log(`[JS] Checking TCP connection to ${host} with timeout ${timeout}ms`);
  try {
    const socket = connect(host);
    const writer = socket.writable.getWriter();
    const reader = socket.readable.getReader();

    // We just want to check connection.
    // If connect() doesn't throw, we assume connection is established?
    // Actually connect() returns a Socket object immediately. The connection is established when we try to write/read or via opened promise?
    // Cloudflare docs say: "connect() returns a Socket".
    // "socket.opened" is a promise that resolves when the socket is ready.

    const timeoutPromise = new Promise((_, reject) =>
      setTimeout(() => reject(new Error("Connection timed out")), timeout)
    );

    await Promise.race([
      socket.opened,
      timeoutPromise
    ]);

    // Connection successful
    console.log(`[JS] TCP connection to ${host} successful`);
    // Close the socket
    await socket.close();

    return "TCP Connection Established Successfully!";
  } catch (e) {
    console.error(`[JS] TCP connection to ${host} failed: ${e.message || e}`);
    throw e.message || e.toString();
  }
};

const go = new Go();
let inst;

async function init(env) {
  if (inst) return;

  // Populate process.env with Worker environment variables
  if (env) {
    // Ensure env exists again just in case
    if (!globalThis.process.env) {
      globalThis.process.env = {};
    }

    for (const [key, value] of Object.entries(env)) {
      if (typeof value === 'string') {
        globalThis.process.env[key] = value;
      }
    }
  }

  // Retrieve WASM module
  // It is imported at the top level as WASM_MODULE
  if (typeof WASM_MODULE === 'undefined') {
     throw new Error("WASM module not found. Import failed.");
  }

  const importObject = go.importObject;

  const result = await WebAssembly.instantiate(WASM_MODULE, importObject);

  if (result.instance) {
    inst = result.instance;
  } else {
    inst = result;
  }

  go.run(inst);
}

// Convert config string to JSON string if it's YAML
function parseConfig(str) {
  if (!str) return str;
  // If it starts with '{', assume it is JSON
  if (str.trim().startsWith('{')) {
    return str;
  }
  // Otherwise, try to parse as YAML using js-yaml (bundled)
  if (typeof jsyaml !== 'undefined') {
    try {
      const obj = jsyaml.load(str);
      return JSON.stringify(obj);
    } catch (e) {
      console.error("Failed to parse YAML config:", e);
      return str; // Return original string, Go unmarshal might fail or report error
    }
  }
  return str;
}

async function handleRequest(request, env) {
  try {
    await init(env);

    // Default config from environment variable
    let configStr = "";
    if (env && env.CONFIG_YAML) {
       configStr = env.CONFIG_YAML;
    }

    // Or allow passing config via POST body for testing
    if (request.method === "POST") {
       configStr = await request.text();
    }

    if (!configStr) {
      return new Response("Configuration not found. Set CONFIG_YAML env var or POST config.", { status: 500 });
    }

    // Convert YAML to JSON if needed
    const jsonConfigStr = parseConfig(configStr);

    const result = await check(jsonConfigStr, false); // false for dryRun

    return new Response(JSON.stringify(result, null, 2), {
      headers: { 'content-type': 'application/json' },
    });
  } catch (e) {
    return new Response(e.stack || e.toString(), { status: 500 });
  }
}

async function handleScheduled(event, env) {
  try {
    await init(env);

    let configStr = "";
    if (env && env.CONFIG_YAML) {
       configStr = env.CONFIG_YAML;
    }

    if (!configStr) {
      console.error("Configuration not found. Set CONFIG_YAML env var.");
      return;
    }

    const jsonConfigStr = parseConfig(configStr);

    const result = await check(jsonConfigStr, false);

    console.log(JSON.stringify(result));

  } catch (e) {
    console.error(e.stack || e.toString());
  }
}
