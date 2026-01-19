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

// Global persistent status map for Edge-Triggered logic
let STATUS_MAP = {};

// Implement tcp check function for WASM (TCP Probe)
globalThis.easeprobe_tcp_check = async function(host, timeout) {
  console.log(`[JS] Checking TCP connection to ${host} with timeout ${timeout}ms`);
  try {
    const socket = connect(host);

    // Check connection via opened promise
    const timeoutPromise = new Promise((_, reject) =>
      setTimeout(() => reject(new Error("Connection timed out")), timeout)
    );

    await Promise.race([
      socket.opened,
      timeoutPromise
    ]);

    // Connection successful
    console.log(`[JS] TCP connection to ${host} successful`);
    await socket.close();

    return "TCP Connection Established Successfully!";
  } catch (e) {
    console.error(`[JS] TCP connection to ${host} failed: ${e.message || e}`);
    throw e.message || e.toString();
  }
};

// Implement connect function for WASM (Email Notification)
// Returns the socket object directly to Go
globalThis.easeprobe_connect = function(addr) {
  console.log(`[JS] Connecting to ${addr} for Email/TCP`);
  // Note: we don't await opened here, Go will await it.
  try {
    const socket = connect(addr);
    return socket;
  } catch (e) {
    console.error(`[JS] Connect failed: ${e}`);
    return null;
  }
};

const go = new Go();
let inst;

async function init(env) {
  if (inst) return;

  // Populate process.env with Worker environment variables
  if (env) {
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
  if (str.trim().startsWith('{')) {
    return str;
  }
  if (typeof jsyaml !== 'undefined') {
    try {
      const obj = jsyaml.load(str);
      return JSON.stringify(obj);
    } catch (e) {
      console.error("Failed to parse YAML config:", e);
      return str;
    }
  }
  return str;
}

async function handleRequest(request, env) {
  try {
    await init(env);

    let configStr = "";
    if (env && env.CONFIG_YAML) {
       configStr = env.CONFIG_YAML;
    }

    if (request.method === "POST") {
       configStr = await request.text();
    }

    if (!configStr) {
      return new Response("Configuration not found. Set CONFIG_YAML env var or POST config.", { status: 500 });
    }

    const jsonConfigStr = parseConfig(configStr);
    const prevStatusJson = JSON.stringify(STATUS_MAP);

    // Call Check: check(config, dryRun, prevStatus)
    const result = await check(jsonConfigStr, false, prevStatusJson);

    // Update STATUS_MAP
    if (result && result.status) {
      STATUS_MAP = result.status;
    }

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
    const prevStatusJson = JSON.stringify(STATUS_MAP);

    const result = await check(jsonConfigStr, false, prevStatusJson);

    if (result && result.status) {
      STATUS_MAP = result.status;
    }

    console.log(JSON.stringify(result));

  } catch (e) {
    console.error(e.stack || e.toString());
  }
}
