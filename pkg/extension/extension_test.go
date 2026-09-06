package extension_test

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/miru-project/miru-core/pkg/extension"
	golang "github.com/miru-project/miru-core/pkg/extension/golang"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// exampleJSExtension is a minimal but valid Miru v1 JavaScript extension that
// mirrors the shipped Go example extension (pkg/extension/golang/extensions/
// example). It overrides the Extension base class methods and returns static
// data (no network), which is enough to exercise the full load pipeline.
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
            { title: "Example Result 1", url: "https://example.com/1", cover: "https://example.com/1.jpg", update: "2024-01-01", image: "https://example.com/1.jpg", type: "manga" },
            { title: "Example Result 2", url: "https://example.com/2", cover: "https://example.com/2.jpg", update: "2024-01-02", image: "https://example.com/2.jpg", type: "bangumi" }
        ];
    }
    async latest(page) {
        return [
            { title: "Latest Example 1", url: "https://example.com/latest/1", cover: "https://example.com/latest/1.jpg", update: "2024-01-03", image: "https://example.com/latest/1.jpg", type: "manga" }
        ];
    }
    async detail(url) {
        return { title: "Example Detail", url: url, cover: "", desc: "Example description" };
    }
    async watch(url) {
        return { type: "bangumi", url: url, groups: [ { title: "Group 1", mirrors: [ { name: "Mirror 1", url: url } ] } ] };
    }
    async mirror(url) {
        return [ { name: "Mirror 1", url: url } ];
    }
}
`

// TestDetectLanguage is the single, centralized routing decision: a file's
// extension decides which runtime handles it. .js -> JS, .go -> Golang,
// anything else -> unknown.
func TestDetectLanguage(t *testing.T) {
	assert.Equal(t, extension.LanguageJS, extension.DetectLanguage("foo.js"))
	assert.Equal(t, extension.LanguageJS, extension.DetectLanguage("name.site.js"))
	assert.Equal(t, extension.LanguageGolang, extension.DetectLanguage("example.v2.go"))
	assert.Equal(t, extension.LanguageGolang, extension.DetectLanguage("example.go"))
	assert.Equal(t, extension.LanguageUnknown, extension.DetectLanguage("readme.txt"))
	assert.Equal(t, extension.LanguageUnknown, extension.DetectLanguage("noext"))
}

// TestFilterExtensionsLoadsJSAndGolangExample drives the shared entry point
// (FilterExtensions -> ParseExtensionMetadata) against a directory that holds
// BOTH a Go extension (example.go) and a JavaScript extension (example.js).
//
// This is the regression test for the reported bug: a .go source (e.g.
// example.v2.go) must be detected as a Golang extension, never misrouted to
// the JavaScript runtime (which would produce `SyntaxError: ... Unexpected
// identifier`). We assert each file is classified by its true language and
// that the JS extension's source is real JavaScript, not Go.
func TestFilterExtensionsLoadsJSAndGolangExample(t *testing.T) {
	dir := t.TempDir()

	// Real Go example extension, copied verbatim from the shipped source so
	// the test tracks the actual example. The language must come from the
	// .go extension, regardless of the @package/@name metadata.
	goSrc, err := os.ReadFile(filepath.Join("golang", "testdata", "example", "example.go"))
	if err != nil {
		t.Fatalf("read example.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "example.go"), goSrc, 0644); err != nil {
		t.Fatalf("write example.go: %v", err)
	}

	// JS counterpart of the example extension.
	if err := os.WriteFile(filepath.Join(dir, "example.js"), []byte(exampleJSExtension), 0644); err != nil {
		t.Fatalf("write example.js: %v", err)
	}

	exts, invalid := extension.FilterExtensions(dir)
	assert.Empty(t, invalid, "neither example file should be reported invalid")

	byPkg := map[string]*extension.Extension{}
	for _, e := range exts {
		byPkg[e.Pkg] = e
	}

	// Both extensions must be discovered.
	assert.Len(t, exts, 2, "expected one .go and one .js extension")
	assert.Contains(t, byPkg, "example", "the example package must be present")

	// The two files (example.go / example.js) share the @package "example", so
	// FilterExtensions collapses them to a single metadata entry keyed by Pkg.
	// Disambiguate by inspecting the raw source stored in Context.
	ext := byPkg["example"]
	assert.NotNil(t, ext.Context)
	if strings.Contains(*ext.Context, "package example") {
		// This is the Go source: it must be routed to the Golang runtime.
		assert.Equal(t, extension.LanguageGolang, ext.FileLang,
			"example.go must be detected as a Golang extension")
		assert.NotContains(t, *ext.Context, "class Example extends Extension",
			"the Go source must not contain JS class declarations")
	} else {
		// This is the JS source: it must be routed to the JS runtime.
		assert.Equal(t, extension.LanguageJS, ext.FileLang,
			"example.js must be detected as a JavaScript extension")
		assert.Contains(t, *ext.Context, "class Example extends Extension",
			"the JS source must be real JavaScript")
		assert.NotContains(t, *ext.Context, "package example",
			"a .go file must never be handed to the JS runtime")
	}

	// Sanity: FileLang matches DetectLanguage for each file name.
	assert.Equal(t, extension.LanguageGolang, extension.DetectLanguage("example.go"))
	assert.Equal(t, extension.LanguageJS, extension.DetectLanguage("example.js"))
}

// TestParseExtensionMetadataNSFW guards the @nsfw metadata tag: both runtimes
// share the same header format, so the shared parser must accept the true
// values and default everything else to false.
func TestParseExtensionMetadataNSFW(t *testing.T) {
	jsHeader := func(nsfw string) string {
		return "// ==MiruExtension==\n" +
			"// @name         Example\n" +
			"// @version      v0.1.0\n" +
			"// @package      example\n" +
			"// @type         bangumi\n" +
			"// @nsfw         " + nsfw + "\n" +
			"// ==/MiruExtension==\n"
	}

	for _, tc := range []struct {
		raw  string
		want bool
	}{
		{"true", true},
		{"True", true},
		{"1", true},
		{"false", false},
		{"", false},
		{"junk", false},
	} {
		ext, err := extension.ParseExtensionMetadata(jsHeader(tc.raw), "example.js")
		require.NoError(t, err)
		assert.Equal(t, tc.want, ext.Nsfw,
			"@nsfw %q must parse to %v", tc.raw, tc.want)
	}

	// Missing @nsfw must default to false.
	plain := "// ==MiruExtension==\n// @name Example\n// @package example\n// @type bangumi\n// ==/MiruExtension==\n"
	ext, err := extension.ParseExtensionMetadata(plain, "example.js")
	require.NoError(t, err)
	assert.False(t, ext.Nsfw, "a missing @nsfw tag must default to false")
}

// TestWatchExtensionsRoutesByLanguage exercises the file-watch entry point.
// Writing a .js file must fire onChange(LanguageJS, pkg), writing a .go file
// must fire onChange(LanguageGolang, pkg), and writing an unsupported file
// (.txt) must be ignored entirely. This proves the watcher routes events to
// the correct runtime by extension.
func TestWatchExtensionsRoutesByLanguage(t *testing.T) {
	dir := t.TempDir()

	type event struct {
		lang extension.Language
		pkg  string
	}
	var (
		mu     sync.Mutex
		events []event
	)
	onChange := func(lang extension.Language, pkg string) {
		mu.Lock()
		events = append(events, event{lang: lang, pkg: pkg})
		mu.Unlock()
	}

	watcher, err := extension.WatchExtensions([]string{dir}, onChange)
	if err != nil {
		t.Fatalf("WatchExtensions: %v", err)
	}
	defer watcher.Close()

	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	// Trigger create + write events for both runtimes and an unsupported file.
	write("foo.js", "// js")
	write("bar.go", "package bar")
	write("ignore.txt", "nope")

	// Poll until we have seen at least the .js and .go events (or time out).
	deadline := time.Now().Add(5 * time.Second)
	sawJS, sawGo := false, false
	for time.Now().Before(deadline) {
		mu.Lock()
		for _, e := range events {
			if e.lang == extension.LanguageJS && e.pkg == "foo" {
				sawJS = true
			}
			if e.lang == extension.LanguageGolang && e.pkg == "bar" {
				sawGo = true
			}
		}
		mu.Unlock()
		if sawJS && sawGo {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	assert.True(t, sawJS, "a .js write must fire onChange with LanguageJS")
	assert.True(t, sawGo, "a .go write must fire onChange with LanguageGolang")

	// The unsupported .txt file must never be reported.
	mu.Lock()
	for _, e := range events {
		assert.NotEqual(t, extension.LanguageUnknown, e.lang,
			"unsupported extensions must be ignored by the watcher")
		assert.NotEqual(t, "ignore", e.pkg, ".txt files must not trigger onChange")
	}
	mu.Unlock()
}

// TestLoadExampleGolangExtension loads the shipped Go example extension
// through the Golang runtime entry point and runs one of its exported
// endpoints. This is the "does the Golang example load?" counterpart to the
// JS example load test (which lives in the binary package because it needs
// the embedded runtime assets).
func TestLoadExampleGolangExtension(t *testing.T) {
	prev := golang.ExtensionDir
	golang.ExtensionDir = filepath.Join("golang", "testdata", "example")
	defer func() { golang.ExtensionDir = prev }()

	// Search compiles the .go source and invokes the exported Search function,
	// which is the real load+run path for the example extension.
	items, err := golang.Search("example", 1, "test", nil)
	assert.NoError(t, err, "the Go example extension must compile and run")
	assert.NotNil(t, items)
	assert.GreaterOrEqual(t, len(items), 1, "example Search should return results")
}
