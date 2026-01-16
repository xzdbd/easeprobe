//go:build js && wasm
// +build js,wasm

package main

import (
	"encoding/json"
	"fmt"
	"sync"
	"syscall/js"
	"time"

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

// ProbeState persists the state of a probe between WASM invocations
type ProbeState struct {
	Status        string    `json:"status"`
	LastProbeTime time.Time `json:"last_probe_time"`
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
	// Map ProbeName -> ProbeState
	prevStatusMap := make(map[string]ProbeState)
	if len(args) > 2 && args[2].Type() == js.TypeString {
		json.Unmarshal([]byte(args[2].String()), &prevStatusMap)
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
			results, newStatusMap := runProbes(confObj, probers, notifies, prevStatusMap)

			// Convert results to []interface{} to return to JS
			var jsResults []interface{}
			for _, res := range results {
				b, _ := json.Marshal(res)
				var m map[string]interface{}
				json.Unmarshal(b, &m)
				jsResults = append(jsResults, m)
			}

			// Convert newStatusMap to map[string]interface{} for js.ValueOf compatibility
			jsStatusMap := make(map[string]interface{})
			for k, v := range newStatusMap {
				// Marshal to ensure clean JSON object structure for JS
				b, _ := json.Marshal(v)
				var m map[string]interface{}
				json.Unmarshal(b, &m)
				jsStatusMap[k] = m
			}

			// Return object { results: [...], status: {...} }
			output := map[string]interface{}{
				"results": jsResults,
				"status":  jsStatusMap,
			}

			resolve.Invoke(js.ValueOf(output))
		}()
		return nil
	}))
}

func runProbes(c conf.Conf, probers []probe.Prober, notifies []notify.Notify, prevStatus map[string]ProbeState) ([]probe.Result, map[string]ProbeState) {
	var results []probe.Result
	// newStatusMap starts as a copy of prevStatus to preserve state of skipped probes
	newStatusMap := make(map[string]ProbeState)
	for k, v := range prevStatus {
		newStatusMap[k] = v
	}

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

	now := time.Now()

	// Run Probes
	for _, p := range probers {
		if err := p.Config(gProbeConf); err != nil {
			log.Errorf("Probe Config Error: %v", err)
			continue
		}

		name := p.Result().Name

		// Check Interval
		interval := p.Interval()
		if state, ok := prevStatus[name]; ok {
			// If LastProbeTime is valid and interval has not passed, skip
			if !state.LastProbeTime.IsZero() && now.Sub(state.LastProbeTime) < interval {
				log.Debugf("Skipping probe %s: interval %v not reached (last run: %v)", name, interval, state.LastProbeTime)
				continue
			}
		}

		wg.Add(1)
		go func(p probe.Prober, name string) {
			defer wg.Done()

			// Run probe
			res := p.Probe()
			log.Infof("%s: %s", p.Kind(), res.DebugJSON())

			// Get Previous Status
			var preStatus probe.Status = probe.StatusInit
			if state, ok := prevStatus[name]; ok {
				// Convert string status back to probe.Status
				var s probe.Status
				s.Status(state.Status)
				preStatus = s
			}
			res.PreStatus = preStatus

			// Edge Triggered Logic
			if res.PreStatus == res.Status {
				log.Debugf("%s (%s) - Status no change [%s] == [%s], no notification.",
					res.Name, res.Endpoint, res.PreStatus, res.Status)
				// Skip notification
			} else {
				// Status Changed (Init->Down, Init->Up, Up->Down, Down->Up)
				log.Infof("%s (%s) - Status changed [%s] ==> [%s]",
					res.Name, res.Endpoint, res.PreStatus, res.Status)

				// Determine if we should notify
				shouldNotify := true
				if res.Status == probe.StatusUp {
					// User requested: "only trigger notify when the status is not success"
					// So if status is UP (Recovery or Init->Up), skip notification
					log.Infof("%s (%s) - Status is success, skipping notification per configuration.", res.Name, res.Endpoint)
					shouldNotify = false
				}

				if shouldNotify {
					for _, n := range notifies {
						if c.Settings.Notify.Dry {
							n.DryNotify(res)
						} else {
							n.Notify(res)
						}
					}
				}
			}

			mu.Lock()
			results = append(results, res)
			newStatusMap[name] = ProbeState{
				Status:        res.Status.String(),
				LastProbeTime: now,
			}
			mu.Unlock()
		}(p, name)
	}

	wg.Wait()
	return results, newStatusMap
}
