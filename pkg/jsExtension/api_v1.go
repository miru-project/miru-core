package jsExtension

import (
	"encoding/json"
	"fmt"
	"log"
	"regexp"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/eventloop"
	errorhandle "github.com/miru-project/miru-core/errorHandle"
	"github.com/miru-project/miru-core/pkg/network"
)

// type ExtApi struct {
// 	Ext     *Ext
// 	service *ExtBaseService
// }

// var ApiPkgCacheV1 = make(map[string]*ExtApi)

func LoadApiV1(ext *Ext, script string) {
	scriptV1 := fmt.Sprintf(script, ext.Pkg, ext.Name, ext.Website)
	re := regexp.MustCompile(`export\s+default\s+class.+extends\s+Extension\s*{`)
	*ext.Context = re.ReplaceAllString(*ext.Context, "class Ext extends Extension {")
	runtimeV1 := errorhandle.HandleFatal(goja.Compile("runtime_v1.js", scriptV1, true))
	api := &ExtApi{Ext: ext, service: &ExtBaseService{base: runtimeV1}}
	ApiPkgCache[ext.Pkg] = api
	ApiPkgCache[ext.Pkg].service.program = compileScript(ext)
	ApiPkgCache[ext.Pkg].Ext.Error = ""
}

func AsyncCallBackV1[T any](api *ExtApi, pkg string, evalStr string) (T, error) {
	ApiPkgCache[pkg] = api

	var loop *eventloop.EventLoop

	// check extension does  contain eventloop runtime
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

		service := api.service
		var job = Job{loop: loop}
		// Run the program for the  first time
		if !eventLoopIsExist {
			reg := SharedRegistry.Enable(vm)
			ser.initModule(reg, vm, &job)
			// eval base runtime
			vm.RunProgram(service.base)
			// eval extension program
			vm.RunProgram(api.service.program)
			// Initialize the Ext class
			_, e := vm.RunString(fmt.Sprintf(`
			ext = new Ext("%s");
			`, api.Ext.Website))

			if e != nil {
				panic(e)
			}
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
