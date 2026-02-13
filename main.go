package main

import (
	"flag"

	"github.com/miru-project/miru-core/binary"
)

import "C"

func main() {

	configPath := flag.String("config", "config.json", "Path to configuration file")
	flag.Parse()
	binary.InitProgram(configPath)

}

//export initDyLib
func initDyLib(configPath *C.char) {
	configPathStr := C.GoString(configPath)
	go binary.InitProgram(&configPathStr)

}
