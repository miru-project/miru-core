package golang

import (
	"os"
	"path/filepath"
	"testing"

	sdk "github.com/miru-project/miru-core/pkg/extension/golang/sdk"
)

// TestLoadSeedsCacheAndSearchReadsIt verifies the cross-function variable
// store end to end. The Load hook (run once at startup, taking no arguments)
// writes a value via sdk.SaveCache, and a later Search call -- which compiles
// and runs a FRESH Scriggo VM -- reads it back via sdk.GetCache. This proves
// the state survives the per-call VM disposal, exactly as the extension API
// requires. The store is keyed by package name then key, matching the layout
// used by the JavaScript runtime.
func TestLoadSeedsCacheAndSearchReadsIt(t *testing.T) {
	dir := t.TempDir()
	const pkg = "cacheext"
	src := `// ==MiruExtension==
// @name Cache Test
// @package cacheext
// @apiVersion 2
package cacheext

import sdk "github.com/miru-project/miru-core/pkg/extension/golang/sdk"

func Load() {
	sdk.SaveCache("cacheext", "token", "secret")
}

func Search(pkg, kw string, page int, filter sdk.Filter) ([]sdk.ExtensionListItem, error) {
	v, ok := sdk.GetCache(pkg, "token")
	if !ok {
		return nil, nil
	}
	return []sdk.ExtensionListItem{{Title: v.(string), URL: "https://example.com"}}, nil
}
`

	extPath := filepath.Join(dir, pkg+".go")
	if err := os.WriteFile(extPath, []byte(src), 0644); err != nil {
		t.Fatalf("write extension source: %v", err)
	}
	ExtensionDir = dir

	// Eagerly load the extension: this runs Load() once, seeding the cache.
	LoadExtensions()

	items, err := Search(pkg, 1, "kw", nil)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Title != "secret" {
		t.Fatalf("expected cached value %q, got %q", "secret", items[0].Title)
	}
}

// TestCrossFunctionCacheStoreGet exercises the store directly: a value written
// by one function must be readable by another, proving the package->key->value
// layout works.
func TestCrossFunctionCacheStoreGet(t *testing.T) {
	const pkg = "cachepkg"
	sdk.SaveCache(pkg, "a", 42)
	sdk.SaveCache(pkg, "b", "hello")

	if v, ok := sdk.GetCache(pkg, "a"); !ok || v.(int) != 42 {
		t.Fatalf("GetCache(a) = %v, %v; want 42, true", v, ok)
	}
	if v, ok := sdk.GetCache(pkg, "b"); !ok || v.(string) != "hello" {
		t.Fatalf("GetCache(b) = %v, %v; want hello, true", v, ok)
	}
	if _, ok := sdk.GetCache(pkg, "missing"); ok {
		t.Fatalf("GetCache(missing) ok = true; want false")
	}
}
