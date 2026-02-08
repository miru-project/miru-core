package jsExtension

import (
	"fmt"
	"os"
	"path"
	"path/filepath"

	errorhandle "github.com/miru-project/miru-core/pkg/errorHandle"
	log "github.com/miru-project/miru-core/pkg/logger"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/console"
	"github.com/dop251/goja_nodejs/require"
	"github.com/dop251/goja_nodejs/url"
)

// read folder from  embed fs and write to file system
func readEmbedFileToDisk(embedPath string, targetDir string) {
	// Read the file from the embedded filesystem
	entries, err := fs.ReadDir(embedPath)
	if err != nil {
		errorhandle.PanicF("Failed to read asset directory in embedFs: %v", err)
	}

	for _, entry := range entries {

		childFs := path.Join(embedPath, entry.Name())
		childDir := filepath.Join(targetDir, entry.Name())

		// Recursively read the directory
		if entry.IsDir() {

			// Create the directory in the file system
			if err := os.MkdirAll(childDir, os.ModePerm); err != nil {
				errorhandle.PanicF("Failed to create directory: %v", err)
			}

			readEmbedFileToDisk(childFs, childDir)
		} else {

			data, err := fs.ReadFile(childFs)
			if err != nil {
				errorhandle.PanicF("Failed to read file %s from embedFs: %v", childFs, err)
			}

			// Create the file in the file system
			outFile, err := os.Create(childDir)
			if err != nil {
				errorhandle.PanicF("Failed to create file %s: %v", childDir, err)
			}
			defer outFile.Close()

			// Write the content to the file
			if _, err := outFile.Write(data); err != nil {
				errorhandle.PanicF("Failed to write file %s: %v", childDir, err)
			}
		}
	}
}

// Init nodeJs module
func (ser *ExtBaseService) initModule(module *require.RequireModule, vm *goja.Runtime, job *Job) {
	ser.initCrypto(vm)
	url.Enable(vm)
	console.Enable(vm)
	// init cryptoJs  and  linkedom
	linkeDom := filepath.Join(jsRoot, "linkedom", "worker.js")
	cryptoJs := filepath.Join(jsRoot, "crypto-js", "crypto-js.js")
	md5 := filepath.Join(jsRoot, "md5", "md5.min.js")
	if _, e := module.Require(linkeDom); e != nil {
		log.Println(e.Error())
	}
	if _, e := module.Require(cryptoJs); e != nil {
		log.Println(e.Error())
	}
	if _, e := module.Require(md5); e != nil {
		log.Println(e.Error())
	}

	vm.RunString(fmt.Sprintf(`var {parseHTML} = require('%s');`, linkeDom))
	vm.RunString(fmt.Sprintf(`var {CryptoJS} = require('%s');`, cryptoJs))
	vm.RunString(fmt.Sprintf(`var {md5} = require('%s');`, md5))
	ser.initFetch(vm, job)

}
