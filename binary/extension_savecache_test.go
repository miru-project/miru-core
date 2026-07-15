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

// saveCacheExtension is a minimal V2 JS extension that stores a value in the
// cross-function cache from load() and reads it back from latest(). Because the
// goja VM is disposed after every call, Miru.saveCache / Miru.getCache are the
// only way state survives between load() and latest(); this exercises that path
// end to end. settingKeys / Miru are already declared by runtime_v2.js, so the
// extension must not redeclare them.
const saveCacheExtension = `// ==MiruExtension==
// @name         SaveCache Test
// @package      savecachetest
// @apiVersion   2
// ==/MiruExtension==

async function load() {
    Miru.saveCache("token", "secret");
}

async function latest(page) {
    const v = Miru.getCache("token");
    return [{ title: v, url: "https://example.com" }];
}
`

// TestJSSaveCacheRoundTrip proves that a value written to the cross-function
// cache in load() (executed on one fresh goja VM) is still readable from
// latest() (executed on a different fresh goja VM). It is the JavaScript
// counterpart of golang's TestLoadSeedsCacheAndSearchReadsIt test.
func TestJSSaveCacheRoundTrip(t *testing.T) {
	dir := t.TempDir()
	jsPath := filepath.Join(dir, "savecachetest.js")
	if err := os.WriteFile(jsPath, []byte(saveCacheExtension), 0644); err != nil {
		t.Fatalf("write savecachetest.js: %v", err)
	}

	jsext.InitRuntime(dir, jsext.AssetsFS)
	// Give the init event-loop bootstrap a moment to finish before we query.
	time.Sleep(500 * time.Millisecond)

	api := jsext.ApiPkgCache.Load("savecachetest")
	assert.NotNil(t, api, "the JS extension should be registered after InitRuntime")
	if api != nil {
		assert.Empty(t, api.Ext.Error, "the JS extension should load without a compile/load error")
	}

	results, err := jsext.Latest[proto.ExtensionListItem]("savecachetest", 1)
	assert.NoError(t, err)
	assert.NotNil(t, results)
	assert.Len(t, results, 1, "latest() should return the cached value as a single item")
	assert.Equal(t, "secret", results[0].Title)
}
