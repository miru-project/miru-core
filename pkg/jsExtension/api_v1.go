package jsExtension

import (
	log "github.com/miru-project/miru-core/pkg/logger"

	"github.com/dop251/goja"
	errorhandle "github.com/miru-project/miru-core/pkg/errorHandle"
)

func LoadApiV1(ext *Ext, baseScript string) {

	*ext.Context = replaceClassExtendsDeclaration(*ext.Context)
	// compile base runtime
	runtimeV1 := errorhandle.HandleFatal(goja.Compile("runtime_v1.js", baseScript, true))
	// compile extension runtime
	compiledExt, e := compileExtension(ext)
	if e != nil {
		return
	}

	api := &ExtApi{Ext: ext, service: &ExtBaseService{base: runtimeV1, program: compiledExt}}
	ApiPkgCache.Store(ext.Pkg, api)
	ApiPkgCache.SetError(ext.Pkg, "")

	api.initEvalV1String()
	api.initRuntimeV1(ext.Pkg)
	api.loadExtensionV1(ext.Pkg)
	log.Println("Extension loaded (V1):", ext.Name, ext.Pkg)

}

func (api *ExtApi) loadExtensionV1(pkg string) {
	if _, e := AsyncCallBack(api, pkg, "ext.load()"); e != nil {
		ApiPkgCache.SetError(pkg, e.Error())
	}
}

func (api *ExtApi) initEvalV1String() {
	// Register  the async callback function for V1
	api.asyncCallBack = AsyncCallBack
	api.latestEval = "ext.latest(%d)"
	api.searchEval = "ext.search('%s', %d, %s)"
	api.detailEval = "ext.detail('%s')"
	api.watchEval = "ext.watch('%s')"
}
