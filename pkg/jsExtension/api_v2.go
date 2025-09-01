package jsExtension

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/eventloop"
	errorhandle "github.com/miru-project/miru-core/errorHandle"
	"github.com/miru-project/miru-core/pkg/network"
)

var ApiPkgCache = make(map[string]*ExtApi)

func LoadApiV2(ext *Ext, script string) {
	scriptV2 := fmt.Sprintf(script, ext.Pkg, ext.Name, ext.Website)

	runtimeV2 := errorhandle.HandleFatal(goja.Compile("runtime_v2.js", scriptV2, true))
	api := &ExtApi{Ext: ext, service: &ExtBaseService{base: runtimeV2}}
	api.service.program, _ = compileExtension(ext)
	ApiPkgCache[ext.Pkg] = api
	api.Ext.Error = ""
	api.initEvalV2String()

}
func (api *ExtApi) initEvalV2String() {
	// Register  the async callback function for V2
	api.asyncCallBack = AsyncCallBackV2[any]
	api.latestEval = "latest(%d)"
	api.searchEval = "search(%d, '%s', %s)"
	api.detailEval = "detail('%s')"
	api.watchEval = "watch('%s')"
}

// Handle any extension async callback like latest, search, watch etc
func AsyncCallBackV2[T any](api *ExtApi, pkg string, evalStr string) (T, error) {
	ApiPkgCache[pkg] = api

	var loop *eventloop.EventLoop

	// Check extension does contain eventloop runtime,if not create a new one
	lop, eventLoopIsExist := extMemMap.Load(pkg)
	if eventLoopIsExist {
		loop = lop.(*eventloop.EventLoop)
	} else {
		loop = eventloop.NewEventLoop(
			eventloop.WithRegistry(SharedRegistry),
		)
		extMemMap.Store(pkg, loop)
	}

	if api == nil || api.service.program == nil {
		return *new(T), fmt.Errorf("extension %s not found", pkg)
	}
	ser := api.service
	res := make(chan PromiseResult)
	defer close(res)

	// var runtime *goja.Runtime
	loop.RunOnLoop(func(vm *goja.Runtime) {

		var job = Job{loop: loop}
		service := api.service
		reg := SharedRegistry.Enable(vm)
		ser.initModule(reg, vm, &job)

		if !eventLoopIsExist {
			// Run the program that has compiled before
			vm.RunProgram(service.base)
			vm.RunProgram(api.service.program)
		}

		vm.Set(`println`, func(args ...any) {
			log.Println(args...)
		})

		ser.createSingleChannel(vm, "jsRequest", &job, func(call goja.FunctionCall, resolve func(any) error) any {

			url := call.Argument(0).ToString().String()
			opt := call.Argument(1).ToObject(vm).Export()
			var requestOptions network.RequestOptions

			jsonData := errorhandle.HandleFatal(json.Marshal(opt))
			if err := json.Unmarshal(jsonData, &requestOptions); err != nil {
				panic("Error unmarshalling JSON:" + err.Error())
			}

			res, err := network.Request[string](url, &requestOptions, network.ReadAll)

			if err != nil {
				panic(vm.ToValue(err))
			}
			return res
		})
		o, e := vm.RunString(evalStr)

		if e != nil {
			// This kind of error happens before the async function is called
			res <- PromiseResult{err: e}
		} else {
			// Because it eval async funcion the value become a promise and send to channel
			res <- PromiseResult{promise: o.Export().(*goja.Promise)}
		}

	})
	loop.Start()
	defer loop.Stop()

	result := <-res
	// handle error from PromiseResult{err: e}
	if result.err != nil {
		var zero T
		return zero, result.err
	}
	// handle result when Promise has established
	o, e := await[T](result.promise)
	return o, e

}
