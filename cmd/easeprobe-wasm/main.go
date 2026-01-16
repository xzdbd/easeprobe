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

	// previousStatus argument (JSON string)
	var prevStatusMap map[string]probe.Status
	if len(args) > 2 && args[2].Type() == js.TypeString {
		json.Unmarshal([]byte(args[2].String()), &prevStatusMap)
	}
	if prevStatusMap == nil {
		prevStatusMap = make(map[string]probe.Status)
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

			// Run probes and notify
			results := runProbes(confObj, probers, notifies, prevStatusMap)

			// Build new status map (convert to map[string]interface{} for js.ValueOf)
			newStatusMap := make(map[string]interface{})
			for _, res := range results {
				// Convert probe.Status (named int) to string explicitly because js.ValueOf panics on named types
				newStatusMap[res.Name] = res.Status.String()
			}

			// Convert results to []interface{} to return to JS
			var jsResults []interface{}
			for _, res := range results {
				b, _ := json.Marshal(res)
				var m map[string]interface{}
				json.Unmarshal(b, &m)
				jsResults = append(jsResults, m)
			}

			// Return object { results: [...], status: {...} }
			output := map[string]interface{}{
				"results": jsResults,
				"status":  newStatusMap,
			}

			resolve.Invoke(js.ValueOf(output))
		}()
		return nil
	}))
}

func runProbes(c conf.Conf, probers []probe.Prober, notifies []notify.Notify, prevStatus map[string]probe.Status) []probe.Result {
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

			// Run probe
			res := p.Probe()
			log.Infof("%s: %s", p.Kind(), res.DebugJSON())

			// Patch PreStatus from persistence
			if status, ok := prevStatus[res.Name]; ok {
				res.PreStatus = status
			} else {
				res.PreStatus = probe.StatusInit
			}

			// Edge Triggered Logic
			// 1. No change: Skip
			if res.PreStatus == res.Status {
				log.Debugf("%s (%s) - Status no change [%s] == [%s], no notification.",
					res.Name, res.Endpoint, res.PreStatus, res.Status)
				// Skip notification
			} else if res.PreStatus == probe.StatusInit {
				// 2. Initial Run (Init -> Down or Init -> Up)
				// User requested NO notification on start
				log.Debugf("%s (%s) - Initial Status [%s] == [%s], skip notification (first run).",
					res.Name, res.Endpoint, res.PreStatus, res.Status)
				// Skip notification
			} else {
				// 3. Status Changed (Up->Down, Down->Up)
				log.Infof("%s (%s) - Status changed [%s] ==> [%s]",
					res.Name, res.Endpoint, res.PreStatus, res.Status)

				// Notify
				for _, n := range notifies {
					if c.Settings.Notify.Dry {
						n.DryNotify(res)
					} else {
						n.Notify(res)
					}
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
