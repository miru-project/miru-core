package jsExtension

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/eventloop"
	errorhandle "github.com/miru-project/miru-core/errorHandle"
	"github.com/miru-project/miru-core/pkg/db"
	"github.com/miru-project/miru-core/pkg/network"
)

func LoadApiV1(ext *Ext, baseScript string) {

	scriptV1 := fmt.Sprintf(baseScript, ext.Pkg, ext.Name, ext.Website)
	*ext.Context = ReplaceClassExtendsDeclaration(*ext.Context)
	runtimeV1 := errorhandle.HandleFatal(goja.Compile("runtime_v1.js", scriptV1, true))
	compiledExt, e := compileExtension(ext)
	if e != nil {
		log.Println("Error compiling extension:", e)
		ext.Error = e.Error()
		ApiPkgCache[ext.Pkg].Ext.Error = ""
		return
	}
	api := &ExtApi{Ext: ext, service: &ExtBaseService{base: runtimeV1, program: compiledExt}}
	ApiPkgCache[ext.Pkg] = api
	ApiPkgCache[ext.Pkg].Ext.Error = ""
	api.initEvalV1String()
	api.InitV1Script(ext.Pkg)
	api.loadExtension(ext.Pkg)

}
func (api *ExtApi) loadExtension(pkg string) {
	if _, e := AsyncCallBackV1(api, pkg, "ext.load()"); e != nil {
		ApiPkgCache[pkg].Ext.Error = e.Error()
	}
}
func (api *ExtApi) initEvalV1String() {
	// Register  the async callback function for V1
	api.asyncCallBack = AsyncCallBackV1
	api.latestEval = "ext.latest(%d)"
	api.searchEval = "ext.search(%d, '%s', %s)"
	api.detailEval = "ext.detail('%s')"
	api.watchEval = "ext.watch('%s')"
}

func (api *ExtApi) InitV1Script(pkg string) {

	ApiPkgCache[pkg] = api
	loop := eventloop.NewEventLoop(
		eventloop.WithRegistry(SharedRegistry),
	)

	if api == nil || api.service.program == nil {
		ApiPkgCache[pkg].Ext.Error = fmt.Sprintf("extension %s not found", pkg)
	}
	ser := api.service
	// res := make(chan PromiseResult)
	// defer close(res)

	// var runtime *goja.Runtime
	loop.RunOnLoop(func(vm *goja.Runtime) {

		defer func() {
			if r := recover(); r != nil {
				if err, ok := r.(error); ok {
					ApiPkgCache[pkg].Ext.Error = err.Error()
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

		vm.Set(`println`, func(args ...any) {
			log.Println(args...)
		})

		// vm.Set("registerSetting", )
		vm.Set("registerSetting", func(call goja.FunctionCall) goja.Value {

			val := call.Argument(0).ToObject(vm).Export()
			value, ok := val.(map[string]any)
			if !ok {
				panic(vm.ToValue(errors.New("invalid setting object need map")))
			}

			e = db.RegisterSetting(value, pkg)
			if e != nil {
				panic(vm.ToValue(fmt.Errorf("error registering setting: %w", e)))
			}
			return vm.ToValue(nil)
		})

		vm.Set("getSetting", func(call goja.FunctionCall) goja.Value {
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

		vm.Set("setSetting", func(call goja.FunctionCall) any {
			pkg := call.Argument(0).ToString().String()
			key := call.Argument(1).ToString().String()
			value := call.Argument(2).ToString().String()
			e := db.SetSetting(pkg, key, value)
			if e != nil {
				panic("Error setting setting:" + e.Error())
			}
			return nil
		})

		ser.createSingleChannel(vm, "jsRequest", &job, func(call goja.FunctionCall, resolve func(any) error) any {

			url := call.Argument(0).ToString().String()
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

	})
	loop.Start()
	defer loop.Stop()

	extMemMap.Store(pkg, loop)

}

func handlePromise(o goja.Value, res chan PromiseResult, e error) {
	if e != nil {
		// This kind of error happens before the async function is called
		res <- PromiseResult{err: e}
		return
	}
	// Because it eval async funcion the value become a promise and send to channel
	res <- PromiseResult{promise: o.Export().(*goja.Promise)}
}

func AsyncCallBackV1(api *ExtApi, pkg string, evalStr string) (any, error) {
	ApiPkgCache[pkg] = api

	var loop *eventloop.EventLoop

	// check extension does  contain eventloop runtime
	lop, eventLoopIsExist := extMemMap.Load(pkg)
	if eventLoopIsExist {
		loop = lop.(*eventloop.EventLoop)
	} else {
		api.InitV1Script(pkg)
		lop, _ = extMemMap.Load(pkg)
		loop = lop.(*eventloop.EventLoop)
	}

	res := make(chan PromiseResult)
	defer close(res)
	loop.RunOnLoop(func(vm *goja.Runtime) {
		o, e := vm.RunString(evalStr)
		handlePromise(o, res, e)
	})

	loop.Start()
	defer loop.Stop()
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
