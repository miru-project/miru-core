package errorhandle

import (
	"fmt"
	"runtime/debug"
	"time"

	"github.com/miru-project/miru-core/pkg/logger"
)

// crashMarker is written at the start of every captured crash entry so the
// Flutter client can identify and export miru_core crashes unambiguously.
const crashMarker = "[CRASH][miru_core]"

// LogCrash records a recovered panic as an identifiable crash entry, including
// a timestamp, an optional context (e.g. the failing request), the panic value
// and a full stack trace. It is safe to call from any goroutine guard.
func LogCrash(recovered any, context ...string) {
	stack := string(debug.Stack())
	logger.LogCrash(fmt.Sprintf(
		"%s %s\ncontext: %v\npanic: %v\nstack:\n%s\n----------------------------------------",
		crashMarker,
		time.Now().Format(time.RFC3339),
		context,
		recovered,
		stack,
	))
}

// RecoverLog logs a captured panic from a goroutine guard and swallows it so
// the surrounding server keeps running.
func RecoverLog(context ...string) {
	if r := recover(); r != nil {
		LogCrash(r, context...)
	}
}

func HandleFatal[T any](out T, err error) T {
	if err != nil {
		PrintStack(err)
	}
	return out
}

func PanicF(format string, a ...any) {
	logger.Printf(format, a...)
	panic(fmt.Sprintf(format, a...))
}

func PrintStack(err error) {
	debug.PrintStack()
	stackTrace := string(debug.Stack())
	logger.Println("Stack trace:", stackTrace)
	panic(err)
}
func HandleError[T any](out T, err error) T {
	if err != nil {
		debug.PrintStack()
		stackTrace := string(debug.Stack())
		logger.Println("Stack trace:", stackTrace)
		logger.Println("Error: ", err)
		return out
	}
	return out
}
