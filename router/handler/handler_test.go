package handler

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/miru-project/miru-core/pkg/extension"
	golang "github.com/miru-project/miru-core/pkg/extension/golang"
	js "github.com/miru-project/miru-core/pkg/extension/js"
	"github.com/stretchr/testify/assert"
)

// TestBuildExtensionMetaIncludesJSAndGolang verifies that the frontend
// extension list (built by buildExtensionMeta) contains BOTH the Go/Scriggo
// extensions discovered on disk and the JavaScript extensions held in the JS
// runtime cache. This is the end-to-end check for "the frontend can get the
// extension list for both golang and js".
func TestBuildExtensionMetaIncludesJSAndGolang(t *testing.T) {
	dir := t.TempDir()

	// A minimal but valid Go extension written to disk.
	goSrc := `// ==MiruExtension==
// @name         Miruro
// @package      example.v2
// @apiVersion   1
// ==/MiruExtension==

package miruro

func Load() {}
`
	if err := os.WriteFile(filepath.Join(dir, "example.v2.go"), []byte(goSrc), 0644); err != nil {
		t.Fatalf("write example.v2.go: %v", err)
	}

	prevDir := golang.ExtensionDir
	golang.ExtensionDir = dir
	defer func() { golang.ExtensionDir = prevDir }()

	// Seed a JavaScript extension into the JS runtime cache (as InitRuntime
	// would after compiling a .js file).
	jsPkg := "examplejs"
	js.ApiPkgCache.Store(jsPkg, &js.ExtApi{Ext: &extension.Extension{Name: "ExampleJS", Pkg: jsPkg}})
	defer js.ApiPkgCache.Remove(jsPkg)

	meta := buildExtensionMeta()

	pkgs := map[string]bool{}
	for _, m := range meta {
		pkgs[m.Pkg] = true
		assert.Nil(t, m.Context, "source context must be stripped from the list payload")
	}

	assert.True(t, pkgs["example.v2"], "the Go extension must appear in the frontend list")
	assert.True(t, pkgs[jsPkg], "the JS extension must appear in the frontend list")
}
