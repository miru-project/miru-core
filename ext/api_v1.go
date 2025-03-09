package ext

import (
	"os"

	"github.com/dop251/goja"
)

type ExtApiV1 struct {
	runtime *goja.Runtime
	ext     Ext
}

func (api *ExtApiV1) loadApiV1() {
	vm := api.runtime
	scriptV1 := handlerror(os.ReadFile("./assets/runtime_v1.js"))

	handlerror(vm.RunString(string(scriptV1)))
}
