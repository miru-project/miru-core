package binary

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/miru-project/miru-core/pkg/extension"
	golang "github.com/miru-project/miru-core/pkg/extension/golang"
	jsext "github.com/miru-project/miru-core/pkg/extension/js"
	"github.com/miru-project/miru-core/proto/generate/proto"
	"github.com/stretchr/testify/assert"
)

// exampleJSExtension is a minimal but valid Miru v1 JavaScript extension that
// mirrors the shipped Go example extension. It returns static data (no
// network) so the full load pipeline can be exercised deterministically.
//
// The loader requires the file name to equal "<package>.js", so the @package
// must be "example" and the file must be named "example.js".
const exampleJSExtension = `// ==MiruExtension==
// @name         Example
// @version      v0.1.0
// @author       AUTHOR_NAME
// @lang         all
// @license      MIT
// @icon         YOUR_LINK_TO_ICON
// @package      example
// @type         bangumi
// @webSite      WEB_LINK
// @nsfw         false
// @apiVersion   1
// ==/MiruExtension==

class Example extends Extension {
    async search(keyword, page, filter) {
        return [
            { title: "Example Result 1", url: "https://example.com/1", cover: "https://example.com/1.jpg" },
            { title: "Example Result 2", url: "https://example.com/2", cover: "https://example.com/2.jpg" }
        ];
    }
    async latest(page) {
        return [];
    }
    async detail(url) {
        return { title: "Example Detail", url: url, cover: "", desc: "" };
    }
    async watch(url) {
        return { type: "bangumi", url: url, groups: [ { title: "Group 1", mirrors: [ { name: "Mirror 1", url: url } ] } ] };
    }
    async mirror(url) {
        return [ { name: "Mirror 1", url: url } ];
    }
}
`

// strayGoExtension is a Go/Scriggo source file placed in the same directory as
// the JS extension. It must NEVER be compiled by the JavaScript runtime. This
// reproduces the originally reported failure: example.v2.go (a Go extension)
// was being handed to goja, producing
// `SyntaxError: example.v2.js: Line 26:9 Unexpected identifier`.
const strayGoExtension = `// ==MiruExtension==
// @package      testgo.org
// ==/MiruExtension==

package testgo

func Foo() string {
	return "i am go, not js"
}
`

// TestExampleJSExtensionLoadsViaEntryPoint loads a JavaScript extension through
// the shared pkg/extension entry point (FilterExtensions for discovery/routing)
// and the JS runtime (InitRuntime) using the embedded assets FS. It also places
// a Go extension in the same directory and asserts the JS runtime ignores it,
// which is the regression for the reported misrouting bug.
func TestExampleJSExtensionLoadsViaEntryPoint(t *testing.T) {
	dir := t.TempDir()

	jsPath := filepath.Join(dir, "example.js")
	if err := os.WriteFile(jsPath, []byte(exampleJSExtension), 0644); err != nil {
		t.Fatalf("write example.js: %v", err)
	}
	goPath := filepath.Join(dir, "testgo.org.go")
	if err := os.WriteFile(goPath, []byte(strayGoExtension), 0644); err != nil {
		t.Fatalf("write testgo.org.go: %v", err)
	}

	// Entry point: discover and route every extension source file by language.
	exts, invalid := extension.FilterExtensions(dir)
	assert.Empty(t, invalid, "neither example file should be reported invalid")
	assert.Len(t, exts, 2, "expected the .js and .go example extensions")

	byPkg := map[string]*extension.Extension{}
	for _, e := range exts {
		byPkg[e.Pkg] = e
	}
	assert.Equal(t, extension.LanguageJS, byPkg["example"].FileLang,
		"example.js must be routed to the JavaScript runtime")
	assert.Equal(t, extension.LanguageGolang, byPkg["testgo.org"].FileLang,
		"testgo.org.go must be routed to the Golang runtime, not JavaScript")

	// Load the directory via the JS runtime (f is the embedded assets FS from
	// lib.go). The JS runtime must only compile .js files.
	jsext.InitRuntime(dir, jsext.AssetsFS)

	// loadExtApi runs asynchronously inside InitRuntime; give the goja compile
	// + event-loop bootstrap a moment to finish before we query.
	time.Sleep(500 * time.Millisecond)

	// The JavaScript example must have loaded and registered cleanly.
	api := jsext.ApiPkgCache.Load("example")
	assert.NotNil(t, api, "the JS example should be registered after InitRuntime")
	if api != nil {
		assert.Empty(t, api.Ext.Error, "the JS example should load without a compile/load error")
	}

	// Regression: the stray Go file must NOT have been compiled by goja and
	// registered as a (broken) JS extension. Before the fix, testgo.org.go was
	// fed to the JS compiler, producing a SyntaxError entry under pkg
	// "testgo.org". ApiPkgCache.Load panics on a missing key, so check presence
	// via the embedded sync.Map directly.
	_, goLoaded := jsext.ApiPkgCache.Map.Load("testgo.org")
	assert.False(t, goLoaded, "a .go extension must never be compiled by the JS runtime")

	// The JS example must serve results through the public Search API.
	results, err := jsext.Search[proto.ExtensionListItem]("example", 1, "test", "")
	assert.NoError(t, err)
	assert.NotNil(t, results)
	assert.GreaterOrEqual(t, len(results), 1, "the JS example should serve at least one search result")
	assert.Equal(t, "Example Result 1", results[0].Title)
}

// TestLoadExtensionsFromSharedFolder proves that a single extension directory
// containing BOTH a Go extension (example.v2.go) and a JavaScript extension
// (example.js) is read by the entry point and routed to the correct runtime:
//   - the Go runtime loads example.v2.go and prints "[GO]"
//   - the JS runtime loads example.js and prints "[JS]"
//   - the JS runtime strictly ignores the .go file (no misrouting)
//
// This is the end-to-end answer to "does the folder get read and does the
// example extension load correctly?": both runtimes load their respective
// extension from the very same folder.
func TestLoadExtensionsFromSharedFolder(t *testing.T) {
	dir := t.TempDir()

	// A real Go extension, copied verbatim from the shipped source.
	goSrc, err := os.ReadFile(filepath.Join("..", "pkg", "extension", "golang", "extensions", "miruro", "examplev2.go"))
	if err != nil {
		t.Fatalf("read examplev2.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "examplev2.go"), goSrc, 0644); err != nil {
		t.Fatalf("write examplev2.go: %v", err)
	}

	// A JS extension in the same directory.
	if err := os.WriteFile(filepath.Join(dir, "example.js"), []byte(exampleJSExtension), 0644); err != nil {
		t.Fatalf("write example.js: %v", err)
	}

	// Go runtime: load its own extensions from the shared folder.
	prev := golang.ExtensionDir
	golang.ExtensionDir = dir
	defer func() { golang.ExtensionDir = prev }()
	golang.LoadExtensions()

	// JS runtime: load from the same folder; it must ignore the .go file.
	jsext.InitRuntime(dir, jsext.AssetsFS)
	time.Sleep(500 * time.Millisecond)

	// JS side: example loaded, examplev2 strictly NOT compiled as JS.
	api := jsext.ApiPkgCache.Load("example")
	assert.NotNil(t, api, "the JS extension must be loaded by the JS runtime")
	_, miruroInJS := jsext.ApiPkgCache.Map.Load("examplev2")
	assert.False(t, miruroInJS, "examplev2.go must be skipped by the JS runtime")

	// GO side: examplev2.go from the given folder compiles and loads via the
	// Go runtime (deterministic, no network).
	ext, perr := golang.ParseExtensionMetadata("examplev2")
	assert.NoError(t, perr, "examplev2.go metadata must parse")
	rt := golang.NewRuntime(golang.NewScriggoVM(nil))
	assert.NoError(t, rt.LoadExtension(ext), "examplev2.go must load as a Go extension")
}
