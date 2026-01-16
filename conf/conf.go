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
	"io/ioutil"
	"os"
	"reflect"
	"strings"

	"github.com/megaease/easeprobe/notify"
	"github.com/megaease/easeprobe/probe"
	log "github.com/sirupsen/logrus"
)

var config *Conf

// Get return the global configuration
func Get() *Conf {
	return config
}

// LogLevel is the log level
type LogLevel struct {
	Level log.Level
}

// UnmarshalYAML is unmarshal the debug level
func (l *LogLevel) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var level string
	if err := unmarshal(&level); err != nil {
		return err
	}
	return l.parse(level)
}

// UnmarshalJSON is unmarshal the debug level
func (l *LogLevel) UnmarshalJSON(b []byte) (err error) {
	return l.parse(strings.Trim(string(b), `"`))
}

func (l *LogLevel) parse(level string) error {
	switch strings.ToLower(level) {
	case "debug":
		l.Level = log.DebugLevel
	case "info":
		l.Level = log.InfoLevel
	case "warn":
		l.Level = log.WarnLevel
	case "error":
		l.Level = log.ErrorLevel
	case "fatal":
		l.Level = log.FatalLevel
	case "panic":
		l.Level = log.PanicLevel
	}
	return nil
}

// Schedule is the schedule.
type Schedule int

const (
	Hourly Schedule = iota
	Daily
	Weekly
	Monthly
	None
)

// UnmarshalYAML is unmarshal the debug level
func (s *Schedule) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var level string
	if err := unmarshal(&level); err != nil {
		return err
	}
	return s.parse(level)
}

// UnmarshalJSON is unmarshal the debug level
func (s *Schedule) UnmarshalJSON(b []byte) (err error) {
	return s.parse(strings.Trim(string(b), `"`))
}

func (s *Schedule) parse(level string) error {
	switch strings.ToLower(level) {
	case "hourly":
		*s = Hourly
	case "daily":
		*s = Daily
	case "weekly":
		*s = Weekly
	case "monthly":
		*s = Monthly
	default:
		*s = None
	}
	return nil
}

// New read the configuration from yaml
func New(conf *string) (Conf, error) {
	y, err := ioutil.ReadFile(*conf)
	if err != nil {
		log.Errorf("error: %v ", err)
		return Conf{}, err
	}
	return NewFromBytes(y)
}

func (conf *Conf) initLog() {
	log.SetFormatter(&log.TextFormatter{FullTimestamp: true})
	if conf == nil {
		log.SetOutput(os.Stdout)
		log.SetLevel(log.InfoLevel)
	} else {
		// open a file
		if conf.Settings.LogFile != "" {
			f, err := os.OpenFile(conf.Settings.LogFile, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0660)
			if err != nil {
				log.Warnf("Cannot open log file: %v", err)
				log.Infoln("Using Standard Output as the log output...")
				log.SetOutput(os.Stdout)
			} else {
				conf.Settings.logfile = f
				log.SetOutput(f)
			}
		} else {
			log.SetOutput(os.Stdout)
		}
		log.SetLevel(conf.Settings.LogLevel.Level)
	}
}

// CloseLogFile close the log file
func (conf *Conf) CloseLogFile() {
	if conf.Settings.logfile != nil {
		conf.Settings.logfile.Close()
	}
}

// isProbe checks whether a interface is a probe type
func isProbe(t reflect.Type) bool {
	modelType := reflect.TypeOf((*probe.Prober)(nil)).Elem()
	return t.Implements(modelType)
}

// AllProbers return all probers
func (conf *Conf) AllProbers() []probe.Prober {
	log.Debugf("--------- Process the probers settings ---------")
	return allProbersHelper(*conf)
}

func allProbersHelper(i interface{}) []probe.Prober {

	var probers []probe.Prober
	t := reflect.TypeOf(i)
	v := reflect.ValueOf(i)
	if t.Kind() != reflect.Struct {
		return probers
	}

	for i := 0; i < t.NumField(); i++ {
		tField := t.Field(i).Type.Kind()
		if tField == reflect.Struct {
			probers = append(probers, allProbersHelper(v.Field(i).Interface())...)
			continue
		}
		if tField != reflect.Slice {
			continue
		}

		vField := v.Field(i)
		for j := 0; j < vField.Len(); j++ {
			if !isProbe(vField.Index(j).Addr().Type()) {
				//log.Debugf("%s is not a probe type", vField.Index(j).Type())
				continue
			}

			log.Debugf("--> %s / %s / %v", t.Field(i).Name, t.Field(i).Type.Kind(), vField.Index(j))
			probers = append(probers, vField.Index(j).Addr().Interface().(probe.Prober))
		}
	}

	return probers
}

// isNotify checks whether a interface is a Notify type
func isNotify(t reflect.Type) bool {
	modelType := reflect.TypeOf((*notify.Notify)(nil)).Elem()
	return t.Implements(modelType)
}

// AllNotifiers return all notifiers
func (conf *Conf) AllNotifiers() []notify.Notify {
	var notifies []notify.Notify

	log.Debugf("--------- Process the notification settings ---------")
	t := reflect.TypeOf(conf.Notify)
	for i := 0; i < t.NumField(); i++ {
		if t.Field(i).Type.Kind() != reflect.Slice {
			continue
		}
		v := reflect.ValueOf(conf.Notify).Field(i)
		for j := 0; j < v.Len(); j++ {
			if !isNotify(v.Index(j).Addr().Type()) {
				log.Debugf("%s is not a probe type", v.Index(j).Type())
				continue
			}
			log.Debugf("--> %s - %s - %v", t.Field(i).Name, t.Field(i).Type.Kind(), v.Index(j))
			notifies = append(notifies, v.Index(j).Addr().Interface().(notify.Notify))
		}
	}

	return notifies
}
