package jsExtension

import (
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/require"
)

// read folder from  embed fs and write to file system
func readEmbedFileToDisk(embedPath string, targetDir string) {
	// Read the file from the embedded filesystem
	entries, err := fs.ReadDir(embedPath)
	if err != nil {
		log.Fatalf("Failed to read asset directory in embedFs: %v", err)
	}

	for _, entry := range entries {

		childFs := path.Join(embedPath, entry.Name())
		childDir := filepath.Join(targetDir, entry.Name())

		// Recursively read the directory
		if entry.IsDir() {

			// Create the directory in the file system
			if err := os.MkdirAll(childDir, os.ModePerm); err != nil {
				log.Fatalf("Failed to create directory: %v", err)
			}

			readEmbedFileToDisk(childFs, childDir)
		} else {

			data, err := fs.ReadFile(childFs)
			if err != nil {
				log.Fatalf("Failed to read file %s from embedFs: %v", childFs, err)
			}

			// Create the file in the file system
			outFile, err := os.Create(childDir)
			if err != nil {
				log.Fatalf("Failed to create file %s: %v", childDir, err)
			}
			defer outFile.Close()

			// Write the content to the file
			if _, err := outFile.Write(data); err != nil {
				log.Fatalf("Failed to write file %s: %v", childDir, err)
			}
		}
	}
}

// Init nodeJs module
func (ser *ExtBaseService) initModule(module *require.RequireModule, vm *goja.Runtime, job *Job) {

	// init cryptoJs  and  linkedom
	linkeDom := filepath.Join(jsRoot, "linkedom", "worker.js")
	cryptoJs := filepath.Join(jsRoot, "crypto-js", "aes.js")

	if _, e := module.Require(linkeDom); e != nil {
		log.Println("linkedom module not found")
	}
	if _, e := module.Require(cryptoJs); e != nil {
		log.Println("crypto-js module not found")
	}

	vm.RunString(fmt.Sprintf(`var {parseHTML} = require('%s');`, linkeDom))
	vm.RunString(fmt.Sprintf(`var {AES} = require('%s');`, cryptoJs))
	ser.initFetch(vm, job)

}
