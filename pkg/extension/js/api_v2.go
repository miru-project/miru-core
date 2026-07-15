package js

import (
	log "github.com/miru-project/miru-core/pkg/logger"
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
	// Run the extension's load() hook once. AsyncCallBack spins up a fresh goja
	// VM for this call and disposes it afterwards; only the compiled program in
	// api.service.program survives, which is exactly what the extension API
	// requires. Any state load() wants to keep across calls must go through
	// Miru.saveCache / Miru.getCache.
	api.loadExtensionV2(ext.Pkg)
	log.Println("Extension loaded (V2) [JS]:", ext.Name, ext.Pkg)
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
