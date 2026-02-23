package jsExtension

import (
	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/console"
	"github.com/dop251/goja_nodejs/require"
	"github.com/dop251/goja_nodejs/url"
	errorhandle "github.com/miru-project/miru-core/pkg/errorHandle"
	"github.com/miru-project/miru-core/pkg/logger"
	log "github.com/miru-project/miru-core/pkg/logger"
)

func initModule() {
	linkeDom := string(errorhandle.HandleFatal(fs.ReadFile("assets/linkedom/worker.js")))
	linkeDomProgram, e := goja.Compile("linkedom.js", linkeDom, true)
	vm := goja.New()
	vm.RunProgram(linkeDomProgram)
	parseHtml := vm.Get("parseHTML").Export().(func(goja.FunctionCall) goja.Value)
	logger.Println(parseHtml)
	if e != nil {
		log.Println("Error executing linkedom:", e)
	}
	RegisterJSModule("linkedom", linkeDom, func(vm *goja.Runtime, module *goja.Object) {
	})

	cryptoJs := string(errorhandle.HandleFatal(fs.ReadFile("assets/crypto-js/crypto-js.js")))
	RegisterJSModule("crypto-js", cryptoJs, func(vm *goja.Runtime, module *goja.Object) {
		initCrypto(vm)
		obj := module.Get("exports").(*goja.Object)
		obj.Set("CryptoJS", vm.Get("CryptoJS"))
	})

	md5 := string(errorhandle.HandleFatal(fs.ReadFile("assets/md5/md5.min.js")))
	RegisterJSModule("md5", md5, func(vm *goja.Runtime, module *goja.Object) {
	})

	jsencrypt := string(errorhandle.HandleFatal(fs.ReadFile("assets/jsencrypt/jsencrypt.min.js")))
	RegisterJSModule("jsencrypt", jsencrypt, func(vm *goja.Runtime, module *goja.Object) {
		exports := module.Get("exports")
		if obj, ok := exports.(*goja.Object); ok {
			obj.Set("JSEncrypt", exports)
		}
	})
}

// Init nodeJs module
func (ser *ExtBaseService) addModule(module *require.RequireModule, vm *goja.Runtime, job *Job) {
	initCrypto(vm)
	url.Enable(vm)
	console.Enable(vm)
	vm.Set("require", module.Require)
	ser.initFetch(vm, job)
}
