//go:build !wasm
// +build !wasm

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
	"net"

	log "github.com/sirupsen/logrus"
)

// DoProbe return the checking result
func (t *TCP) DoProbe() (bool, string) {
	conn, err := net.DialTimeout("tcp", t.Host, t.Timeout())
	status := true
	message := ""
	if err != nil {
		message = fmt.Sprintf("Error: %v", err)
		log.Errorf("error: %v", err)
		status = false
	} else {
		message = "TCP Connection Established Successfully!"
		conn.Close()
	}
	return status, message
}
