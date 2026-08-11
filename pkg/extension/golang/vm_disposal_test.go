package golang

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// vmDisposalSrc is a Go/Scriggo extension that uses a package-level variable
// (counter) to detect whether the VM is recreated on every call. It also uses
// the cross-function cache (seeded from Load) to show that the cache -- not the
// VM -- is what survives between calls.
const vmDisposalSrc = `// ==MiruExtension==
// @name VM Disposal Test
// @package vmdisposal
// @apiVersion 2
package vmdisposal

import (
	"strconv"

	sdk "github.com/miru-project/miru-core/pkg/extension/golang/sdk"
)

var counter int

func Load() {
	sdk.SaveCache("vmdisposal", "token", "secret")
}

func Latest(pkg string, page int) ([]sdk.ExtensionListItem, error) {
	counter++
	v, _ := sdk.GetCache(pkg, "token")
	return []sdk.ExtensionListItem{{Title: v.(string) + ":" + strconv.Itoa(counter), URL: "https://example.com"}}, nil
}
`

// TestGolangVMDisposedPerCallAndCachePersists proves two of the changed
// behaviours for the Go runtime:
//  1. The VM/Scriggo program is compiled fresh for every call (so the
//     package-level counter is always re-initialized to 0 -> result "secret:1"
//     on every call, never "secret:2").
//  2. The Load hook runs once at startup (seeding the cross-function cache), and
//     the cache -- not the VM -- is what survives between the disposed calls
//     (the value "secret" is still readable from Latest).
func TestGolangVMDisposedPerCallAndCachePersists(t *testing.T) {
	dir := t.TempDir()
	extPath := filepath.Join(dir, "vmdisposal.go")
	if err := os.WriteFile(extPath, []byte(vmDisposalSrc), 0644); err != nil {
		t.Fatalf("write extension source: %v", err)
	}
	ExtensionDir = dir
	// Eagerly load: runs the Load hook once, seeding the cache.
	LoadExtensions()

	// First Latest: fresh VM -> counter 0->1; cache yields "secret".
	r1, err := Latest("vmdisposal", 1)
	assert.NoError(t, err)
	assert.Len(t, r1, 1)
	assert.Equal(t, "secret:1", r1[0].Title)

	// Second Latest: VM is recompiled fresh, counter resets to 0->1 again. If
	// the VM were reused/persisted, counter would be 2. The cache still yields
	// "secret", proving only the compiled result + cache survive.
	r2, err := Latest("vmdisposal", 1)
	assert.NoError(t, err)
	assert.Equal(t, "secret:1", r2[0].Title)
}
