package js

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/eventloop"
	"github.com/miru-project/miru-core/pkg/db"
	"github.com/miru-project/miru-core/pkg/event"
	log "github.com/miru-project/miru-core/pkg/logger"
	"github.com/miru-project/miru-core/pkg/network"
	"github.com/miru-project/miru-core/proto/generate/proto"
)

// AsyncCallBack runs an extension function (evalStr) on a FRESH goja event
// loop and returns its (awaited) result. The loop -- i.e. the goja VM -- is
// created here, used for this single call, and then stopped so it can be
// garbage collected. Nothing about the VM survives between calls: only the
// compiled program (api.service.program) is retained in the ExtApi, exactly as
// the extension API requires.
//
// Cross-call state is carried by the Go-native snapshot (instance_state.go):
// V1 instance properties and V2 globals are captured onto the recorded
// *stateNode tree before the loop is disposed and restored onto the next
// call's fresh VM. That gives both versions their "runtime is kept" semantics
// -- the logical extension instance survives -- without ever keeping a goja
// VM (and its event loop) alive, so the anti-leak guarantee (one VM per call,
// always disposed) still holds. The capture runs on the SUCCESS path and on
// failure paths alike (once the call actually reached the eval stage), so
// state written before an error is still waiting for the next call.
func AsyncCallBack(api *ExtApi, pkg string, evalStr string) (any, error) {
	// Lock to prevent multiple calls to the same extension
	api.lock.Lock()
	defer api.lock.Unlock()

	ApiPkgCache.Store(pkg, api)

	if api.service == nil || api.service.program == nil {
		return nil, fmt.Errorf("extension %s not loaded", pkg)
	}

	// Spin up a brand new event loop / goja VM for this call. It is disposed at
	// the end of this function (loop.Stop()).
	loop := eventloop.NewEventLoop(eventloop.WithRegistry(sharedRegistry))
	res := make(chan PromiseResult)
	defer close(res)

	// evalStarted marks that the call reached the extension eval stage on a
	// fully set-up VM (base runtime + program + host primitives + restore).
	// Only then is a failure-path capture meaningful; if setup itself failed
	// the VM never saw the extension, and capturing it would store a pristine
	// VM over the last good snapshot. Written on the loop goroutine, read
	// after the result channel below -- channel send/close establishes the
	// happens-before edge.
	evalStarted := false

	loop.RunOnLoop(func(vm *goja.Runtime) {
		defer func() {
			if r := recover(); r != nil {
				if err, ok := r.(error); ok {
					ApiPkgCache.SetError(pkg, err.Error())
					res <- PromiseResult{err: err}
					return
				}
				log.Print("Unknown panic:", r)
			}
		}()

		var job = Job{loop: loop}
		// Run the program for the  first time
		reg := sharedRegistry.Enable(vm)
		api.addModule(reg, vm, &job)
		// eval base runtime (v1 or v2)
		base := baseV1
		if api.Ext.ApiVersion == "2" {
			base = baseV2
		}
		if _, e := vm.RunProgram(base); e != nil {
			panic(e)
		}
		// eval the compiled extension program
		if _, e := vm.RunProgram(api.service.program); e != nil {
			panic(e)
		}
		// Initialize the V1 Ext class instance.
		if api.Ext.ApiVersion != "2" {
			if _, e := vm.RunString(fmt.Sprintf(`ext = new globalThis.Ext("%s");`, api.Ext.Website)); e != nil {
				panic(e)
			}
		}
		// Register the host primitives (jsRequest, settings, cookies,
		// btoa/atob...) BEFORE the previous state is restored, so a stale
		// snapshot value can never overwrite them.
		api.registerFunction(vm, job)

		// Restore the previous call's state onto this VM. This is a pure-Go
		// rebuild of the recorded *stateNode tree (no JS helpers, no JSON):
		// V1 writes onto the `ext` instance, V2 onto globalThis minus the
		// host-owned keys. Values keep their original structure -- functions
		// are recompiled from source, Date/RegExp/Map/Set/TypedArray/BigInt
		// are re-instantiated via their constructors.
		if prev, ok := loadInstanceState(pkg); ok {
			restoreStateOnVM(api, vm, prev)
		}

		evalStarted = true
		o, e := vm.RunString(evalStr)
		handlePromise(o, res, e)
	})
	loop.Start()

	result := <-res
	// The promise must be awaited BEFORE the state is captured: an async
	// latest()/load() mutates state after it returns its promise, so
	// capturing earlier would record the state as it was BEFORE the awaited
	// work finished.
	//
	// Capture ordering rationale: the snapshot is the ONLY carrier of
	// cross-call state now, so a write made before an error must still reach
	// the next call -- the capture therefore runs on failure paths too (what
	// saveCache's immediate write used to guarantee). Two guards keep that
	// safe: evalStarted ensures the VM was fully set up + restored (a setup
	// failure must not overwrite last-good with a pristine VM), and
	// finalizeInstanceState only saves when the capture itself succeeds, so a
	// broken VM keeps the previous snapshot instead of recording garbage.
	if result.err == nil {
		o, e := await(result.promise)
		if e != nil {
			// Async rejection: capture what the extension wrote before it
			// threw, then surface the error.
			if evalStarted {
				finalizeInstanceState(api, loop)
			}
			loop.Stop()
			return nil, e
		}
		// Capture the state before the loop is disposed. Both versions go
		// through here: V1 from the `ext` instance, V2 from globalThis.
		finalizeInstanceState(api, loop)
		// Stop (and therefore dispose) the loop now that the call is done.
		loop.Stop()
		return o, nil
	}

	// Sync throw / panic: same capture guarantee as above.
	if evalStarted {
		finalizeInstanceState(api, loop)
	}
	loop.Stop()
	return nil, result.err
}

func (api *ExtApi) setFunction(vm *goja.Runtime, name string, fn any) {
	if e := vm.Set(name, fn); e != nil {
		log.Println("Error setting function:", api.Ext.Pkg, name, e)
	}
}

func (api *ExtApi) registerFunction(vm *goja.Runtime, job Job) {
	pkg := api.Ext.Pkg
	// api.setFunction(vm, `println`, func(args ...any) {
	// 	log.Println(args...)
	// })
	api.setFunction(vm, "registerSetting", func(call goja.FunctionCall) goja.Value {

		val := call.Argument(0).Export()
		value, ok := val.(map[string]any)
		if !ok {
			panic(vm.ToValue(errors.New("invalid setting object need map")))
		}

		if e := db.RegisterSetting(value, pkg); e != nil {
			panic(vm.ToValue(fmt.Errorf("error registering setting: %w", e)))
		}
		return vm.ToValue(nil)
	})

	api.setFunction(vm, "getSetting", func(call goja.FunctionCall) goja.Value {
		key := call.Argument(0).ToString().String()
		setting, e := db.GetSetting(pkg, key)
		if e != nil {
			panic(vm.ToValue(errors.New("Error getting setting:" + e.Error())))
		}
		if setting == nil {
			return nil
		}
		return vm.ToValue(setting.Value)
	})

	api.setFunction(vm, "setSetting", func(call goja.FunctionCall) any {
		pkg := call.Argument(0).ToString().String()
		key := call.Argument(1).ToString().String()
		value := call.Argument(2).ToString().String()
		e := db.SetSetting(pkg, key, value)
		if e != nil {
			panic(vm.ToValue(errors.New("Error setting setting:" + e.Error())))
		}
		return nil
	})

	api.setFunction(vm, "getCookies", func(call goja.FunctionCall) any {
		url := call.Argument(0).ToString().String()
		cookie, e := network.GetCookies(url)
		if e != nil {
			panic(vm.ToValue(errors.New("Error getting cookies:" + e.Error())))
		}
		return cookie
	})

	api.setFunction(vm, "setCookies", func(call goja.FunctionCall) any {
		url := call.Argument(0).ToString().String()
		cookiesInterface := call.Argument(1).Export()
		cookies, ok := cookiesInterface.([]any)
		if !ok {
			panic(vm.ToValue(errors.New("invalid cookies format, expected array of strings")))
		}
		cookieStrs := make([]string, len(cookies))
		for i, c := range cookies {
			cookieStrs[i] = fmt.Sprintf("%v", c)
		}
		e := network.SetCookiesString(url, cookieStrs)
		if e != nil {
			panic(vm.ToValue(errors.New("Error setting cookies:" + e.Error())))
		}
		return nil
	})

	api.setFunction(vm, "btoa", func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(base64.RawStdEncoding.EncodeToString([]byte(call.Arguments[0].String())))
	})
	api.setFunction(vm, "atob", func(call goja.FunctionCall) goja.Value {
		str, _ := base64.RawStdEncoding.DecodeString(call.Arguments[0].String())
		return vm.ToValue(string(str))
	})

	api.service.createSingleChannel(vm, "jsRequest", &job, func(call goja.FunctionCall, resolve func(any) error) any {

		url := call.Argument(0).ToString().String()
		url = strings.ReplaceAll(url, "&amp;", "&")

		// Safe export of options
		var opt any
		if len(call.Arguments) > 1 {
			opt = call.Argument(1).Export()
		}

		var requestOptions network.RequestOptions
		if opt != nil {
			jsonData, e := json.Marshal(opt)
			if e != nil {
				panic("Error marshalling options to JSON:" + e.Error())
			}

			if err := json.Unmarshal(jsonData, &requestOptions); err != nil {
				panic("Error unmarshalling JSON:" + err.Error())
			}
		}

		start := time.Now()
		res, err := network.Request[string](url, &requestOptions, network.ReadAll)
		duration := time.Since(start).Milliseconds()

		// Capture event for dev mode if anyone is listening
		if event.GlobalBus.HasSubscribers() {
			status := res.StatusCode
			resHeaders := fmt.Sprintf("%v", res.Headers)

			go func(s int, h string, b string) {
				event.SendDevNetwork(&proto.DevNetworkEvent{
					Package:         pkg,
					Url:             url,
					Method:          requestOptions.Method,
					Status:          int32(s),
					Duration:        duration,
					Timestamp:       time.Now().UnixMilli(),
					RequestHeaders:  fmt.Sprintf("%v", requestOptions.Headers),
					RequestBody:     requestOptions.RequestBody,
					ResponseHeaders: h,
					ResponseBody:    b,
				})
			}(status, resHeaders, res.Body)
		}

		if err != nil {
			panic(err.Error())
		}
		return res.Body
	})
}
