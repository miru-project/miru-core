package golang

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	example "github.com/miru-project/miru-core/pkg/extension/golang/extensions/example"
	"github.com/stretchr/testify/assert"
)

// TestDualStage exercises a single Miru Go extension in the two ways it can be
// driven, and prints the JSON result of each call so the output reads like a
// runnable test script:
//
//	Stage 1 (Golang / native): the extension is imported as a normal Go
//	                        library and its exported entry points are called as
//	                        ordinary Go functions -- exactly what an author does
//	                        in a fresh `go mod init` project to get full gopls /
//	                        `go test` support while developing.
//
//	Stage 2 (Scriggo VM):      the SAME example.go is compiled + run by the
//	                        Scriggo VM (the same path miru-core uses at runtime),
//	                        invoking entry points by name. The extension imports
//	                        the SDK explicitly (`import sdk "..."`), so the host
//	                        compiles the source as-is with no rewriting.
//
// Both stages call Latest / Search / Mirror and print the results, so an author
// can confirm their extension produces the expected output in either style.
//
// Run with:
//
//	go test -run TestDualStage -v ./pkg/extension/golang/...
func TestDualStage(t *testing.T) {
	const pkg = "example"
	const url = "https://example.com/1"

	// ----------------------------------------------------------------------
	// Stage 1 -- Golang / native: import the extension package directly and call
	// its exported entry points as ordinary Go functions.
	// ----------------------------------------------------------------------
	t.Log("=== Stage 1: Golang / native (direct package import) ===")

	latest, err := example.Latest(pkg, 1)
	assert.NoError(t, err)
	printStage(t, "Latest", latest)

	search, err := example.Search(pkg, "test", 1, "")
	assert.NoError(t, err)
	printStage(t, "Search", search)

	mirrors, err := example.Mirror(pkg, url)
	assert.NoError(t, err)
	printStage(t, "Mirror", mirrors)

	detail, err := example.Detail(pkg, url)
	assert.NoError(t, err)
	printStage(t, "Detail", detail)

	watch, err := example.Watch(pkg, url)
	assert.NoError(t, err)
	printStage(t, "Watch", watch)

	// ----------------------------------------------------------------------
	// Stage 2 -- Scriggo VM: example.go (which imports the SDK explicitly) is
	// read from disk and compiled + run by the Scriggo VM, then the same entry
	// points are invoked by name.
	// ----------------------------------------------------------------------
	t.Log("=== Stage 2: Scriggo VM (example.go, call by name) ===")

	ExtensionDir = filepath.Join("extensions", "example")

	latestVM, err := golangCall(t, pkg, "Latest", pkg, 1)
	assert.NoError(t, err)
	printStage(t, "Latest", latestVM)

	searchVM, err := golangCall(t, pkg, "Search", pkg, "test", 1, "")
	assert.NoError(t, err)
	printStage(t, "Search", searchVM)

	mirrorsVM, err := golangCall(t, pkg, "Mirror", pkg, url)
	assert.NoError(t, err)
	printStage(t, "Mirror", mirrorsVM)

	detailVM, err := golangCall(t, pkg, "Detail", pkg, url)
	assert.NoError(t, err)
	printStage(t, "Detail", detailVM)

	watchVM, err := golangCall(t, pkg, "Watch", pkg, url)
	assert.NoError(t, err)
	printStage(t, "Watch", watchVM)
}

// golangCall compiles the extension source at ExtensionDir/pkg.go with the
// Scriggo VM and invokes fn by name (mirroring endpoint.callExtension). The
// extension is expected to import the SDK explicitly; the host compiles it
// as-is. It returns the raw result value.
func golangCall(t *testing.T, pkg, fn string, args ...any) (any, error) {
	t.Helper()
	src, err := os.ReadFile(filepath.Join(ExtensionDir, pkg+".go"))
	if err != nil {
		return nil, err
	}
	vm := NewScriggoVM(nil)
	prog, err := vm.Compile(pkg+"_"+fn, string(src))
	if err != nil {
		return nil, fmt.Errorf("compile extension %s: %w", pkg, err)
	}
	res, err := prog.program.Call(fn, args...)
	if err != nil {
		return nil, withStackTrace("extension "+pkg+"."+fn, err)
	}
	if len(res) == 0 {
		return nil, nil
	}
	return res[0], nil
}

// printStage marshals a stage result to indented JSON and logs it, so the test
// output doubles as a readable print-out of what each call returned.
func printStage(t *testing.T, name string, v any) {
	t.Helper()
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Logf("[%s] <marshal error: %v>", name, err)
		return
	}
	t.Logf("[%s]\n%s", name, string(b))
}
