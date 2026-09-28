package js

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/miru-project/miru-core/pkg/extension"
)

// waitForCached polls the js API cache because the loader runs in a goroutine
// (loadExtApi dispatches to go LoadApiV1/V2).
func waitForCached(pkg string, timeout time.Duration) *ExtApi {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if v, ok := ApiPkgCache.Map.Load(pkg); ok {
			return v.(*ExtApi)
		}
		time.Sleep(10 * time.Millisecond)
	}
	return nil
}

// An install is more than a file landing on disk: the watcher has to route it to
// the runtime that owns its suffix. This downloads a js and a golang extension
// from one repo, fires the watcher callback for both, and checks that the js
// runtime picked up the .js file and left the .go file to the golang runtime.
func TestInstalledExtensionsAreRoutedToTheirRuntime(t *testing.T) {
	jsSource, err := os.ReadFile(filepath.Join("testdata", "v2watch_example.js"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	fixture := newRepoFixture(t, map[string]string{
		"/repo/js/v2watch.js":           string(jsSource),
		"/repo/golang/royalroad.com.go": golangSource,
	})
	repoUrl := fixture.useRepo(t, []GithubExtension{
		{Name: "V2 Watch/Mirror Example", Package: "v2watch", URL: "js/v2watch.js"},
		{Name: "RoyalRoad", Package: "royalroad.com", URL: "golang/royalroad.com.go"},
	})

	if err := DownloadExtension(repoUrl, "v2watch"); err != nil {
		t.Fatalf("js download failed: %v (requested %v)", err, fixture.requested())
	}
	if err := DownloadExtension(repoUrl, "royalroad.com"); err != nil {
		t.Fatalf("golang download failed: %v (requested %v)", err, fixture.requested())
	}

	// The unified watcher calls the per-language handler and binary.Init routes
	// by the changed file's suffix, so the test does the same. Only the js
	// branch is driven here; golang.HandleReload is covered by the golang
	// package's own install test.
	for _, name := range entries(ExtPath) {
		lang := extension.DetectLanguage(name)
		if lang != extension.LanguageJS {
			continue
		}
		HandleReload(strings.TrimSuffix(name, string(lang)))
	}

	api := waitForCached("v2watch", 5*time.Second)
	if api == nil {
		t.Fatalf("the js runtime never loaded the installed extension, dir=%v", entries(ExtPath))
	}
	if api.Ext.Name != "V2 Watch/Mirror Example" {
		t.Fatalf("loaded name = %q", api.Ext.Name)
	}
	if api.Ext.ApiVersion != "2" {
		t.Fatalf("apiVersion = %q, want 2", api.Ext.ApiVersion)
	}
	// A compiled program is the install signal: AsyncCallBack refuses to run
	// without one, so this is what "the extension works" means. Ext.Error is
	// not asserted because the fixture's load() hook reaches for runtime state
	// that only a fully initialized runtime provides.
	if api.service == nil || api.service.program == nil {
		t.Fatal("installed extension has no compiled program")
	}

	// The golang extension must not be claimed by the js runtime, otherwise a
	// Scriggo source would be evaluated as JavaScript.
	if v, ok := ApiPkgCache.Map.Load("royalroad.com"); ok {
		t.Fatalf("js runtime claimed the golang extension: %v", v)
	}

	// Both are discoverable by the shared scan the gRPC list is built from, each
	// tagged with the runtime that owns it.
	exts, invalid := extension.FilterExtensions(ExtPath)
	if len(invalid) != 0 {
		t.Fatalf("installed files failed to parse: %v", invalid)
	}
	byPkg := map[string]*extension.Extension{}
	for _, e := range exts {
		byPkg[e.Pkg] = e
	}
	if len(byPkg) != 2 {
		t.Fatalf("expected both installed extensions, got %v", byPkg)
	}
	if byPkg["v2watch"].FileLang != extension.LanguageJS {
		t.Errorf("v2watch routed to %q", byPkg["v2watch"].FileLang)
	}
	if byPkg["royalroad.com"].FileLang != extension.LanguageGolang {
		t.Errorf("royalroad.com routed to %q", byPkg["royalroad.com"].FileLang)
	}

	ApiPkgCache.Delete("v2watch")
}
