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

package slack

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/megaease/easeprobe/global"
	"github.com/megaease/easeprobe/notify/base"
	"github.com/megaease/easeprobe/report"
	log "github.com/sirupsen/logrus"
)

// NotifyConfig is the slack notification configuration
type NotifyConfig struct {
	base.DefaultNotify `yaml:",inline"`
	WebhookURL         string `yaml:"webhook"`
}

// Kind return the type of Notify
func (c *NotifyConfig) Kind() string {
	return c.MyKind
}

// Config configures the slack notification
func (c *NotifyConfig) Config(gConf global.NotifySettings) error {
	c.MyKind = "slack"
	c.Format = report.Slack
	c.SendFunc = c.SendSlack
	c.DefaultNotify.Config(gConf)
	log.Debugf("Notification [%s] - [%s] configuration: %+v", c.MyKind, c.Name, c)
	return nil
}

// SendSlack is the wrapper for SendSlackNotification
func (c *NotifyConfig) SendSlack(title, msg string) error {
	return c.SendSlackNotification(msg)
}

// SendSlackNotification will post to an 'Incoming Webhook' url setup in Slack Apps. It accepts
// some text and the slack channel is saved within Slack.
func (c *NotifyConfig) SendSlackNotification(msg string) error {
	req, err := http.NewRequest(http.MethodPost, c.WebhookURL, bytes.NewBuffer([]byte(msg)))
	if err != nil {
		return err
	}

	req.Header.Add("Content-Type", "application/json")
	req.Close = true

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("Error response from Slack [%d] - [%s]", resp.StatusCode, string(buf))
	}
	// if buf.String() != "ok" {
	// 	return errors.New("Non-ok response returned from Slack " + buf.String())
	// }
	return nil
}
