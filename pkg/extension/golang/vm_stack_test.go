package golang

import (
	"strings"
	"testing"
)

// TestScriggoBuildErrorHasStackTrace verifies the explicit requirement that a
// Scriggo VM Go error (here a compile failure) is returned with a goroutine
// stack trace embedded, so callers can see where in the Go runtime it
// originated rather than receiving a bare string.
func TestScriggoBuildErrorHasStackTrace(t *testing.T) {
	vm := NewScriggoVM(nil)
	_, err := vm.Compile("bad", "package main\nfunc ( { // syntax error\n")
	if err == nil {
		t.Fatalf("expected a compile error")
	}
	if !strings.Contains(err.Error(), "goroutine") {
		t.Fatalf("expected embedded stack trace in error, got: %s", err.Error())
	}
	t.Logf("stacktrace-enriched error OK: %.120s...", err.Error())
}
