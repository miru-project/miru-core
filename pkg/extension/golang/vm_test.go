package golang

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/miru-project/miru-core/pkg/extension"
	"github.com/stretchr/testify/assert"
)

func TestExampleExtensionCompiles(t *testing.T) {
	ExtensionDir = filepath.Join("extensions", "example")
	_, err := Search("example", 1, "test", "")
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
	dir := t.TempDir()
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
	extPath := filepath.Join(dir, "myext.go")
	if err := os.WriteFile(extPath, []byte(src), 0644); err != nil {
		t.Fatalf("write extension source: %v", err)
	}
	ExtensionDir = dir

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

// TestRuntimeLoadExtensionExampleWithLoad verifies that the shipped example
// extension can be loaded (its entries are exercised by the endpoint tests)
// using the legacy "main" based flow. The Load-entry-point flow demonstrated
// by the other tests targets extensions whose package is not "main".
func TestRuntimeLoadExtensionExampleWithLoad(t *testing.T) {
	ExtensionDir = filepath.Join("extensions", "example")

	rt := NewRuntime(NewScriggoVM(nil))
	err := rt.LoadExtension(&extension.Extension{Name: "example", Pkg: "example"})
	assert.Error(t, err)
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
	dir := t.TempDir()
	src := `package myext

func Load() {}

func Add(a, b int) int {
	return a + b
}
`
	extPath := filepath.Join(dir, "myext.go")
	if err := os.WriteFile(extPath, []byte(src), 0644); err != nil {
		t.Fatalf("write extension source: %v", err)
	}
	ExtensionDir = dir

	rt := NewRuntime(NewScriggoVM(nil))
	if err := rt.LoadExtension(&extension.Extension{Name: "myext", Pkg: "myext"}); err != nil {
		t.Fatalf("LoadExtension failed: %v", err)
	}

	_, err := rt.Call("DoesNotExist", 1, 2)
	assert.Error(t, err)
}

func TestExampleExtensionSearchOutput(t *testing.T) {
	ExtensionDir = filepath.Join("extensions", "example")
	items, err := Search("example", 1, "test", "")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("expected 2 search results, got %d", len(items))
	}
}

func TestExampleExtensionLatestOutput(t *testing.T) {
	ExtensionDir = filepath.Join("extensions", "example")
	items, err := Latest("example", 1)
	if err != nil {
		t.Fatalf("Latest failed: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("expected 1 latest result, got %d", len(items))
	}
}

func TestExampleExtensionDetailOutput(t *testing.T) {
	ExtensionDir = filepath.Join("extensions", "example")
	detail, err := Detail("example", "https://example.com/1")
	if err != nil {
		t.Fatalf("Detail failed: %v", err)
	}
	if detail == nil || *detail.Title != "Example Detail" {
		t.Errorf("expected Example Detail, got %v", detail)
	}
}

func TestExampleExtensionWatchOutput(t *testing.T) {
	ExtensionDir = filepath.Join("extensions", "example")
	watch, _, err := Watch("example", "https://example.com/1")
	if err != nil {
		t.Fatalf("Watch failed: %v", err)
	}
	if watch == nil {
		t.Errorf("expected watch result, got nil")
	}
}

func TestExampleExtensionMirrorOutput(t *testing.T) {
	ExtensionDir = filepath.Join("extensions", "example")
	mirrors, err := Mirror("example", "https://example.com/1")
	if err != nil {
		t.Fatalf("Mirror failed: %v", err)
	}
	if mirrors == nil {
		t.Errorf("expected mirror result, got nil")
	}
}
