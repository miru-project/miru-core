package binary

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	errorhandle "github.com/miru-project/miru-core/pkg/errorHandle"
	log "github.com/miru-project/miru-core/pkg/logger"
)

// triggerStartupPanic simulates the early-startup panic class (a stray
// panic("defer") inside startMemoryMonitor, fired before logging was
// initialized) and recovers it exactly like InitProgram's top-level guard does, returning the recovered value so tests drive
// errorhandle.LogCrash themselves.
func triggerStartupPanic(t *testing.T) any {
	t.Helper()
	var recovered any
	func() {
		defer func() {
			recovered = recover()
		}()
		panic("defer")
	}()
	if recovered == nil {
		t.Fatal("expected simulated panic")
	}
	return recovered
}

// requireCrashEntry asserts the crash log contains an identifiable,
// attributed entry for the startup panic.
func requireCrashEntry(t *testing.T, data []byte) {
	t.Helper()
	for _, want := range []string{
		"[CRASH][miru_core]",
		"context: [InitProgram]",
		"panic: defer",
	} {
		if !strings.Contains(string(data), want) {
			t.Errorf("crash log missing %q, got:\n%s", want, data)
		}
	}
}

// chdir changes the process working directory for the duration of the test,
// restoring it afterwards.
func chdir(t *testing.T, dir string) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
}

// TestEarlyPanicWritesFallbackCrashLog covers the reported bug: a panic
// fired BEFORE log.InitLog ran must still be persisted. InitProgram's
// top-level recover calls errorhandle.LogCrash, which falls back to a
// miru_core_crash.log in the working directory when the configured log
// files are not open yet. Must run before TestPanicAfterInitLogWritesConfiguredCrashLog
// (declaration order below guarantees this) because the logger's fallback
// state is process-global.
func TestEarlyPanicWritesFallbackCrashLog(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	r := triggerStartupPanic(t)
	errorhandle.LogCrash(r, "InitProgram")

	data, err := os.ReadFile(filepath.Join(dir, "miru_core_crash.log"))
	if err != nil {
		t.Fatalf("crash log not written: %v", err)
	}
	requireCrashEntry(t, data)
}

// TestPanicAfterInitLogWritesConfiguredCrashLog verifies the normal
// pipeline: once InitLog ran, crash entries are appended to the dedicated
// crash log next to the config file.
func TestPanicAfterInitLogWritesConfiguredCrashLog(t *testing.T) {
	dir := t.TempDir()
	log.InitLog(dir)

	r := triggerStartupPanic(t)
	errorhandle.LogCrash(r, "InitProgram")

	data, err := os.ReadFile(filepath.Join(dir, "miru_core_crash.log"))
	if err != nil {
		t.Fatalf("crash log not written: %v", err)
	}
	requireCrashEntry(t, data)
}
