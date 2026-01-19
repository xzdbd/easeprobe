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

package notify

import (
	"github.com/megaease/easeprobe/notify/bark"
	"github.com/megaease/easeprobe/notify/email"
	"github.com/megaease/easeprobe/notify/log"
)

//Config is the notify configuration (Slim for WASM)
type Config struct {
	Log      []log.NotifyConfig      `yaml:"log" json:"log"`
	Email    []email.NotifyConfig    `yaml:"email" json:"email"`
	Bark     []bark.NotifyConfig     `yaml:"bark" json:"bark"`
	// Removed others to save space
}
