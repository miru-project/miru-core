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

// testJSExtension is a minimal but valid Miru v1 JavaScript extension. It
// overrides the Extension base class methods and returns static data (no
// network), which is enough to exercise the full loader pipeline:
// filename discovery -> goja compile -> event-loop bootstrap -> registration.
//
// The loader requires the file name to be two segments (e.g. "name.site.js")
// AND equal to "<package>.js", so the @package must be "testjs.tv" and the
// file must be named "testjs.tv.js".
const testJSExtension = `// ==MiruExtension==
// @name         TestJS
// @version      v1.0.0
// @author       test
// @package      testjs.tv
// @apiVersion   1
// @webSite      https://example.com
// @license      MIT
// ==/MiruExtension==

class TestJS extends Extension {
    async search(keyword, page, filter) {
        return [
            { title: "Test JS Result", url: "https://example.com/1", cover: "https://example.com/1.jpg" }
        ];
    }
    async latest(page) {
        return [];
    }
    async detail(url) {
        return { title: "Test JS Detail", url: url, cover: "", desc: "" };
    }
    async watch(url) {
        return { type: "anime", url: url, headers: {}, datas: [] };
    }
    async mirror(url) {
        return url;
    }
}
`

// TestJSExtensionLoadsAndServes verifies the JavaScript extension runtime can
// load a real .js extension file from disk and serve results through the
// public Search API (the same path the router/handler uses).
func TestJSExtensionLoadsAndServes(t *testing.T) {
	dir := t.TempDir()
	jsPath := filepath.Join(dir, "testjs.tv.js")
	if err := os.WriteFile(jsPath, []byte(testJSExtension), 0644); err != nil {
		t.Fatalf("write test js extension: %v", err)
	}

	// f is the embedded assets FS (runtime_v1.js etc.) defined in lib.go.
	jsext.InitRuntime(dir, jsext.AssetsFS)

	// loadExtApi runs asynchronously inside InitRuntime; give the goja compile
	// + event-loop bootstrap a moment to finish before we query.
	time.Sleep(500 * time.Millisecond)

	pkg := "testjs.tv"
	api := jsext.ApiPkgCache.Load(pkg)
	assert.NotNil(t, api, "extension should be registered in the cache after InitRuntime")
	if api != nil {
		assert.Empty(t, api.Ext.Error, "extension should load without a compile/load error")
	}

	results, err := jsext.Search[proto.ExtensionListItem](pkg, 1, "test", "")
	assert.NoError(t, err)
	assert.NotNil(t, results)
	assert.GreaterOrEqual(t, len(results), 1, "extension should serve at least one search result")
	assert.Equal(t, "Test JS Result", results[0].Title)
}
