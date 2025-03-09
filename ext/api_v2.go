package ext

import (
	"log"
	"os"

	"github.com/dop251/goja"
)

type ExtApiV2 struct {
	runtime *goja.Runtime
	ext     Ext
}

func (api *ExtApiV2) loadApiV2() {
	vm := api.runtime
	ext := api.ext
	reg := SharedRegistry.Enable(vm)
	scriptV1 := handlerror(os.ReadFile("./assets/runtime_v2.js"))

	handlerror(vm.RunString(string(scriptV1)))
	o := handlerror(vm.RunString("console"))
	log.Println(o)
	handlerror(vm.RunString(*ext.context))

	// vm.Set()
	initModule(reg, vm)
}
