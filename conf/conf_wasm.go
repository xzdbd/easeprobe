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

package conf

import (
	"encoding/json"
	"os"
	"time"

	"github.com/megaease/easeprobe/global"
	"github.com/megaease/easeprobe/notify"
	"github.com/megaease/easeprobe/probe/http"
	"github.com/megaease/easeprobe/probe/tcp"
	log "github.com/sirupsen/logrus"
)

// Conf is Probe configuration (Slim for WASM)
type Conf struct {
	HTTP     []http.HTTP     `yaml:"http" json:"http"`
	TCP      []tcp.TCP       `yaml:"tcp" json:"tcp"`
	Notify   notify.Config   `yaml:"notify" json:"notify"`
	Settings Settings        `yaml:"settings" json:"settings"`
}

// NewFromBytes read the configuration from bytes
// Note: For WASM, we expect JSON format because yaml parser is heavy.
func NewFromBytes(y []byte) (Conf, error) {
	c := Conf{
		HTTP:  []http.HTTP{},
		TCP:   []tcp.TCP{},
		Notify: notify.Config{},
		Settings: Settings{
			LogFile:    "",
			LogLevel:   LogLevel{log.InfoLevel},
			TimeFormat: "2006-01-02 15:04:05 UTC",
			Probe: Probe{
				Interval: time.Second * 60,
				Timeout:  time.Second * 10,
			},
			Notify: Notify{
				Retry: global.Retry{
					Times:    3,
					Interval: time.Second * 5,
				},
				Dry: false,
			},
			SLAReport: SLAReport{
				Schedule: Daily,
				Time:     "00:00",
				Debug:    false,
			},
			logfile: nil,
		},
	}

	// os.ExpandEnv for WASM depends on whether we polyfilled it in JS.
	y = []byte(os.ExpandEnv(string(y)))

	// Use JSON Unmarshal instead of YAML to save size
	err := json.Unmarshal(y, &c)
	if err != nil {
		log.Errorf("error: %v (Note: WASM build requires JSON configuration)", err)
		return c, err
	}

	c.initLog()

	config = &c

	log.Infoln("Load the configuration file successfully!")
	if log.GetLevel() >= log.DebugLevel {
		s, err := json.MarshalIndent(c, "", "  ")
		if err != nil {
			log.Debugf("%+v", c)
		} else {
			log.Debugf("%s", string(s))
		}
	}

	return c, err
}
