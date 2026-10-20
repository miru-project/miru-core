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

	// Seed cross-call state the way the generated capture bridge stores it.
	seed := "stale"
	runtime.MiruStatePut(pkg, "__miru_var.token", &seed)
	var got string
	if !runtime.MiruStateGet(pkg, "__miru_var.token", &got) || got != "stale" {
		t.Fatalf("precondition failed: state seed missing")
	}

	// Seed a per-package native packages entry.
	pkgPackages.Store(pkg, packages)
	if _, ok := pkgPackages.Load(pkg); !ok {
		t.Fatalf("precondition failed: pkgPackages seed missing")
	}

	// Act: simulate the watcher firing because the .go file changed.
	HandleReload(pkg)

	// The captured cross-call state for the package must be gone.
	var still string
	assert.False(t, runtime.MiruStateGet(pkg, "__miru_var.token", &still),
		"HandleReload must invalidate the per-package captured state")

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
