package golang

import (
	"testing"

	"github.com/miru-project/miru-core/pkg/extension/golang/runtime"
	"github.com/stretchr/testify/assert"
)

// TestHandleReloadInvalidatesCaches verifies the unified watcher's Go reload
// handler drops the per-package cross-call variable cache AND the per-package
// native packages map, so the next request recompiles against the updated
// source instead of reusing stale cached state.
func TestHandleReloadInvalidatesCaches(t *testing.T) {
	const pkg = "reloadtest"

	// Seed cross-call state via the runtime cache.
	runtime.SaveCache(pkg, "token", "stale")
	if _, ok := runtime.GetCache(pkg, "token"); !ok {
		t.Fatalf("precondition failed: cache seed missing")
	}

	// Seed a per-package native packages entry.
	pkgPackages.Store(pkg, packages)
	if _, ok := pkgPackages.Load(pkg); !ok {
		t.Fatalf("precondition failed: pkgPackages seed missing")
	}

	// Act: simulate the watcher firing because the .go file changed.
	HandleReload(pkg)

	// The cross-call cache for the package must be gone.
	_, stillCached := runtime.GetCache(pkg, "token")
	assert.False(t, stillCached, "HandleReload must invalidate the per-package cross-call cache")

	// The per-package native packages map must be gone too.
	_, stillPkg := pkgPackages.Load(pkg)
	assert.False(t, stillPkg, "HandleReload must invalidate the per-package native packages map")
}

// TestHandleReloadUnregisteredPackageIsSafe verifies a reload for a package that
// has no cached state is a no-op rather than a panic.
func TestHandleReloadUnregisteredPackageIsSafe(t *testing.T) {
	const pkg = "neverloaded"
	runtime.DeleteCache(pkg) // ensure clean slate
	pkgPackages.Delete(pkg)

	assert.NotPanics(t, func() {
		HandleReload(pkg)
	})
}
