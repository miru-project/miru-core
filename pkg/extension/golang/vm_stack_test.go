package golang

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestScriggoBuildErrorHasStackTrace verifies the explicit requirement that a
// Scriggo VM Go error (here a COMPILE failure) is returned with a goroutine
// stack trace embedded, so callers can see where in the Go runtime it
// originated rather than receiving a bare string.
//
// Note: this is a COMPILE error, not a runtime panic. No extension code is ever
// executed, so there is no interpreter call stack to capture; the host
// goroutine stack (debug.Stack) is the only stack that exists here and is the
// correct trace. The Scriggo interpreter call stack (the extension's own inner
// function chain) is produced only by RUNTIME panics and is demonstrated by
// TestGolangExtensionPanicShowsInnerCallChain.
func TestScriggoBuildErrorHasStackTrace(t *testing.T) {
	vm := NewScriggoVM(nil)
	_, err := vm.Compile("", "bad", "package main\nfunc ( { // syntax error\n")
	if err == nil {
		t.Fatalf("expected a compile error")
	}
	if !strings.Contains(err.Error(), "goroutine") {
		t.Fatalf("expected embedded stack trace in error, got: %s", err.Error())
	}
	t.Logf("compile-error trace (host goroutine stack; correct for a COMPILE error - no interpreter stack exists):\n%s", err.Error())
}

// stackTraceSrc is a Go/Scriggo extension whose exported entry point (Latest)
// calls a private helper (middle), which calls another private helper
// (innerBoom) that panics. This gives us a multi-level inner call chain inside
// the extension:
//
//	Latest -> middle -> innerBoom -> panic("boom in innerBoom")
//
// The Load hook is required by the Go runtime; here it just seeds the
// cross-function cache via sdk.SaveCache (mirroring the JS Miru.saveCache
// contract) so the extension compiles and loads.
const stackTraceSrc = `
// ==MiruExtension==
// @name Stack Trace Test
// @package stacktraceext
// @apiVersion 2
package stacktraceext

import (
	sdk "github.com/miru-project/miru-core/pkg/extension/golang/sdk"
)

func innerBoom() string {
	panic("boom in innerBoom")
}

func middle() string {
	return innerBoom()
}

func Load() {
	sdk.SaveCache("stacktraceext", "k", "v")
}

func Latest(pkg string, page int) ([]sdk.ExtensionListItem, error) {
	_ = middle()
	return nil, nil
}
`

// TestGolangExtensionPanicShowsInnerCallChain proves the requirement that when a
// Go extension panics, the returned error's stack trace enumerates the
// extension's OWN inner function call chain (Latest -> middle -> innerBoom),
// instead of only showing the surrounding host goroutine frames. The test runs
// the real endpoint (Latest) so it exercises the full path: runtime.Call ->
// scriggo Program.Call -> interpreter -> panic, and back out as a
// *scriggo.PanicError carrying the captured interpreter stack.
func TestGolangExtensionPanicShowsInnerCallChain(t *testing.T) {
	writeExtensionSource(t, "stacktraceext", stackTraceSrc)
	LoadExtensions() // runs Load once, seeding the cache

	_, err := Latest("stacktraceext", 1)
	require.Error(t, err, "expected the extension panic to surface as an error")

	msg := err.Error()
	t.Logf("extension panic error:\n%s", msg)

	// The panic message must be present.
	assert.Contains(t, msg, "boom in innerBoom")

	// The extension's inner call chain must be visible: the entry point and
	// every private helper that was on the call stack when it panicked.
	for _, fn := range []string{"Latest", "middle", "innerBoom"} {
		assert.Contains(t, msg, fn,
			"extension stack trace should list the inner function %q", fn)
	}

	// The interpreter call stack must be package-qualified so the author can
	// see exactly which call failed.
	assert.Contains(t, msg, "stacktraceext.Latest()",
		"interpreter stack should show the package-qualified entry point")
}
