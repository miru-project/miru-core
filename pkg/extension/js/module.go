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
		// md5.min.js ends with `module.exports = { md5: md5 }`, so a bare
		// require('md5') is an OBJECT and not callable -- an author hits a
		// confusing "TypeError: not a function" on the very first call.
		// Replace the export with a callable that delegates to the module's
		// md5 function while preserving every other property, so both
		// `require('md5')('abc')` and `require('md5').md5('abc')` work.
		exports := module.Get("exports")
		obj, ok := exports.(*goja.Object)
		if !ok {
			return
		}
		fnVal := obj.Get("md5")
		if _, ok := goja.AssertFunction(fnVal); !ok {
			return
		}
		wrapped, err := vm.RunString(`
			(function (inner, source) {
				const out = function () {
					return inner.apply(null, arguments);
				};
				for (const k of Object.keys(source)) {
					out[k] = source[k];
				}
				out.md5 = inner;
				out.default = inner;
				return out;
			})
		`)
		if err != nil {
			log.Println("Error wrapping md5 module:", err)
			return
		}
		makeCallable, ok := goja.AssertFunction(wrapped)
		if !ok {
			return
		}
		result, err := makeCallable(goja.Undefined(), obj.Get("md5"), obj)
		if err != nil {
			log.Println("Error wrapping md5 module:", err)
			return
		}
		module.Set("exports", result)
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
