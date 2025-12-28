package jsExtension

import "sync"

var extMemMap = sync.Map{}

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
	m.Modify(key, func(ea *ExtApi) *ExtApi {
		ea.Ext.Error = errString
		return ea
	})
}

func (m *ExtMapCache) Remove(key string) {
	m.Map.Delete(key)
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
