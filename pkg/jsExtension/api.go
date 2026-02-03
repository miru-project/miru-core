package jsExtension

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/eventloop"
	"github.com/miru-project/miru-core/pkg/db"
	log "github.com/miru-project/miru-core/pkg/logger"
	"github.com/miru-project/miru-core/pkg/network"
)

func AsyncCallBack(api *ExtApi, pkg string, evalStr string) (any, error) {
	// Lock to prevent multiple calls to the same extension
	api.lock.Lock()
	defer api.lock.Unlock()

	ApiPkgCache.Store(pkg, api)

	var loop *eventloop.EventLoop

	// check extension does  contain eventloop runtime
	lop, eventLoopIsExist := extMemMap.Load(pkg)
	if eventLoopIsExist {
		loop = lop.(*eventloop.EventLoop)
	} else {
		api.initRuntimeV1(pkg)
		lop, _ = extMemMap.Load(pkg)
		loop = lop.(*eventloop.EventLoop)
	}
	loop.Stop()
	res := make(chan PromiseResult)
	defer close(res)
	loop.RunOnLoop(func(vm *goja.Runtime) {
		o, e := vm.RunString(evalStr)
		handlePromise(o, res, e)
	})
	loop.Start()
	defer loop.StopNoWait()
	result := <-res
	// close(res)
	// handle error from PromiseResult{err: e}
	if result.err != nil {
		var zero any
		return zero, result.err
	}
	// handle result when Promise has established
	o, e := await(result.promise)
	return o, e
}

func (api *ExtApi) setFunction(vm *goja.Runtime, name string, fn any) {
	if e := vm.Set(name, fn); e != nil {
		log.Println("Error setting function:", api.Ext.Pkg, name, e)
	}
}

func (api *ExtApi) initRuntimeV1(pkg string) {

	ApiPkgCache.Store(pkg, api)
	loop := eventloop.NewEventLoop(
		eventloop.WithRegistry(SharedRegistry),
	)

	if api == nil || api.service.program == nil {
		ApiPkgCache.SetError(pkg, fmt.Sprintf("extension %s not found", pkg))
	}
	ser := api.service
	loop.RunOnLoop(func(vm *goja.Runtime) {

		defer func() {
			if r := recover(); r != nil {
				if err, ok := r.(error); ok {
					ApiPkgCache.SetError(pkg, err.Error())
					return
				}
				log.Print("Unknown panic:", r)
			}
		}()

		service := api.service
		var job = Job{loop: loop}
		// Run the program for the  first time
		reg := SharedRegistry.Enable(vm)
		ser.initModule(reg, vm, &job)
		// eval base runtime
		if _, e := vm.RunProgram(service.base); e != nil {
			log.Println("Error running base script:", e)
			panic(e)
		}
		// eval extension program
		if _, e := vm.RunProgram(api.service.program); e != nil {
			log.Println("Error running extension script:", e)
			panic(e)
		}
		// Initialize the Ext class
		_, e := vm.RunString(fmt.Sprintf(`
			ext = new globalThis.Ext("%s");
			`, api.Ext.Website))

		if e != nil {
			panic(e)
		}

		api.registerFunction(vm, job)

	})
	loop.Start()
	defer loop.Stop()

	extMemMap.Store(pkg, loop)
}

func (api *ExtApi) registerFunction(vm *goja.Runtime, job Job) {
	pkg := api.Ext.Pkg
	// api.setFunction(vm, `println`, func(args ...any) {
	// 	log.Println(args...)
	// })
	api.setFunction(vm, "registerSetting", func(call goja.FunctionCall) goja.Value {

		val := call.Argument(0).ToObject(vm).Export()
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
		cookiesInterface := call.Argument(1).ToObject(vm).Export()
		cookies, ok := cookiesInterface.([]any)
		if !ok {
			panic(vm.ToValue(errors.New("invalid cookies format, expected array of strings")))
		}
		cookieStrs := make([]string, len(cookies))
		for i, c := range cookies {
			cookieStrs[i] = fmt.Sprintf("%v", c)
		}
		e := network.SetCookies(url, cookieStrs)
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
		opt := call.Argument(1).ToObject(vm).Export()
		var requestOptions network.RequestOptions
		jsonData, e := json.Marshal(opt)
		if e != nil {
			panic("Error marshalling options to JSON:" + e.Error())
		}

		if err := json.Unmarshal(jsonData, &requestOptions); err != nil {
			panic("Error unmarshalling JSON:" + err.Error())
		}

		res, err := network.Request[string](url, &requestOptions, network.ReadAll)

		if err != nil {
			panic(vm.ToValue(err))
		}
		return res
	})
}
