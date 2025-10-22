package errorhandle

import (
	"fmt"
	"runtime/debug"

	"github.com/miru-project/miru-core/pkg/logger"
)

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
