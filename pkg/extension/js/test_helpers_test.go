package js

import (
	"testing"
	"time"

	"github.com/miru-project/miru-core/pkg/extension"
)

// registerMockExt registers a mock ExtApi with the package cache using an
// in-memory asyncCallBack. This replaces the repeated
// extension.Extension + ExtApi + ApiPkgCache.Store + ApiPkgCache.SetError
// boilerplate used across tests.
func registerMockExt(t *testing.T, pkg, apiVersion string, callback func(api *ExtApi, pkg string, evalStr string) (any, error)) *ExtApi {
	t.Helper()
	ext := &extension.Extension{
		Name:       "Test Extension",
		Pkg:        pkg,
		ApiVersion: apiVersion,
		Website:    "https://example.com",
	}
	api := &ExtApi{
		Ext:           ext,
		asyncCallBack: callback,
	}
	ApiPkgCache.Store(ext.Pkg, api)
	ApiPkgCache.SetError(ext.Pkg, "")
	return api
}

// waitForPkg polls the API cache until pkg is registered or the deadline hits.
// The loader runs in a goroutine, so a freshly called InitRuntime may not have
// registered the package yet.
func waitForPkg(t *testing.T, pkg string) *ExtApi {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		if v, ok := ApiPkgCache.Map.Load(pkg); ok {
			return v.(*ExtApi)
		}
		if time.Now().After(deadline) {
			t.Fatalf("extension %q was not registered within timeout", pkg)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
