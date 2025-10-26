package logger

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)

var mirulogger *log.Logger

func InitLog(logFolder string) {
	outLogFile, e := os.Create(filepath.Join(logFolder, "miru_core.log"))
	if e != nil {
		panic(fmt.Sprintf("Failed to create log file: %v", e))
	}
	mirulogger = log.New(outLogFile, "[Miru Core] ", log.LstdFlags)
}

// Export logging functions
func Printf(format string, v ...interface{}) {
	if mirulogger != nil {
		mirulogger.Printf(format, v...)
	}
	log.Printf(format, v...)
}

func Println(v ...interface{}) {
	if mirulogger != nil {
		mirulogger.Println(v...)
	}
	log.Println(v...)

}

func Fatalf(format string, v ...interface{}) {
	if mirulogger != nil {
		mirulogger.Fatalf(format, v...)
	}
	log.Fatalf(format, v...)
}

func Fatalln(v ...interface{}) {
	if mirulogger != nil {
		mirulogger.Fatalln(v...)
	}
	log.Fatalln(v...)
}
func Fatal(v ...interface{}) {
	if mirulogger != nil {
		mirulogger.Fatal(v...)
	}
	log.Fatal(v...)
}

func Print(v ...interface{}) {
	if mirulogger != nil {
		mirulogger.Print(v...)
	}
	log.Print(v...)
}
