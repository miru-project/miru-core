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
	"sync"
	"time"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/eventloop"
	"github.com/dop251/goja_nodejs/require"
	"github.com/fsnotify/fsnotify"
	// "github.com/miru-project/miru-core/ext"
)

var extMemMap = sync.Map{}

type ExtBaseService struct {
	// User extension compile into goja program
	program *goja.Program
	// Base runtime (v1 or v2) compiles into goja program
	base *goja.Program
}

var SharedRegistry *require.Registry = require.NewRegistry()
var fs embed.FS
var jsRoot string

type ExtApiV2 struct {
	ext     *Ext
	service *ExtBaseService
}

type Job struct {
	loop  *eventloop.EventLoop
	flag  *eventloop.Interval
	count uint64
}

type PromiseResult struct {
	promise *goja.Promise
	err     error
}

func (j *Job) Add() {
	j.count++

	if j.count == 1 {
		j.flag = j.loop.SetInterval(func(r *goja.Runtime) {}, time.Hour*24*365*100)
	}
}
func (j *Job) Done() {
	j.count--

	if j.count == 0 {
		j.loop.ClearInterval(j.flag)
		j.flag = nil
	}
}

// Entry point of miru extension runtime
func InitRuntime(extPath string, f embed.FS) {
	exts := filterExts(extPath)
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
	WatchDir(extPath)
	scriptV2 := string(handlerror(fs.ReadFile("assets/runtime_v2.js")))
	scriptV1 := string(handlerror(fs.ReadFile("assets/runtime_v1.js")))

	for _, ext := range exts {
		if ext.api == "2" {
			LoadApiV2(&ext, scriptV2)
		} else {
			LoadApiV1(&ext, scriptV1)

		}
	}
}

// Watch the extension directory for changes
func WatchDir(dir string) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	// defer watcher.Close()

	go func() {
		for {
			select {
			case event := <-watcher.Events:
				if event.Has(fsnotify.Write) {
					log.Println("Modified file:", event.Name)

					ext, ok := filterExt(event.Name)
					if !ok {
						log.Println("File is not a valid extension:", event.Name)
						continue
					}

					// Compile the extension file and update the cache
					compile := handlerror(goja.Compile(ext.pkg+".js", *ext.context, true))
					switch ext.api {
					case "2":
						ApiPkgCacheV2[ext.pkg].service.program = compile
					default:
						ApiPkgCacheV1[ext.pkg].service.program = compile
					}

					// Reload the extension
					lop, ok := extMemMap.Load(ext.pkg)
					if !ok {
						log.Println("Extension not found in memory map:", ext.pkg)
						continue
					}

					loop := lop.(*eventloop.EventLoop)
					loop.Run(func(vm *goja.Runtime) {
						file := (handlerror(os.ReadFile(event.Name)))
						script := ReplaceClassExtendsDeclaration(string(file))
						if _, e := vm.RunString(script); e != nil {
							log.Println("Error running extension script:", e)
							return
						}

						if ext.api == "1" || ext.api == "" {
							vm.RunString("ext = new Ext();")
						}

					})
					log.Println("Reloaded extension:", ext.name, "-", ext.pkg)
				}
			case err := <-watcher.Errors:
				if err != nil {
					log.Println("Error:", err)
				}
			}
		}
	}()

	err = watcher.Add(dir)
	if err != nil {
		log.Fatal(err)
	}
}

// ReplaceClassExtendsDeclaration replaces `class X extends Extension` with `X = class extends Extension {`
func ReplaceClassExtendsDeclaration(jsCode string) string {
	re := regexp.MustCompile(`(?m)^class\s+([A-Za-z_][A-Za-z0-9_]*)\s+extends\s+Extension\s*{`)
	return re.ReplaceAllString(jsCode, `$1 = class extends Extension {`)
}

//	func ReloadExt(ext *Ext) bool {
//		switch ext.api{
//		case "2":
//			val,ok := extMemMap.Load(ext.pkg)
//			if !ok{
//				return false
//			}
//			val.()
//		}
//	}
func filterExts(dir string) []Ext {
	_, err := os.Stat(dir)
	if os.IsNotExist(err) {
		if e := os.Mkdir(dir, os.ModePerm); e != nil {
			log.Println("Failed to create directory:", dir)
			return nil
		}
	}
	files := handlerror(os.ReadDir(dir))
	var exts []Ext
	for _, file := range files {

		if file.IsDir() {
			continue
		}
		name := file.Name()
		if ext, ok := filterExt(dir + "/" + name); ok {
			exts = append(exts, ext)
		}
	}
	return exts
}

func filterExt(fileLoc string) (Ext, bool) {
	name := filepath.Base(fileLoc)
	re := regexp.MustCompile(`\w.+\.\w+\.js`)

	if _, e := os.Stat(fileLoc); !re.MatchString(name) || os.IsNotExist(e) {
		return Ext{}, false
	}
	f := handlerror(os.ReadFile(fileLoc))
	ext, err := ParseExtMetadata(string(f), name)
	if err != nil {
		log.Println(err)
		return Ext{}, false
	}
	return ext, true
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

func isV2(pkg string) bool {
	api, ok := ApiPkgCacheV2[pkg]
	return ok && api.ext != nil && api.ext.api == "2"
}

func isV1(pkg string) bool {
	api, ok := ApiPkgCacheV1[pkg]
	return ok && api.ext != nil
}

// Extension latest should contain V1 and V2 api
func Latest(pkg string, page int) (ExtensionListItems, error) {
	if isV2(pkg) {
		evalStr := fmt.Sprintf("latest(%d)", page)
		return AsyncCallBackV2[ExtensionListItems](ApiPkgCacheV2[pkg], pkg, evalStr)
	}
	if isV1(pkg) {
		evalStr := fmt.Sprintf("ext.latest(%d)", page)
		return AsyncCallBackV1[ExtensionListItems](ApiPkgCacheV1[pkg], pkg, evalStr)
	}
	return ExtensionListItems{}, errors.New("extension not found")
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
