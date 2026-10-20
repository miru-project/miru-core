package golang

import (
	"os"
	"path/filepath"
	"testing"

	rt "github.com/miru-project/miru-core/pkg/extension/golang/runtime"
	"github.com/stretchr/testify/assert"
)

// vmDisposalSrc is a Go/Scriggo extension that uses package-level variables
// only (no cache API) to prove that state survives the per-call VM recycling:
// the VM is still compiled fresh on every call, but the host captures the
// package variables after each call and restores them before the next one, so
// counter keeps counting and the Load-seeded token persists.
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
var token string

func Load() {
	token = "secret"
}

func Latest(pkg string, page int) ([]sdk.ExtensionListItem, error) {
	counter++
	return []sdk.ExtensionListItem{{Title: token + ":" + strconv.Itoa(counter), URL: "https://example.com"}}, nil
}
`

// TestGolangVMDisposedPerCallAndCachePersists proves the state semantics of
// the Go runtime:
//  1. Every call still runs a FRESH compile of the extension source (the VM is
//     disposed between calls), yet the package-level counter PERSISTS across
//     calls ("secret:1" then "secret:2") because the host captures the
//     package variables after each call and restores them into the next one --
//     the same model as the JavaScript runtime.
//  2. The Load hook runs once at startup (seeding the token package
//     variable), and its captured value is readable from every later call
//     ("secret").
func TestGolangVMDisposedPerCallAndCachePersists(t *testing.T) {
	dir := t.TempDir()
	extPath := filepath.Join(dir, "vmdisposal.go")
	if err := os.WriteFile(extPath, []byte(vmDisposalSrc), 0644); err != nil {
		t.Fatalf("write extension source: %v", err)
	}
	ExtensionDir = dir
	rt.DeleteCache("vmdisposal")
	t.Cleanup(func() { rt.DeleteCache("vmdisposal") })
	// Eagerly load: runs the Load hook once, seeding the cache.
	LoadExtensions()

	// First Latest: fresh VM -> counter 0->1; cache yields "secret".
	r1, err := Latest("vmdisposal", 1)
	assert.NoError(t, err)
	assert.Len(t, r1, 1)
	assert.Equal(t, "secret:1", r1[0].Title)

	// Second Latest: the VM is recompiled fresh, but the captured package
	// variables are restored first, so the counter continues at 2. The cache
	// still yields "secret": only the compiled result is disposed, both the
	// captured vars and the cache survive.
	r2, err := Latest("vmdisposal", 1)
	assert.NoError(t, err)
	assert.Equal(t, "secret:2", r2[0].Title)
}
