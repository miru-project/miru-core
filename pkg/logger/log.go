package logger

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
)

var mirulogger *log.Logger

// crashFile receives dedicated, identifiable crash entries so the Flutter
// client can export miru_core crashes without scraping the runtime log.
var crashFile *os.File

func InitLog(logFolder string) {
	outLogFile, e := os.Create(filepath.Join(logFolder, "miru_core.log"))
	if e != nil {
		panic(fmt.Sprintf("Failed to create log file: %v", e))
	}
	mirulogger = log.New(outLogFile, "[Miru Core] ", log.LstdFlags)

	// Crash log accumulates across runs (append) so no crash is lost on
	// restart; fall back to the main log file if it cannot be opened.
	cf, ce := os.OpenFile(
		filepath.Join(logFolder, "miru_core_crash.log"),
		os.O_CREATE|os.O_APPEND|os.O_WRONLY,
		0o644,
	)
	if ce != nil {
		crashFile = outLogFile
	} else {
		crashFile = cf
	}
}

// LogCrash writes a single identified crash entry to the dedicated crash log
// and mirrors it to stderr so it is never silently lost. If InitLog has not
// run yet (or failed to open the files), the entry is appended to
// miru_core_crash.log in the process working directory so very early startup
// crashes are still persisted instead of reaching stderr only.
func LogCrash(s string) {
	ensureCrashFile()
	if crashFile != nil {
		fmt.Fprintln(crashFile, s)
	}
	log.Println(s)
}

// ensureCrashFile lazily opens a fallback crash log exactly once. InitLog
// normally provides crashFile; this covers panics raised before InitLog ran,
// where losing the entry entirely would defeat the crash-reporting pipeline.
var ensureCrashFileOnce sync.Once

func ensureCrashFile() {
	ensureCrashFileOnce.Do(func() {
		if crashFile != nil {
			return
		}
		cf, ce := os.OpenFile(
			"miru_core_crash.log",
			os.O_CREATE|os.O_APPEND|os.O_WRONLY,
			0o644,
		)
		if ce == nil {
			crashFile = cf
		}
	})
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
