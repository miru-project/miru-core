package jsExtension

import (
	"embed"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"strings"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/require"
)

type ExtBaseService struct {
	// User extension compile into goja program
	program *goja.Program
	// Base runtime (v1 or v2) compiles into goja program
	base *goja.Program
}

var SharedRegistry *require.Registry = require.NewRegistry()
var fs embed.FS
var jsRoot string

// Entry point of miru extension runtime
func InitRuntime(extPath string, f embed.FS) {
	exts := filterExt(extPath)
	fs = f

	jsRoot = filepath.Join(extPath, "root")

	// create js root directory if not exist
	if _, err := os.Stat(jsRoot); os.IsNotExist(err) {
		if err := os.Mkdir(jsRoot, os.ModePerm); err != nil {
			log.Println("Failed to create directory:", jsRoot)
			return
		}
	}

	readEmbedFileToDisk("assets", jsRoot)
	scriptV2 := string(handlerror(fs.ReadFile("assets/runtime_v2.js")))

	for _, ext := range exts {
		// V2
		if ext.api == "2" {
			LoadApiV2(&ext, scriptV2)
		} else {
			// V1

		}
	}
}

func filterExt(dir string) []Ext {
	_, err := os.Stat(dir)
	if os.IsNotExist(err) {
		if e := os.Mkdir(dir, os.ModePerm); e != nil {
			log.Println("Failed to create directory:", dir)
			return nil
		}
	}
	files := handlerror(os.ReadDir(dir))
	re := regexp.MustCompile(`\w.+\.\w+\.js`)
	var exts []Ext
	for _, file := range files {
		if !file.IsDir() {
			name := file.Name()
			if !re.MatchString(name) {
				continue
			}
			f := handlerror(os.ReadFile(dir + "/" + name))
			ext, err := ParseExtMetadata(string(f), name)
			if err != nil {
				log.Println(err)
				continue
			}
			exts = append(exts, ext)

		}
	}
	return exts
}

func handlerror[T any](out T, err error) T {
	if err != nil {
		log.Fatal(err)
		debug.PrintStack()
		stackTrace := string(debug.Stack())
		log.Println("Stack trace:", stackTrace)
	}
	return out
}

func ParseExtMetadata(content string, fileName string) (Ext, error) {
	ext := Ext{}
	err := error(nil)

	// Regex to match @key value pattern

	re := regexp.MustCompile(`@(\w+)\s+(.*)`)
	matches := re.FindAllStringSubmatch(content, -1)

	for _, match := range matches {
		key := match[1]
		value := strings.TrimSpace(match[2])

		switch key {
		case "name":
			ext.name = value
		case "version":
			ext.version = value
		case "author":
			ext.author = value
		case "license":
			ext.license = value
		case "lang":
			ext.lang = value
		case "icon":
			ext.icon = value
		case "package":
			ext.pkg = value
		case "webSite":
			ext.website = value
		case "description":
			ext.description = value
		case "api":
			ext.api = value
		case "type":
			ext.watchType = value
		case "tags":
			// Split tags by comma and trim whitespace
			tagList := strings.Split(value, ",")
			for i, tag := range tagList {
				tagList[i] = strings.TrimSpace(tag)
			}
			ext.tags = tagList
		}
	}

	// make sure package name + .js equals file name

	if ext.pkg+".js" != fileName {
		err = errors.New("package name does not match the file name \r\n file name:" + fileName + "\r\n package name:" + ext.pkg)
	}

	ext.context = &content
	return ext, err
}

// Init nodeJs module
func initModule(module *require.RequireModule, vm *goja.Runtime) {

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

}

// read folder from  embed fs and write to file system
func readEmbedFileToDisk(path string, tagetDir string) {
	// Read the file from the embedded filesystem
	data, err := fs.ReadDir(path)
	if err != nil {
		log.Fatalf("Failed to read asset directory in embedFs: %v", err)
	}

	for _, file := range data {

		childFs := filepath.Join(path, file.Name())
		childDir := filepath.Join(tagetDir, file.Name())

		// Recursively read the directory
		if file.IsDir() {

			// Create the directory in the file system
			if err := os.MkdirAll(childDir, os.ModePerm); err != nil {
				log.Fatalf("Failed to create directory: %v", err)
			}

			readEmbedFileToDisk(childFs, childDir)
		} else {

			file, err := fs.ReadFile(childFs)
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
			if _, err := outFile.Write(file); err != nil {
				log.Fatalf("Failed to write file %s: %v", childDir, err)
			}
		}
	}
}

// Promise for creating a single channel
func (ser *ExtBaseService) resolvePromise(resolve func(any) error, reason any, job *Job) {

	job.Done()
	resolve(reason)

}
func (ser *ExtBaseService) rejectPromise(reject func(any) error, reason any, job *Job) {

	job.Done()
	reject(reason)
}

// Create a single channel
func (ser *ExtBaseService) createSingleChannel(vm *goja.Runtime, name string, job *Job, fun func(call goja.FunctionCall, resolve func(any) error) any) {

	vm.Set(name, func(call goja.FunctionCall) goja.Value {
		promise, resolve, reject := vm.NewPromise()
		// 增加一個等待事件
		job.Add()
		// 異步方法
		go func() {

			defer func() {
				if r := recover(); r != nil {
					log.Println("Recovered from panic:", r)
					ser.rejectPromise(reject, r, job)
				}
			}()

			// Use RunOnLoop to dispatch function to event goroutine
			reason := fun(call, resolve)
			ser.resolvePromise(resolve, reason, job)

		}()
		// 返回 promise
		return vm.ToValue(promise)
	})
}

// Extension latest should contain V1 and V2 api
func Latest(pkg string, page int) (ExtensionListItems, error) {
	evalStr := fmt.Sprintf("latest(%d)", page)
	return AsyncCallBackV2[ExtensionListItems](ApiPkgCacheV2[pkg], pkg, evalStr)

}

// Extension search should contain V1 and V2 api
func Search(pkg string, page int, kw string, filter string) (ExtensionListItems, error) {
	evalStr := fmt.Sprintf("search(`%s`,%d,%s)", kw, page, filter)
	return AsyncCallBackV2[ExtensionListItems](ApiPkgCacheV2[pkg], pkg, evalStr)

}

// Extension watch should contain V1 and V2 api
func Watch(pkg string, url string) (any, error) {

	api := ApiPkgCacheV2[pkg]
	watchType := api.ext.watchType
	funcall := fmt.Sprintf("watch(`%s`)", url)

	switch watchType {
	case "manga":
		return AsyncCallBackV2[ExtensionMangaWatch](api, pkg, funcall)
	case "bangumi":
		return AsyncCallBackV2[ExtensionBangumiWatch](api, pkg, funcall)
	case "fikushon":
		return AsyncCallBackV2[ExtensionFikushonWatch](api, pkg, funcall)
	default:
		return "Invalid watch type", errors.New("invalid watch type")
	}

}

func Detail(pkg string, url string) (ExtensionDetail, error) {

	evalStr := fmt.Sprintf("detail(`%s`)", url)
	o, e := AsyncCallBackV2[ExtensionDetail](ApiPkgCacheV2[pkg], pkg, evalStr)

	if e != nil {
		return ExtensionDetail{}, e
	}

	return o, nil
}
