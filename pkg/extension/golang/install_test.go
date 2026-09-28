package golang

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/miru-project/miru-core/pkg/extension"
	"github.com/stretchr/testify/assert"
)

// A .go extension that lands in the extension dir while the app is running is an
// install, not a startup scan: LoadExtensions() already ran, so the file is
// only reachable if the listing and the lazy compile both read the directory on
// demand. This mirrors what js.DownloadExtension writes after a successful
// download (ExtensionDir/<pkg>.go) followed by the watcher's HandleReload.
func TestInstalledGolangExtensionIsListed(t *testing.T) {
	dir := t.TempDir()
	src, err := os.ReadFile(filepath.Join("testdata", "example", "example.go"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "example.go"), src, 0644); err != nil {
		t.Fatalf("write extension source: %v", err)
	}
	prev := ExtensionDir
	ExtensionDir = dir
	t.Cleanup(func() { ExtensionDir = prev })

	// The watcher fires for the newly created file.
	HandleReload("example")

	var found *extension.Extension
	for _, ext := range GetExtensions() {
		if ext.Pkg == "example" {
			found = ext
		}
	}
	if found == nil {
		t.Fatalf("installed extension missing from GetExtensions(): %v", GetExtensions())
	}
	assert.Equal(t, "Example", found.Name)
	assert.Equal(t, "v0.1.0", found.Version)
	assert.Equal(t, extension.LanguageGolang, found.FileLang, "must be routed to the golang runtime")
	assert.Empty(t, found.Error)
}

// The listing is only half of an install: the first search has to compile the
// freshly written source through Scriggo, which is the lazy path
// getPkgFromCache -> ParseExtensionMetadata -> loadExtApi.
func TestInstalledGolangExtensionCompilesOnFirstUse(t *testing.T) {
	dir := t.TempDir()
	src, err := os.ReadFile(filepath.Join("testdata", "example", "example.go"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "example.go"), src, 0644); err != nil {
		t.Fatalf("write extension source: %v", err)
	}
	prev := ExtensionDir
	ExtensionDir = dir
	t.Cleanup(func() {
		ExtensionDir = prev
		HandleReload("example") // drop caches so other tests start clean
	})

	HandleReload("example")

	items, err := Search("example", 1, "test", nil)
	if err != nil {
		t.Fatalf("search in a freshly installed extension failed: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("expected the fixture extension to return results")
	}
}

// The js runtime must not claim a .go file, and the shared directory scan must
// route each suffix to its own runtime.
func TestDirectoryScanRoutesEachSuffixToItsRuntime(t *testing.T) {
	dir := t.TempDir()
	jsSrc := "// ==MiruExtension==\n// @name JS Ext\n// @version v1\n// @package jsext.net\n// @type all\n// ==/MiruExtension==\n"
	goSrc := "// ==MiruExtension==\n// @name GO Ext\n// @version v2\n// @package goext.net\n// @type all\n// ==/MiruExtension==\n"
	if err := os.WriteFile(filepath.Join(dir, "jsext.net.js"), []byte(jsSrc), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "goext.net.go"), []byte(goSrc), 0644); err != nil {
		t.Fatal(err)
	}

	exts, invalid := extension.FilterExtensions(dir)
	assert.Empty(t, invalid, "both files must parse")

	byPkg := map[string]*extension.Extension{}
	for _, e := range exts {
		byPkg[e.Pkg] = e
	}
	if len(byPkg) != 2 {
		t.Fatalf("expected both extensions, got %v", byPkg)
	}
	assert.Equal(t, extension.LanguageJS, byPkg["jsext.net"].FileLang)
	assert.Equal(t, extension.LanguageGolang, byPkg["goext.net"].FileLang)

	// The golang runtime only ever lists .go files.
	prev := ExtensionDir
	ExtensionDir = dir
	t.Cleanup(func() { ExtensionDir = prev })
	for _, e := range GetExtensions() {
		assert.Equal(t, extension.LanguageGolang, e.FileLang)
		assert.NotEqual(t, "jsext.net", e.Pkg, "a .js file must not be listed by the golang runtime")
	}
}
