package errorhandle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/miru-project/miru-core/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// panicOrigin is a named frame so tests can assert the captured stack trace
// actually points at the panic site rather than just carrying the label.
func panicOrigin() {
	panic("boom-goroutine")
}

// TestRecoverLog_GoroutinePanicIsCaptured verifies that a panic inside a
// goroutine guarded by RecoverLog is swallowed (the process keeps running)
// and written to the dedicated crash log as an identifiable [CRASH][miru_core]
// entry carrying the panic value, the supplied context, and a full stack
// trace that names the panic site.
func TestRecoverLog_GoroutinePanicIsCaptured(t *testing.T) {
	dir := t.TempDir()
	logger.InitLog(dir)

	done := make(chan struct{})
	go func() {
		// RecoverLog must be deferred AFTER close so the crash entry is fully
		// written before the channel signals completion.
		defer close(done)
		defer RecoverLog("grpc.StartServer")
		panicOrigin()
	}()
	<-done // only reached if the panic was recovered, not propagated

	raw, err := os.ReadFile(filepath.Join(dir, "miru_core_crash.log"))
	require.NoError(t, err, "crash log should be written")

	got := string(raw)
	assert.Contains(t, got, "[CRASH][miru_core]", "missing crash marker")
	assert.Contains(t, got, "boom-goroutine", "missing panic value")
	assert.Contains(t, got, "grpc.StartServer", "missing context")
	assert.Contains(t, got, "stack:", "missing stack trace header")
	assert.Contains(t, got, "panicOrigin",
		"stack trace must name the panic site")
	assert.Regexp(t, `(?m)^goroutine \d+ \[running\]:`, got,
		"stack trace must contain a goroutine dump header")
}

// TestRecoverLog_NoPanicStaysSilent verifies that a goroutine that does not
// panic leaves no crash entry, so healthy goroutines are not misreported as
// crashes.
func TestRecoverLog_NoPanicStaysSilent(t *testing.T) {
	dir := t.TempDir()
	logger.InitLog(dir)

	done := make(chan struct{})
	go func() {
		defer close(done)
		defer RecoverLog("healthy")
		// intentionally no panic
	}()
	<-done

	raw, err := os.ReadFile(filepath.Join(dir, "miru_core_crash.log"))
	require.NoError(t, err)
	assert.Empty(t, strings.TrimSpace(string(raw)),
		"no crash entry expected when nothing panicked")
}

// TestLogCrash_WritesIdentifiableEntry verifies the low-level crash writer
// emits the marker, timestamp, context, panic value and stack in one entry.
func TestLogCrash_WritesIdentifiableEntry(t *testing.T) {
	dir := t.TempDir()
	logger.InitLog(dir)

	LogCrash("kaboom", "InitProgram")

	raw, err := os.ReadFile(filepath.Join(dir, "miru_core_crash.log"))
	require.NoError(t, err)

	got := string(raw)
	assert.True(t, strings.HasPrefix(got, "[CRASH][miru_core]"),
		"entry must start with the crash marker")
	assert.Contains(t, got, "context: [InitProgram]")
	assert.Contains(t, got, "panic: kaboom")
	assert.Contains(t, got, "stack:", "missing stack trace header")
	assert.Contains(t, got, "LogCrash",
		"stack trace must include the capture frame")
}
