package example_test

import (
	"encoding/json"
	"path/filepath"
	"runtime"
	"testing"

	golang "github.com/miru-project/miru-core/pkg/extension/golang"
	example "github.com/miru-project/miru-core/pkg/extension/golang/extensions/example"
)

const (
	pkg = "example"
	url = "https://example.com/1"
)

// TestExampleAsScript drives the example extension in the two ways miru-core
// can run it, and prints the result of every entry point so the output reads
// like a runnable "test script" an author uses to confirm the extension is
// wired up correctly:
//
//	go test -run TestExampleAsScript -v ./pkg/extension/golang/extensions/example/...
//
// Stage 1 (Golang / native): the extension is authored the ordinary way -- a
// freshly `go mod init`'d project `go get`s the SDK and imports it directly
// (`import sdk "github.com/miru-project/miru-core/pkg/extension/golang/sdk"`),
// referring to the model types as sdk.ExtensionListItem, sdk.ExtensionDetail,
// and so on. Its entry points are ordinary, directly-callable Go functions.
//
// Stage 2 (Scriggo VM): the SAME example.go is compiled by the host with the
// Scriggo VM and its functions called by name -- exactly what the public
// golang endpoint does at runtime. Because the extension imports the SDK
// explicitly (`import sdk "..."`), the host compiles the source as-is with no
// source rewriting. We drive it through that same endpoint so the test mirrors
// production behaviour.
func TestExampleAsScript(t *testing.T) {
	// Point the golang endpoint at this directory, which holds example.go.
	_, file, _, _ := runtime.Caller(0)
	golang.ExtensionDir = filepath.Dir(file)

	// ------------------------------------------------------------------
	// Stage 1 -- Golang / native: import the SDK-backed package directly.
	// ------------------------------------------------------------------
	t.Log("=== Stage 1: Golang / native (direct package import) ===")

	latest, err := example.Latest(pkg, 1)
	printStage(t, "Latest", latest, err)

	search, err := example.Search(pkg, "test", 1, "")
	printStage(t, "Search", search, err)

	mirrors, err := example.Mirror(pkg, url)
	printStage(t, "Mirror", mirrors, err)

	detail, err := example.Detail(pkg, url)
	printStage(t, "Detail", detail, err)

	watch, err := example.Watch(pkg, url)
	printStage(t, "Watch", watch, err)

	// ------------------------------------------------------------------
	// Stage 2 -- Scriggo VM: compile example.go (which imports the SDK explicitly)
	// and call the entry points by name through the public golang endpoint.
	// ------------------------------------------------------------------
	t.Log("=== Stage 2: Scriggo VM (example.go, call by name) ===")

	// Same entry points, now driven through the public golang endpoint, which
	// compiles example.go with the Scriggo VM and returns the model structs
	// DIRECTLY. No JSON round-trip is needed -- everything is Go, so the
	// structs are passed straight through, exactly as in Stage 1.
	latestVM, err := golang.Latest(pkg, 1)
	printStage(t, "Latest", latestVM, err)

	searchVM, err := golang.Search(pkg, 1, "test", "")
	printStage(t, "Search", searchVM, err)

	mirrorsVM, err := golang.Mirror(pkg, url)
	printStage(t, "Mirror", mirrorsVM, err)

	detailVM, err := golang.Detail(pkg, url)
	printStage(t, "Detail", detailVM, err)

	watchVM, _, err := golang.Watch(pkg, url)
	printStage(t, "Watch", watchVM, err)
}

// printStage marshals a single native-stage entry-point result to indented JSON
// and logs it, so the test output reads like a printed result dump.
func printStage(t *testing.T, name string, v any, err error) {
	t.Helper()
	if err != nil {
		t.Logf("[%s] error: %v", name, err)
		return
	}
	b, mErr := json.MarshalIndent(v, "", "  ")
	if mErr != nil {
		t.Logf("[%s] <marshal error: %v>", name, mErr)
		return
	}
	t.Logf("[%s]\n%s", name, string(b))
}
