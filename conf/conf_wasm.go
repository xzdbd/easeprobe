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

	"github.com/megaease/easeprobe/notify"
	"github.com/megaease/easeprobe/probe"
	"github.com/megaease/easeprobe/probe/http"
	"github.com/megaease/easeprobe/probe/tcp"
	log "github.com/sirupsen/logrus"
)

// Retry is the settings of notification retry
// Redefined for JSON support (ConfigDuration)
type Retry struct {
	Times    int                  `json:"times"`
	Interval probe.ConfigDuration `json:"interval"`
}

// Notify is the settings of notification
type Notify struct {
	Retry Retry `yaml:"retry" json:"retry"`
	Dry   bool  `yaml:"dry" json:"dry"`
}

// Probe is the settings of prober
type Probe struct {
	Interval probe.ConfigDuration `yaml:"interval" json:"interval"`
	Timeout  probe.ConfigDuration `yaml:"timeout" json:"timeout"`
}

// SLAReport is the settings for SLA report
type SLAReport struct {
	Schedule Schedule `yaml:"schedule" json:"schedule"`
	Time     string   `yaml:"time" json:"time"`
	Debug    bool     `yaml:"debug" json:"debug"`
}

// Settings is the EaseProbe configuration
type Settings struct {
	LogFile    string    `yaml:"logfile" json:"logfile"`
	LogLevel   LogLevel  `yaml:"loglevel" json:"loglevel"`
	TimeFormat string    `yaml:"timeformat" json:"timeformat"`
	Probe      Probe     `yaml:"probe" json:"probe"`
	Notify     Notify    `yaml:"notify" json:"notify"`
	SLAReport  SLAReport `yaml:"sla" json:"sla"`
	logfile    *os.File  `yaml:"-" json:"-"`
}

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
				Interval: probe.ConfigDuration{time.Second * 60},
				Timeout:  probe.ConfigDuration{time.Second * 10},
			},
			Notify: Notify{
				Retry: Retry{
					Times:    3,
					Interval: probe.ConfigDuration{time.Second * 5},
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
