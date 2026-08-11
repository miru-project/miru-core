package golang

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setExampleExtensionDir points ExtensionDir at the synthetic example
// extension fixture in testdata. Per-site extensions and their tests live in
// their own repositories; this fixture exists only to exercise the runtime.
func setExampleExtensionDir() {
	ExtensionDir = filepath.Join("testdata", "example")
}

// writeExtensionSource writes a Go extension source file into a fresh temp dir
// and points ExtensionDir at it.
func writeExtensionSource(t *testing.T, pkg, src string) {
	t.Helper()
	dir := t.TempDir()
	extPath := filepath.Join(dir, pkg+".go")
	if err := os.WriteFile(extPath, []byte(src), 0644); err != nil {
		t.Fatalf("write extension source: %v", err)
	}
	ExtensionDir = dir
}

// captureStdout runs fn with os.Stdout redirected to a pipe and returns what
// was written to stdout.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = oldStdout

	var out strings.Builder
	data, _ := io.ReadAll(r)
	out.WriteString(string(data))
	r.Close()
	return out.String()
}
