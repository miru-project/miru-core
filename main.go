package main

import (
	"flag"

	"github.com/miru-project/miru-core/binary"
)

// #include <stdlib.h>
import "C"

func main() {
	// Parse command line arguments
	configPath := flag.String("config", "config.json", "Path to configuration file")
	flag.Parse()
	binary.InitProgram(configPath)

}
