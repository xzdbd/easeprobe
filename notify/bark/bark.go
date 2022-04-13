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

package bark

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/megaease/easeprobe/global"
	"github.com/megaease/easeprobe/notify/base"
	"github.com/megaease/easeprobe/report"
	log "github.com/sirupsen/logrus"
)

// PushOptions is optional configurations of bark push options
type PushOptions struct {
	Category          string `json:"category" yaml:"category"`
	Level             string `json:"level,omitempty" yaml:"level"`
	Badge             string `json:"badge,omitempty" yaml:"badge"`
	AutomaticallyCopy string `json:"automaticallyCopy,omitempty" yaml:"automaticallyCopy"`
	Copy              string `json:"code,omitempty" yaml:"code"`
	Sound             string `json:"sound,omitempty" yaml:"sound"`
	Icon              string `json:"icon,omitempty" yaml:"icon"`
	Archive           string `json:"isArchive,omitempty" yaml:"isArchive"`
	Url               string `json:"url,omitempty" yaml:"url"`
	Group             string `json:"group,omitempty" yaml:"group"`
}

// Options is the type of body struct of bark push request
type Options struct {
	PushOptions
	Title     string `json:"title"`
	Body      string `json:"body"`
	DeviceKey string `json:"device_key"`
}

// PushResponse is the response of bark push
type PushResponse struct {
	Code    int    `json:"code"`
	Data    string `json:"data"`
	Message string `json:"message"`
}

// NotifyConfig is the bark notification configuration
type NotifyConfig struct {
	base.DefaultNotify `yaml:",inline"`
	Key                string `yaml:"key"`
	ServerUrl          string `yaml:"server"`
	PushOptions        `yaml:",inline"`
}

// Kind return the type of Notify
func (c *NotifyConfig) Kind() string {
	return c.MyKind
}

// Config configures the bark notification
func (c *NotifyConfig) Config(gConf global.NotifySettings) error {
	c.MyKind = "bark"
	c.Format = report.Text
	c.SendFunc = c.Push
	c.DefaultNotify.Config(gConf)
	log.Debugf("Notification [%s] - [%s] configuration: %+v", c.MyKind, c.Name)
	return nil
}

// Push pushes notification to bark-server
func (c *NotifyConfig) Push(subject string, message string) error {

	requestURL, err := url.Parse(c.ServerUrl)
	if err != nil {
		return err
	}
	opts := &Options{Title: subject, Body: message, DeviceKey: c.Key, PushOptions: c.PushOptions}

	log.Infof("opts %+v", opts)

	respBody, err := json.Marshal(opts)
	if err != nil {
		return err
	}

	resp, err := http.Post(requestURL.String(), "application/json", bytes.NewBuffer(respBody))
	if err != nil {
		return err
	}
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	pushResp := &PushResponse{}
	err = json.Unmarshal(b, pushResp)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 || pushResp.Code != 200 {
		return errors.New(pushResp.Message)
	}
	return nil
}
