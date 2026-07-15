package binary

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	jsext "github.com/miru-project/miru-core/pkg/extension/js"
	"github.com/miru-project/miru-core/proto/generate/proto"
	"github.com/stretchr/testify/assert"
)

// vmDisposalExtension uses a module-level variable (counter) to detect whether
// the goja VM is recreated on every call, and the cross-function cache to show
// that the cache -- not the VM -- is what survives between calls. settingKeys /
// Miru are already declared by runtime_v2.js.
const vmDisposalExtension = `// ==MiruExtension==
// @name         VM Disposal Test
// @package      vmdisposal
// @apiVersion   2
// ==/MiruExtension==

let counter = 0;

async function load() {
    Miru.saveCache("token", "secret");
}

async function latest(page) {
    counter++;
    const cached = Miru.getCache("token");
    return [{ title: cached + ":" + counter, url: "https://example.com" }];
}

async function search(keyword, page, filter) {
    // counter is re-initialized to 0 on this fresh VM; only the cache survives.
    const cached = Miru.getCache("token") || "none";
    return [{ title: cached + ":" + counter, url: "https://example.com" }];
}
`

// TestJSVMDisposedPerCallAndCachePersists proves two of the changed behaviours
// for the JS runtime:
//  1. The goja VM is disposed after every execution (so the module-level counter
//     is re-initialized to 0 on every call -> "secret:1" each time, never
//     "secret:2"), and only the compiled program is retained.
//  2. The load() hook runs once at startup (seeding the cross-function cache),
//     and the cache -- not the VM -- is what survives between the disposed calls
//     (the value "secret" is still readable from latest() and search()).
func TestJSVMDisposedPerCallAndCachePersists(t *testing.T) {
	dir := t.TempDir()
	jsPath := filepath.Join(dir, "vmdisposal.js")
	if err := os.WriteFile(jsPath, []byte(vmDisposalExtension), 0644); err != nil {
		t.Fatalf("write vmdisposal.js: %v", err)
	}

	jsext.InitRuntime(dir, jsext.AssetsFS)
	// Give the init event-loop bootstrap a moment to finish before we query.
	time.Sleep(500 * time.Millisecond)

	api := jsext.ApiPkgCache.Load("vmdisposal")
	assert.NotNil(t, api, "the JS extension should be registered after InitRuntime")
	if api != nil {
		assert.Empty(t, api.Ext.Error, "the JS extension should load without a compile/load error")
	}

	// First latest(): fresh VM -> counter 0->1; cache yields "secret".
	r1, err := jsext.Latest[proto.ExtensionListItem]("vmdisposal", 1)
	assert.NoError(t, err)
	assert.Len(t, r1, 1)
	assert.Equal(t, "secret:1", r1[0].Title)

	// Second latest(): VM disposed & recreated -> counter resets 0->1 again.
	// If the VM were reused, counter would be 2.
	r2, err := jsext.Latest[proto.ExtensionListItem]("vmdisposal", 1)
	assert.NoError(t, err)
	assert.Equal(t, "secret:1", r2[0].Title)

	// search() runs on yet another fresh VM: counter is 0 there, but the cache
	// still yields "secret" -> "secret:0". This proves the VM is disposed yet
	// the cross-function cache survives across the disposed VMs.
	rs, err := jsext.Search[proto.ExtensionListItem]("vmdisposal", 1, "kw", "")
	assert.NoError(t, err)
	assert.Len(t, rs, 1)
	assert.Equal(t, "secret:0", rs[0].Title)
}
