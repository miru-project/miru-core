package js

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/miru-project/miru-core/pkg/network"
)

// repoFixture serves the layout a real extension repo publishes: the index at
// /index.json, sources under /repo/<language>/<package>.<ext>. Every requested
// path is recorded so a test can assert which file the downloader asked for.
// Anything not in files answers 404, exactly like raw.githubusercontent.
type repoFixture struct {
	*httptest.Server
	mu    sync.Mutex
	paths []string
}

func newRepoFixture(t *testing.T, files map[string]string) *repoFixture {
	t.Helper()
	f := &repoFixture{}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.paths = append(f.paths, r.URL.Path)
		f.mu.Unlock()
		body, ok := files[r.URL.Path]
		if !ok {
			http.Error(w, "404: Not Found", http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(f.Close)
	return f
}

func (f *repoFixture) requested() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.paths...)
}

// useRepo points the package-level state at the fixture: the already-fetched
// index and a scratch extension directory.
func (f *repoFixture) useRepo(t *testing.T, entries []GithubExtension) string {
	t.Helper()
	network.Init()
	prevRepo, prevPath := fetchedExtensionRepo, ExtPath
	repoUrl := f.URL + "/index.json"
	fetchedExtensionRepo = map[string][]GithubExtension{repoUrl: entries}
	ExtPath = t.TempDir()
	t.Cleanup(func() {
		fetchedExtensionRepo, ExtPath = prevRepo, prevPath
	})
	return repoUrl
}

const (
	golangSource = "package main\n\n// @name RoyalRoad\n// @package royalroad.com\n"
	jsSource     = "// ==MiruExtension==\n// @name 345movie\n// @package 345movie.net\n"
)

// A golang extension lives at repo/golang/<pkg>.go, so the downloader must ask
// for the path the index published and save it with the .go suffix the golang
// runtime scans for (golang.LoadExtensions stats <ExtensionDir>/<pkg>.go).
// Asking for a .js URL 404s, and the error page body was written to disk as if
// it were the extension.
func TestDownloadExtension_GolangExtensionIsNotFetchedAsJs(t *testing.T) {
	fixture := newRepoFixture(t, map[string]string{
		"/repo/golang/royalroad.com.go": golangSource,
		"/repo/js/345movie.net.js":      jsSource,
	})
	repoUrl := fixture.useRepo(t, []GithubExtension{
		{Name: "RoyalRoad", Package: "royalroad.com", URL: "golang/royalroad.com.go"},
		{Name: "345movie", Package: "345movie.net", URL: "js/345movie.net.js"},
	})

	if err := DownloadExtension(repoUrl, "royalroad.com"); err != nil {
		t.Fatalf("download failed: %v (requested %v)", err, fixture.requested())
	}

	if err := DownloadExtension(repoUrl, "345movie.net"); err != nil {
		t.Fatalf("download failed: %v (requested %v)", err, fixture.requested())
	}

	want := []string{"/repo/golang/royalroad.com.go", "/repo/js/345movie.net.js"}
	if got := fixture.requested(); len(got) != len(want) {
		t.Fatalf("requested %v, want %v", got, want)
	} else {
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("requested %v, want %v", got, want)
			}
		}
	}

	for name, source := range map[string]string{
		"royalroad.com.go": golangSource,
		"345movie.net.js":  jsSource,
	} {
		got, err := os.ReadFile(filepath.Join(ExtPath, name))
		if err != nil {
			t.Fatalf("expected %s in the extension dir, dir=%v", name, entries(ExtPath))
		}
		if string(got) != source {
			t.Fatalf("%s content = %q, want %q", name, got, source)
		}
	}
}

// The index is the source of truth for where a file lives, so the url field has
// to survive unmarshalling.
func TestGithubExtension_KeepsSourceURL(t *testing.T) {
	var index []GithubExtension
	if err := json.Unmarshal([]byte(`[
		{"name":"RoyalRoad","package":"royalroad.com","type":"fikushon","lang":"en","url":"golang/royalroad.com.go"},
		{"name":"345movie","package":"345movie.net","type":"bangumi","lang":"all","url":"js/345movie.net.js"}
	]`), &index); err != nil {
		t.Fatal(err)
	}
	if index[0].URL != "golang/royalroad.com.go" {
		t.Fatalf("url = %q, want golang/royalroad.com.go", index[0].URL)
	}
	if index[1].URL != "js/345movie.net.js" {
		t.Fatalf("url = %q, want js/345movie.net.js", index[1].URL)
	}
}

// Repos predating the url field must still install, via the js layout.
func TestDownloadExtension_FallsBackToJsLayout(t *testing.T) {
	fixture := newRepoFixture(t, map[string]string{"/repo/js/345movie.net.js": jsSource})
	repoUrl := fixture.useRepo(t, []GithubExtension{{Package: "345movie.net"}})

	if err := DownloadExtension(repoUrl, "345movie.net"); err != nil {
		t.Fatalf("download failed: %v (requested %v)", err, fixture.requested())
	}
	if got := fixture.requested(); len(got) != 1 || got[0] != "/repo/js/345movie.net.js" {
		t.Fatalf("requested %v, want [/repo/js/345movie.net.js]", got)
	}
}

// An HTTP error must never be saved as an extension file: network.Request
// resolves 4xx/5xx bodies without an error, so the status has to be checked.
func TestDownloadExtension_RejectsErrorResponse(t *testing.T) {
	fixture := newRepoFixture(t, map[string]string{})
	repoUrl := fixture.useRepo(t, []GithubExtension{
		{Package: "royalroad.com", URL: "golang/royalroad.com.go"},
	})

	if err := DownloadExtension(repoUrl, "royalroad.com"); err == nil {
		t.Fatal("expected a 404 response to fail the download")
	}

	if got := entries(ExtPath); len(got) != 0 {
		t.Fatalf("nothing may be written when the download fails, found %v", got)
	}
}

// The caller has to name the repo the entry came from: an empty repoUrl is a
// distinct failure (the caller dropped the field) and must not be reported as
// a network problem.
func TestDownloadExtension_EmptyRepoURLIsRejected(t *testing.T) {
	fixture := newRepoFixture(t, map[string]string{"/repo/js/345movie.net.js": jsSource})
	fixture.useRepo(t, []GithubExtension{{Package: "345movie.net", URL: "js/345movie.net.js"}})

	err := DownloadExtension("", "345movie.net")
	if err == nil {
		t.Fatal("expected an empty repo url to fail")
	}
	if got := fixture.requested(); len(got) != 0 {
		t.Fatalf("nothing may be requested without a repo url, got %v", got)
	}
}

// Uninstall must find the golang file, not just <pkg>.js.
func TestRemoveExtension_RemovesGolangSource(t *testing.T) {
	prevPath := ExtPath
	ExtPath = t.TempDir()
	t.Cleanup(func() { ExtPath = prevPath })

	if err := os.WriteFile(filepath.Join(ExtPath, "royalroad.com.go"), []byte(golangSource), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := RemoveExtension("royalroad.com"); err != nil {
		t.Fatalf("remove failed: %v", err)
	}
	if got := entries(ExtPath); len(got) != 0 {
		t.Fatalf("expected the file to be gone, found %v", got)
	}
}

func TestRemoveExtension_StillRemovesJsSource(t *testing.T) {
	prevPath := ExtPath
	ExtPath = t.TempDir()
	t.Cleanup(func() { ExtPath = prevPath })

	if err := os.WriteFile(filepath.Join(ExtPath, "345movie.net.js"), []byte(jsSource), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := RemoveExtension("345movie.net"); err != nil {
		t.Fatalf("remove failed: %v", err)
	}
	if got := entries(ExtPath); len(got) != 0 {
		t.Fatalf("expected the file to be gone, found %v", got)
	}
}

func entries(dir string) []string {
	list, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(list))
	for _, e := range list {
		names = append(names, e.Name())
	}
	return names
}
