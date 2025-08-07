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
	api.InitV1Script(ext.Pkg)

}

func (api *ExtApi) InitV1Script(pkg string) error {
	ApiPkgCache[pkg] = api

	loop := eventloop.NewEventLoop(
		eventloop.WithRegistry(SharedRegistry),
	)

	if api == nil || api.service.program == nil {
		return fmt.Errorf("extension %s not found", pkg)
	}
	ser := api.service
	res := make(chan PromiseResult)
	defer close(res)

	// var runtime *goja.Runtime
	loop.RunOnLoop(func(vm *goja.Runtime) {

		defer func() {
			if r := recover(); r != nil {
				if err, ok := r.(error); ok {
					ApiPkgCache[pkg].Ext.Error = err.Error()
					res <- PromiseResult{err: err}
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
		if _, e := vm.RunString(ReplaceClassExtendsDeclaration(*api.Ext.Context)); e != nil {
			log.Println("Error running extension script:", e)
			panic(e)
		}
		// Initialize the Ext class
		_, e := vm.RunString(fmt.Sprintf(`
			ext = new Ext("%s");
			`, api.Ext.Website))

		if e != nil {
			panic(e)
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

		o, e := (vm.RunString(fmt.Sprintf(`
		try{%s}catch(e){println(e);throw e}`, "ext.load()")))
		handlePromise(o, res, e)

	})
	loop.Start()
	defer loop.Stop()

	result := <-res
	extMemMap.Store(pkg, loop)
	// handle error from PromiseResult{err: e}
	if result.err != nil {
		return result.err
	}
	// handle result when Promise has established
	_, e := await[any](result.promise)
	return e

}

func handlePromise(o goja.Value, res chan PromiseResult, e error) {
	if e != nil {
		// This kind of error happens before the async function is called
		res <- PromiseResult{err: e}
	} else {
		// Because it eval async funcion the value become a promise and send to channel
		res <- PromiseResult{promise: o.Export().(*goja.Promise)}
	}
}

func AsyncCallBackV1[T any](api *ExtApi, pkg string, evalStr string) (T, error) {
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

	// var runtime *goja.Runtime
	loop.RunOnLoop(func(vm *goja.Runtime) {

		o, e := vm.RunString(fmt.Sprintf(`
		try{%s}catch(e){println(e);throw e}`, evalStr))

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
