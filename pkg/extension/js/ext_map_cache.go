package js

import (
	"sync"

	log "github.com/miru-project/miru-core/pkg/logger"
)

// extVarCache is the cross-function / cross-call variable store shared by all
// functions of an extension. Because the goja VM is now disposed after every
// execution (see AsyncCallBack), any long-lived state an extension wants to
// share between its functions -- for example a value computed once in load()
// and read later by latest()/search()/detail() -- must live OUTSIDE the VM.
// saveExtVar/getExtVar (exposed to JS as Miru.saveCache / Miru.getCache) read
// and write this map. Values are stored as strings because a string is the only
// goja value that can be carried safely across VM instances.
//
// Structure: pkg -> (key -> value).
var extVarCache sync.Map

// saveExtVar stores a string value under (pkg, key) in the cross-function store.
func saveExtVar(pkg, key, value string) {
	m, _ := extVarCache.LoadOrStore(pkg, &sync.Map{})
	m.(*sync.Map).Store(key, value)
}

// getExtVar reads a string value stored under (pkg, key). The second return
// value reports whether the key was present.
func getExtVar(pkg, key string) (string, bool) {
	m, ok := extVarCache.Load(pkg)
	if !ok {
		return "", false
	}
	v, ok := m.(*sync.Map).Load(key)
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

// deleteExtVarCache drops every cached variable for a package. It is called when
// an extension is reloaded or removed so stale values from a previous version do
// not leak into the new one.
func deleteExtVarCache(pkg string) {
	extVarCache.Delete(pkg)
}

type ExtMapCache struct {
	sync.Map
}

var ApiPkgCache = &ExtMapCache{sync.Map{}}

var OnExtensionUpdate func([]*ExtApi)

func (m *ExtMapCache) Load(key string) *ExtApi {
	val, _ := m.Map.Load(key)
	return val.(*ExtApi)
}
func (m *ExtMapCache) Store(key string, value *ExtApi) {
	m.Map.Store(key, value)
	m.notify()
}
func (m *ExtMapCache) Modify(key string, f func(*ExtApi) *ExtApi) {
	val, _ := m.Map.Load(key)
	m.Map.Store(key, f(val.(*ExtApi)))
	m.notify()
}
func (m *ExtMapCache) SetError(key string, errString string) {
	if errString != "" {
		log.Println("Extension Error", key, errString)
	}
	m.Modify(key, func(ea *ExtApi) *ExtApi {
		ea.Ext.Error = errString
		return ea
	})
}

func (m *ExtMapCache) Remove(key string) {
	m.Map.Delete(key)
	// Drop cross-function variables for the removed package.
	deleteExtVarCache(key)
	m.notify()
}

func (m *ExtMapCache) notify() {
	if OnExtensionUpdate != nil {
		OnExtensionUpdate(m.GetAll())
	}
}

func (m *ExtMapCache) GetAll() []*ExtApi {
	var exts []*ExtApi
	ApiPkgCache.Map.Range(func(key, value any) bool {
		exts = append(exts, value.(*ExtApi))
		return true
	})
	return exts
}
