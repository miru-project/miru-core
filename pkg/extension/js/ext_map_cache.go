package js

import (
	"sync"

	log "github.com/miru-project/miru-core/pkg/logger"
)

// Cross-call state used to be mirrored into an in-process string store here
// (the old saveCache / getCache surface). It is gone: the Go-native snapshot
// in instance_state.go is the sole carrier of state between calls, and it
// holds full-fidelity values instead of strings. The recorded state for a
// package is dropped alongside its cache entry on reload/remove (see
// deleteInstanceState).

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
	// Drop the recorded cross-call state for the removed package so nothing
	// from the outgoing extension leaks into a later install.
	deleteInstanceState(key)
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
