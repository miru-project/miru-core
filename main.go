package main

import (
	"flag"
	"fmt"

	"github.com/miru-project/miru-core/binary"
)

import "C"

func main() {

	configPath := flag.String("config", "config.json", "Path to configuration file")
	flag.Parse()
	binary.InitProgram(configPath)

}

//export initDyLib
func initDyLib(configPath *C.char) *C.char {
	var result *C.char
	defer func() {
		if r := recover(); r != nil {
			result = C.CString("Error: " + fmt.Sprint(r))
		}
	}()
	if result != nil {
		return result
	}
	configPathStr := C.GoString(configPath)
	binary.InitProgram(&configPathStr)

	return result
}
