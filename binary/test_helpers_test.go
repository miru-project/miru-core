package binary

import (
	"testing"
	"time"

	jsext "github.com/miru-project/miru-core/pkg/extension/js"
	"github.com/stretchr/testify/require"
)

// loadExtApi reads the registered extension API for pkg without panicking on a
// missing key. jsext.ApiPkgCache.Load type-asserts unconditionally, so we read
// the underlying sync.Map directly to detect "not yet registered" cleanly.
func loadExtApi(pkg string) *jsext.ExtApi {
	if v, ok := jsext.ApiPkgCache.Map.Load(pkg); ok {
		if api, ok := v.(*jsext.ExtApi); ok {
			return api
		}
	}
	return nil
}

// waitForExtension polls the JS runtime cache until the package is registered
// (the load + goja compile + event-loop bootstrap run asynchronously inside
// InitRuntime) or a short timeout elapses. It replaces the previously used
// fixed `time.Sleep` waits, which made the load tests flaky under slow CI.
func waitForExtension(t *testing.T, pkg string) *jsext.ExtApi {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if api := loadExtApi(pkg); api != nil {
			return api
		}
		if time.Now().After(deadline) {
			t.Fatalf("extension %q was not registered within timeout", pkg)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// requireLoaded asserts the extension registered without a compile/load error,
// failing the test (rather than a soft assertion) so a broken load surfaces
// loudly and at a stable point in the test.
func requireLoaded(t *testing.T, pkg string) {
	t.Helper()
	api := waitForExtension(t, pkg)
	require.Empty(t, api.Ext.Error, "extension %q should load without a compile/load error", pkg)
}
