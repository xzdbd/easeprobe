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

package tcp

import (
	"fmt"
	"syscall/js"

	log "github.com/sirupsen/logrus"
)

// DoProbe return the checking result
func (t *TCP) DoProbe() (bool, string) {
	log.Debugf("WASM TCP Probe checking: %s", t.Host)
	checkFunc := js.Global().Get("easeprobe_tcp_check")
	if checkFunc.IsUndefined() || checkFunc.IsNull() {
		return false, "Error: easeprobe_tcp_check function not found in JS environment"
	}

	timeoutMs := t.Timeout().Milliseconds()
	if timeoutMs == 0 {
		timeoutMs = 5000 // Default 5s
	}

	// Call JS function: easeprobe_tcp_check(host, timeoutMs) -> Promise
	promise := checkFunc.Invoke(t.Host, timeoutMs)

	// Await the promise
	resultChan := make(chan string, 1)
	errorChan := make(chan string, 1)

	successFunc := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		// args[0] might be message
		msg := "TCP Connection Established Successfully!"
		if len(args) > 0 && args[0].Type() == js.TypeString {
			msg = args[0].String()
		}
		resultChan <- msg
		return nil
	})
	defer successFunc.Release()

	failureFunc := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		msg := "Unknown Error"
		if len(args) > 0 {
			msg = args[0].String()
		}
		errorChan <- msg
		return nil
	})
	defer failureFunc.Release()

	promise.Call("then", successFunc).Call("catch", failureFunc)

	select {
	case msg := <-resultChan:
		return true, msg
	case err := <-errorChan:
		log.Errorf("WASM TCP Error: %s", err)
		return false, fmt.Sprintf("Error: %s", err)
	}
}
