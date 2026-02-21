package jsExtension

import (
	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/console"
	"github.com/dop251/goja_nodejs/require"
	"github.com/dop251/goja_nodejs/url"
	errorhandle "github.com/miru-project/miru-core/pkg/errorHandle"
	log "github.com/miru-project/miru-core/pkg/logger"
)

func initModule() {
	linkeDom := string(errorhandle.HandleFatal(fs.ReadFile("assets/linkedom/worker.js")))
	linkeDomProgram, e := goja.Compile("linkedom.js", linkeDom, true)
	if e != nil {
		log.Println("Error executing linkedom:", e)
	}
	sharedRegistry.RegisterNativeModule("linkedom", func(vm *goja.Runtime, module *goja.Object) {
		// Save global fetch and Element from being overridden by linkedom.js's internal definitions
		savedFetch := vm.Get("fetch")
		savedElement := vm.Get("Element")

		obj := module.Get("exports").(*goja.Object)
		vm.Set("module", module)
		vm.Set("exports", obj)

		if _, err := vm.RunProgram(linkeDomProgram); err != nil {
			log.Println("Error running linkedom:", err)
		}

		// Restore fetch and Element if they were saved
		if savedFetch != nil {
			vm.Set("fetch", savedFetch)
		}
		if savedElement != nil {
			vm.Set("Element", savedElement)
		}

		// Manually export common linkedom exports if they were set globally instead of on module.exports
		exports := []string{"Attr", "CDATASection", "CharacterData", "Comment", "CustomEvent", "DOMParser", "Document", "DocumentFragment", "DocumentType", "Element", "Event", "EventTarget", "Facades", "HTMLAnchorElement", "HTMLAreaElement", "HTMLAudioElement", "HTMLBRElement", "HTMLBaseElement", "HTMLBodyElement", "HTMLButtonElement", "HTMLCanvasElement", "HTMLClasses", "HTMLDListElement", "HTMLDataElement", "HTMLDataListElement", "HTMLDetailsElement", "HTMLDirectoryElement", "HTMLDivElement", "HTMLElement", "HTMLEmbedElement", "HTMLFieldSetElement", "HTMLFontElement", "HTMLFormElement", "HTMLFrameElement", "HTMLFrameSetElement", "HTMLHRElement", "HTMLHeadElement", "HTMLHeadingElement", "HTMLHtmlElement", "HTMLIFrameElement", "HTMLImageElement", "HTMLInputElement", "HTMLLIElement", "HTMLLabelElement", "HTMLLegendElement", "HTMLLinkElement", "HTMLMapElement", "HTMLMarqueeElement", "HTMLMediaElement", "HTMLMenuElement", "HTMLMetaElement", "HTMLMeterElement", "HTMLModElement", "HTMLOListElement", "HTMLObjectElement", "HTMLOptGroupElement", "HTMLOptionElement", "HTMLOutputElement", "HTMLParagraphElement", "HTMLParamElement", "HTMLPictureElement", "HTMLPreElement", "HTMLProgressElement", "HTMLQuoteElement", "HTMLScriptElement", "HTMLSelectElement", "HTMLSlotElement", "HTMLSourceElement", "HTMLSpanElement", "HTMLStyleElement", "HTMLTableCaptionElement", "HTMLTableCellElement", "HTMLTableElement", "HTMLTableRowElement", "HTMLTemplateElement", "HTMLTextAreaElement", "HTMLTimeElement", "HTMLTitleElement", "HTMLTrackElement", "HTMLUListElement", "HTMLUnknownElement", "HTMLVideoElement", "InputEvent", "Node", "NodeFilter", "NodeList", "SVGElement", "ShadowRoot", "Text", "illegalConstructor", "parseHTML", "parseJSON", "toJSON"}
		for i := range exports {
			if val := vm.Get(exports[i]); val != nil && !goja.IsUndefined(val) {
				obj.Set(exports[i], val)
			}
		}

		vm.Set("module", goja.Undefined())
		vm.Set("exports", goja.Undefined())
	})

	cryptoJs := string(errorhandle.HandleFatal(fs.ReadFile("assets/crypto-js/crypto-js.js")))
	cryptoJsProgram, e := goja.Compile("crypto-js.js", cryptoJs, true)
	if e != nil {
		log.Println("Error executing crypto-js:", e)
	}
	sharedRegistry.RegisterNativeModule("crypto-js", func(vm *goja.Runtime, module *goja.Object) {
		initCrypto(vm)
		if _, e := vm.RunProgram(cryptoJsProgram); e != nil {
			log.Println("Error executing crypto-js:", e)
		}
		obj := module.Get("exports").(*goja.Object)
		obj.Set("CryptoJS", vm.Get("CryptoJS"))

	})

	md5 := string(errorhandle.HandleFatal(fs.ReadFile("assets/md5/md5.min.js")))
	md5Program, e := goja.Compile("md5.js", md5, true)
	if e != nil {
		log.Println("Error compiling md5:", e)
	}
	sharedRegistry.RegisterNativeModule("md5", func(vm *goja.Runtime, module *goja.Object) {
		_, e := vm.RunProgram(md5Program)
		if e != nil {
			log.Println("Error executing md5:", e)
		}
		obj := module.Get("exports").(*goja.Object)
		obj.Set("md5", vm.Get("md5"))
	})

	jsencrypt := string(errorhandle.HandleFatal(fs.ReadFile("assets/jsencrypt/jsencrypt.min.js")))
	jsencryptProgram, e := goja.Compile("jsencrypt.min.js", jsencrypt, true)
	if e != nil {
		log.Println("Error compiling jsencrypt:", e)
	}
	sharedRegistry.RegisterNativeModule("jsencrypt", func(vm *goja.Runtime, module *goja.Object) {
		_, e := vm.RunProgram(jsencryptProgram)
		if e != nil {
			log.Println("Error executing jsencrypt:", e)
		}
		obj := module.Get("exports").(*goja.Object)
		obj.Set("JSEncrypt", vm.Get("JSEncrypt"))
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
