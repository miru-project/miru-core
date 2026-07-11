package js

import (
	"fmt"
	"time"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/console"
	"github.com/dop251/goja_nodejs/require"
	"github.com/dop251/goja_nodejs/url"
	errorhandle "github.com/miru-project/miru-core/pkg/errorHandle"
	"github.com/miru-project/miru-core/pkg/event"
	log "github.com/miru-project/miru-core/pkg/logger"
	"github.com/miru-project/miru-core/proto/generate/proto"
)

type DevPrinter struct {
	Pkg string
}

func (p *DevPrinter) Log(s string) {
	event.SendDevLog(&proto.DevLogEvent{
		Package:   p.Pkg,
		Message:   s,
		Level:     "info",
		Timestamp: time.Now().UnixMilli(),
	})
	log.Println(fmt.Sprintf("[%s] %s", p.Pkg, s))
}

func (p *DevPrinter) Warn(s string) {
	event.SendDevLog(&proto.DevLogEvent{
		Package:   p.Pkg,
		Message:   s,
		Level:     "warn",
		Timestamp: time.Now().UnixMilli(),
	})
	log.Println(fmt.Sprintf("[%s] WARN: %s", p.Pkg, s))
}

func (p *DevPrinter) Error(s string) {
	event.SendDevLog(&proto.DevLogEvent{
		Package:   p.Pkg,
		Message:   s,
		Level:     "error",
		Timestamp: time.Now().UnixMilli(),
	})
	log.Println(fmt.Sprintf("[%s] ERROR: %s", p.Pkg, s))
}

func initModule() {
	linkeDom := string(errorhandle.HandleFatal(fs.ReadFile("assets/linkedom/worker.js")))
	linkeDomProgram, e := goja.Compile("linkedom.js", linkeDom, true)
	vm := goja.New()
	vm.RunProgram(linkeDomProgram)

	parseHtmlVal := vm.Get("parseHTML")
	if parseHtmlVal != nil && !goja.IsUndefined(parseHtmlVal) {
		if _, ok := parseHtmlVal.Export().(func(goja.FunctionCall) goja.Value); ok {
			log.Println("parseHTML loaded")
		}
	}

	if e != nil {
		log.Println("Error executing linkedom:", e)
	}
	RegisterJSModule("linkedom", linkeDom, func(vm *goja.Runtime, module *goja.Object) {
	})

	cryptoJs := string(errorhandle.HandleFatal(fs.ReadFile("assets/crypto-js/crypto-js.js")))
	RegisterJSModule("crypto-js", cryptoJs, func(vm *goja.Runtime, module *goja.Object) {
		initCrypto(vm)
		exports := module.Get("exports")
		if obj, ok := exports.(*goja.Object); ok {
			obj.Set("CryptoJS", vm.Get("CryptoJS"))
		}
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

	// Register the zlib module as a require-able native module
	RegisterZlibModule(sharedRegistry)
}

// Init nodeJs module
func (api *ExtApi) addModule(module *require.RequireModule, vm *goja.Runtime, job *Job) {
	initCrypto(vm)
	url.Enable(vm)
	consoleObj := vm.NewObject()
	exportsObj := vm.NewObject()
	consoleObj.Set("exports", exportsObj)
	console.RequireWithPrinter(&DevPrinter{Pkg: api.Ext.Pkg})(vm, consoleObj)
	vm.Set("console", exportsObj)
	vm.Set("require", module.Require)
	api.initFetch(vm, job)
}