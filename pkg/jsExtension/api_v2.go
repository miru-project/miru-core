package jsExtension

import (
	"fmt"

	log "github.com/miru-project/miru-core/pkg/logger"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/eventloop"
)

func LoadApiV2(ext *Ext) {

	compiledExt, e := compileExtension(ext)
	if e != nil {
		return
	}
	api := &ExtApi{Ext: ext, service: &ExtBaseService{program: compiledExt}}
	ApiPkgCache.Store(ext.Pkg, api)
	ApiPkgCache.SetError(ext.Pkg, "")

	api.initEvalV2String()
	api.initRuntimeV2(ext.Pkg)
	api.loadExtensionV2(ext.Pkg)
	log.Println("Extension loaded (V2):", ext.Name, ext.Pkg)
}

func (api *ExtApi) initRuntimeV2(pkg string) {

	ApiPkgCache.Store(pkg, api)
	loop := eventloop.NewEventLoop(
		eventloop.WithRegistry(sharedRegistry),
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

		var job = Job{loop: loop}
		// Run the program for the  first time
		reg := sharedRegistry.Enable(vm)
		ser.addModule(reg, vm, &job)
		// eval base runtime
		if _, e := vm.RunProgram(baseV2); e != nil {
			log.Println("Error running base script:", e)
			panic(e)
		}
		// eval extension program
		if _, e := vm.RunProgram(api.service.program); e != nil {
			log.Println("Error running extension script:", e)
			panic(e)
		}

		api.registerFunction(vm, job)

	})
	loop.Start()
	defer loop.Stop()

	extMemMap.Store(pkg, loop)
}

func (api *ExtApi) loadExtensionV2(pkg string) {
	if _, e := AsyncCallBack(api, pkg, "load()"); e != nil {
		ApiPkgCache.SetError(pkg, e.Error())
	}
}

func (api *ExtApi) initEvalV2String() {
	// Register  the async callback function for V2
	api.asyncCallBack = AsyncCallBack
	api.latestEval = "latest(%d)"
	api.searchEval = "search('%s', %d, %s)"
	api.detailEval = "detail('%s')"
	api.watchEval = "watch('%s')"
	api.mirrorEval = "mirror('%s')"
	api.createFilterEval = "createFilter(%s)"
}
