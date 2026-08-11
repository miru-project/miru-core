package golang

import (
	"testing"

	"github.com/miru-project/miru-core/pkg/extension"
	"github.com/miru-project/miru-core/proto/generate/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExampleExtensionCompiles(t *testing.T) {
	setExampleExtensionDir()
	_, err := Search("example", 1, "test", nil)
	if err != nil {
		t.Fatalf("failed to compile/run extension example: %v", err)
	}
}

// TestRuntimeLoadExtensionAndCall verifies the full flow: an extension whose
// entry point is the Load function is loaded into the runtime (the Load entry
// point is run once to extend the runtime) and a specific function exported by
// the extension is later invoked by name with Runtime.Call, using the local
// Scriggo build that supports calling a function by name.
func TestRuntimeLoadExtensionAndCall(t *testing.T) {
	// Write an extension that declares a Load entry point and a couple of
	// callable functions. The package must not be named "main" so that Scriggo
	// accepts Load as the entry point (a "main" package requires a func main).
	src := `package myext

func Load() {
	// Entry point: initialize the extension.
}

func Add(a, b int) int {
	return a + b
}

func Greet(name string) string {
	return "Hello, " + name
}
`
	writeExtensionSource(t, "myext", src)

	rt := NewRuntime(NewScriggoVM(nil))
	err := rt.LoadExtension(&extension.Extension{Name: "myext", Pkg: "myext"})
	assert.NoError(t, err)

	// Later, call a specific function exported by the extension.
	res, err := rt.Call("Add", 2, 3)
	assert.NoError(t, err)
	assert.Equal(t, 5, res.(int))

	res, err = rt.Call("Greet", "World")
	assert.NoError(t, err)
	assert.Equal(t, "Hello, World", res.(string))
}

// TestRuntimeLoadEntryPointRequired verifies that LoadExtension runs the
// extension's "Load" entry point AND that a missing Load entry point is a hard
// compile/load error surfaced at the Load symbol -- not a silent success.
//
// Per the load contract, the load error must point at the same source location
// as the compiled error: when "Load" is not declared, the runtime reports it as
// an undefined entry point at "Load". The shipped example extension deliberately
// omits Load (it opts into lazy loading), so this test pins the expected failure
// so a regression that silently swallows a missing load hook is caught.
func TestRuntimeLoadEntryPointRequired(t *testing.T) {
	setExampleExtensionDir()

	rt := NewRuntime(NewScriggoVM(nil))
	err := rt.LoadExtension(&extension.Extension{Name: "example", Pkg: "example"})
	require.Error(t, err, "an extension without a Load entry point must fail to load")
	// The failure must be localized to the Load entry point, not a bare generic
	// message -- mirroring how a compiled error names its location.
	assert.Contains(t, err.Error(), "Load", "load error must name the Load entry point where it failed")
}

// TestRuntimeCallBeforeLoad verifies that calling a function before an
// extension has been loaded returns an error.
func TestRuntimeCallBeforeLoad(t *testing.T) {
	rt := NewRuntime(NewScriggoVM(nil))
	_, err := rt.Call("Add", 1, 2)
	assert.Error(t, err)
}

// TestRuntimeCallUnknownFunction verifies that calling a function that does
// not exist in the loaded extension returns an error.
func TestRuntimeCallUnknownFunction(t *testing.T) {
	src := `package myext

func Load() {}

func Add(a, b int) int {
	return a + b
}
`
	writeExtensionSource(t, "myext", src)

	rt := NewRuntime(NewScriggoVM(nil))
	if err := rt.LoadExtension(&extension.Extension{Name: "myext", Pkg: "myext"}); err != nil {
		t.Fatalf("LoadExtension failed: %v", err)
	}

	_, err := rt.Call("DoesNotExist", 1, 2)
	assert.Error(t, err)
}

func TestExampleExtensionSearchOutput(t *testing.T) {
	setExampleExtensionDir()
	items, err := Search("example", 1, "test", nil)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("expected 2 search results, got %d", len(items))
	}
}

func TestExampleExtensionLatestOutput(t *testing.T) {
	setExampleExtensionDir()
	items, err := Latest("example", 1)
	if err != nil {
		t.Fatalf("Latest failed: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("expected 1 latest result, got %d", len(items))
	}
}

func TestExampleExtensionDetailOutput(t *testing.T) {
	setExampleExtensionDir()
	detail, err := Detail("example", "https://example.com/1")
	if err != nil {
		t.Fatalf("Detail failed: %v", err)
	}
	if detail == nil || *detail.Title != "Example Detail" {
		t.Errorf("expected Example Detail, got %v", detail)
	}
}

func TestExampleExtensionWatchOutput(t *testing.T) {
	setExampleExtensionDir()
	watch, _, err := Watch("example", "https://example.com/1")
	if err != nil {
		t.Fatalf("Watch failed: %v", err)
	}
	if watch == nil {
		t.Errorf("expected watch result, got nil")
	}
}

func TestExampleExtensionMirrorOutput(t *testing.T) {
	setExampleExtensionDir()
	res, err := Mirror("example", "https://example.com/1")
	if err != nil {
		t.Fatalf("Mirror failed: %v", err)
	}
	if res == nil {
		t.Errorf("expected mirror result, got nil")
	}
	all, ok := res.(*proto.ExtensionAllWatch)
	if !ok {
		t.Fatalf("expected *proto.ExtensionAllWatch, got %T", res)
	}
	if all.Bangumi == nil || all.Bangumi.Url != "https://example.com/1" {
		t.Errorf("expected bangumi mirror url to be set, got %+v", all.Bangumi)
	}
}
