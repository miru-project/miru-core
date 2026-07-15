package js

import (
	log "github.com/miru-project/miru-core/pkg/logger"
)

func LoadApiV1(ext *Ext) {

	*ext.Context = replaceClassExtendsDeclaration(*ext.Context)

	// compile extension runtime
	compiledExt, e := compileExtension(ext)
	if e != nil {
		return
	}

	api := &ExtApi{Ext: ext, service: &ExtBaseService{program: compiledExt}}
	ApiPkgCache.Store(ext.Pkg, api)
	ApiPkgCache.SetError(ext.Pkg, "")

	api.initEvalV1String()
	// Run the extension's load() hook once. AsyncCallBack spins up a fresh goja
	// VM for this call and disposes it afterwards; only the compiled program in
	// api.service.program survives. Any state load() wants to keep across calls
	// must go through Miru.saveCache / Miru.getCache.
	api.loadExtensionV1(ext.Pkg)
	log.Println("Extension loaded (V1) [JS]:", ext.Name, ext.Pkg)

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
	api.createFilterEval = "ext.createFilter(%s)"
}
