package jsExtension

import (
	"encoding/json"
	"fmt"
	"log"
	"regexp"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/eventloop"
	"github.com/miru-project/miru-core/pkg/network"
)

type ExtApiV1 struct {
	ext     *Ext
	service *ExtBaseService
}

var ApiPkgCacheV1 = make(map[string]*ExtApiV1)

func LoadApiV1(ext *Ext, script string) {
	scriptV1 := fmt.Sprintf(script, ext.pkg, ext.name, ext.website)
	re := regexp.MustCompile(`export\s+default\s+class.+extends\s+Extension\s*{`)
	*ext.context = re.ReplaceAllString(*ext.context, "class Ext extends Extension {")
	compile := handleFatal(goja.Compile(ext.pkg+".js", *ext.context, true))
	runtimeV1 := handleFatal(goja.Compile("runtime_v1.js", scriptV1, true))
	api := &ExtApiV1{ext: ext, service: &ExtBaseService{program: compile, base: runtimeV1}}

	ApiPkgCacheV1[ext.pkg] = api
}

func AsyncCallBackV1[T any](api *ExtApiV1, pkg string, evalStr string) (T, error) {
	ApiPkgCacheV1[pkg] = api

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

	if api == nil || api.ext == nil {
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
			`, api.ext.website))

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

			jsonData := handleFatal(json.Marshal(opt))
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
