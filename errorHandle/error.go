package errorhandle

import (
	"log"
	"runtime/debug"
)

func HandleFatal[T any](out T, err error) T {
	if err != nil {
		PrintStack(err)
	}
	return out
}

func PrintStack(err error) {
	debug.PrintStack()
	stackTrace := string(debug.Stack())
	log.Println("Stack trace:", stackTrace)
	log.Fatal("Fatal: ", err)
}
func HandleError[T any](out T, err error) T {
	if err != nil {
		debug.PrintStack()
		stackTrace := string(debug.Stack())
		log.Println("Stack trace:", stackTrace)
		log.Println("Error: ", err)
		return out
	}
	return out
}
