//go:build js && wasm
// +build js,wasm

package main

import (
	"encoding/json"
	"fmt"
	"sync"
	"syscall/js"

	"github.com/megaease/easeprobe/conf"
	"github.com/megaease/easeprobe/global"
	"github.com/megaease/easeprobe/notify"
	"github.com/megaease/easeprobe/probe"
	log "github.com/sirupsen/logrus"
)

func main() {
	c := make(chan struct{}, 0)
	js.Global().Set("check", js.FuncOf(check))
	<-c
}

func check(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return js.Global().Get("Promise").New(js.FuncOf(func(this js.Value, pArgs []js.Value) interface{} {
			pArgs[1].Invoke("Error: config YAML string required")
			return nil
		}))
	}
	configStr := args[0].String()

	dryRun := false
	if len(args) > 1 && args[1].Bool() {
		dryRun = true
	}

	// Create a Promise
	promiseConstructor := js.Global().Get("Promise")
	return promiseConstructor.New(js.FuncOf(func(this js.Value, pArgs []js.Value) interface{} {
		resolve := pArgs[0]
		reject := pArgs[1]

		go func() {
			defer func() {
				if r := recover(); r != nil {
					reject.Invoke(fmt.Sprintf("Panic: %v", r))
				}
			}()

			confObj, err := conf.NewFromBytes([]byte(configStr))
			if err != nil {
				reject.Invoke(fmt.Sprintf("Error parsing config: %v", err))
				return
			}

			if dryRun {
				confObj.Settings.Notify.Dry = true
			}

			probers := confObj.AllProbers()
			notifies := confObj.AllNotifiers()

			// This is now running in a goroutine, so it won't block the JS event loop
			results := runProbes(confObj, probers, notifies)

			// Convert results to []interface{} to return to JS
			var jsResults []interface{}
			for _, res := range results {
				// Marshal to map to make it easy to pass to JS
				b, _ := json.Marshal(res)
				var m map[string]interface{}
				json.Unmarshal(b, &m)
				jsResults = append(jsResults, m)
			}

			resolve.Invoke(js.ValueOf(jsResults))
		}()
		return nil
	}))
}

func runProbes(c conf.Conf, probers []probe.Prober, notifies []notify.Notify) []probe.Result {
	var results []probe.Result
	var mu sync.Mutex
	var wg sync.WaitGroup

	gProbeConf := global.ProbeSettings{
		TimeFormat: c.Settings.TimeFormat,
		Interval:   c.Settings.Probe.Interval,
		Timeout:    c.Settings.Probe.Timeout,
	}

	gNotifyConf := global.NotifySettings{
		TimeFormat: c.Settings.TimeFormat,
		Retry:      c.Settings.Notify.Retry,
	}

	// Config Notifiers
	for _, n := range notifies {
		if err := n.Config(gNotifyConf); err != nil {
			log.Errorf("Notify Config Error: %v", err)
		}
	}

	// Run Probes
	for _, p := range probers {
		if err := p.Config(gProbeConf); err != nil {
			log.Errorf("Probe Config Error: %v", err)
			continue
		}

		wg.Add(1)
		go func(p probe.Prober) {
			defer wg.Done()
			res := p.Probe()
			log.Infof("%s: %s", p.Kind(), res.DebugJSON())

			// Notify
			for _, n := range notifies {
				if c.Settings.Notify.Dry {
					n.DryNotify(res)
				} else {
					n.Notify(res)
				}
			}

			mu.Lock()
			results = append(results, res)
			mu.Unlock()
		}(p)
	}

	wg.Wait()
	return results
}
