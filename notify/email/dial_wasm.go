//go:build wasm
// +build wasm

/*
 * Copyright (c) 2022, MegaEase
 * All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package email

import (
	"crypto/tls"
	"errors"
	"io"
	"net"
	"syscall/js"
	"time"
)

// Dial dials using cloudflare:sockets and wraps it in net.Conn and tls.Client
func Dial(network, addr string, config *tls.Config) (net.Conn, error) {
	// 1. Call JS connect(addr)
	connectFunc := js.Global().Get("easeprobe_connect")
	if connectFunc.IsUndefined() || connectFunc.IsNull() {
		return nil, errors.New("easeprobe_connect function not found in JS environment")
	}

	// connect(addr, options)
	// We might need { secureTransport: "starttls" } or similar if we want direct TLS from Cloudflare?
	// But tls.Client will handle TLS. So we want raw TCP.
	// connect(addr) returns a Socket.

	// Wait, calling connect() is synchronous in JS (returns Socket object immediately), but opening is async.
	// We need to wait for opened?
	// Socket has .opened Promise.
	// Or we can just start writing/reading.

	socketVal := connectFunc.Invoke(addr)
	if socketVal.IsNull() || socketVal.IsUndefined() {
		return nil, errors.New("easeprobe_connect returned null/undefined")
	}

	// Wrap in WasmConn
	wc := &WasmConn{
		socket:   socketVal,
		readable: socketVal.Get("readable"),
		writable: socketVal.Get("writable"),
	}

	// We should probably wait for connection to be established before returning?
	// net.Dial blocks until connected.
	// socket.opened is a promise.
	openedPromise := socketVal.Get("opened")
	awaitPromise(openedPromise) // This blocks until resolved or rejected

	// Wrap in TLS client
	tlsConn := tls.Client(wc, config)
	return tlsConn, nil
}

// Helper to await promise
func awaitPromise(promise js.Value) (js.Value, error) {
	resultChan := make(chan js.Value, 1)
	errorChan := make(chan error, 1)

	success := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		var res js.Value
		if len(args) > 0 {
			res = args[0]
		}
		resultChan <- res
		return nil
	})
	defer success.Release()

	failure := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		var errStr string
		if len(args) > 0 {
			errStr = args[0].String()
		} else {
			errStr = "Unknown promise rejection"
		}
		errorChan <- errors.New(errStr)
		return nil
	})
	defer failure.Release()

	promise.Call("then", success).Call("catch", failure)

	select {
	case res := <-resultChan:
		return res, nil
	case err := <-errorChan:
		return js.Undefined(), err
	}
}

// WasmConn implements net.Conn over Cloudflare Socket
type WasmConn struct {
	socket   js.Value
	readable js.Value
	writable js.Value
	reader   *io.PipeReader
	writer   *io.PipeWriter // Used if we need pipe, but here we read from JS

	// Buffering for Read
	readBuf []byte
}

func (c *WasmConn) Read(b []byte) (n int, err error) {
	// If we have buffered data, return it
	if len(c.readBuf) > 0 {
		n = copy(b, c.readBuf)
		c.readBuf = c.readBuf[n:]
		return n, nil
	}

	// Read from JS readable stream
	// readable.getReader().read()
	// But getReader() locks the stream. We should keep the reader.
	// Let's modify WasmConn to hold the reader.

	// Assuming we call getReader() once.
	// c.jsReader = c.readable.Call("getReader")
	// result = await c.jsReader.Call("read")
	// result is { value: Uint8Array, done: bool }

	// Implementation needed.
	// For now, minimal stub to pass compilation, but we need real logic.

	// Let's implement lazy reader init
	if c.reader == nil {
		// We need to spin up a goroutine that reads from JS and writes to a pipe
		pr, pw := io.Pipe()
		c.reader = pr
		// Start pumper
		go c.pumpRead(pw)
	}

	return c.reader.Read(b)
}

func (c *WasmConn) pumpRead(pw *io.PipeWriter) {
	// getReader
	jsReader := c.readable.Call("getReader")
	for {
		// read() returns promise
		promise := jsReader.Call("read")
		val, err := awaitPromise(promise)
		if err != nil {
			pw.CloseWithError(err)
			return
		}

		done := val.Get("done").Bool()
		if done {
			pw.Close() // EOF
			return
		}

		chunk := val.Get("value") // Uint8Array
		length := chunk.Get("length").Int()
		buf := make([]byte, length)
		js.CopyBytesToGo(buf, chunk)

		// Write to pipe
		_, err = pw.Write(buf)
		if err != nil {
			// Pipe closed or error
			return
		}
	}
}

func (c *WasmConn) Write(b []byte) (n int, err error) {
	// Writable stream
	// writer = writable.getWriter()
	// await writer.write(Uint8Array)
	// writer.releaseLock()

	// Or keep writer open?
	// c.jsWriter = c.writable.Call("getWriter")

	// Create Uint8Array
	uint8Array := js.Global().Get("Uint8Array").New(len(b))
	js.CopyBytesToJS(uint8Array, b)

	writer := c.writable.Call("getWriter")
	promise := writer.Call("write", uint8Array)
	_, err = awaitPromise(promise)
	writer.Call("releaseLock")

	if err != nil {
		return 0, err
	}
	return len(b), nil
}

func (c *WasmConn) Close() error {
	c.socket.Call("close")
	return nil
}

func (c *WasmConn) LocalAddr() net.Addr { return &DummyAddr{} }
func (c *WasmConn) RemoteAddr() net.Addr { return &DummyAddr{} }
func (c *WasmConn) SetDeadline(t time.Time) error { return nil }
func (c *WasmConn) SetReadDeadline(t time.Time) error { return nil }
func (c *WasmConn) SetWriteDeadline(t time.Time) error { return nil }

type DummyAddr struct{}
func (a *DummyAddr) Network() string { return "tcp" }
func (a *DummyAddr) String() string { return "wasm" }
