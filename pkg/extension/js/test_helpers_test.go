package js

import (
	"testing"

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
