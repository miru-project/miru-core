package jsExtension

import (
	"embed"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/eventloop"
	"github.com/dop251/goja_nodejs/require"
	"github.com/fsnotify/fsnotify"
	errorhandle "github.com/miru-project/miru-core/errorHandle"
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

// jsRoot is the root directory for JavaScript files copy from the embedded filesystem
var jsRoot string
var ScriptV1 string
var ScriptV2 string

type ExtApi struct {
	Ext     *Ext
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
	ScriptV2 = string(errorhandle.HandleFatal(fs.ReadFile("assets/runtime_v2.js")))
	ScriptV1 = string(errorhandle.HandleFatal(fs.ReadFile("assets/runtime_v1.js")))

	defer func() {
		if r := recover(); r != nil {
			if err, ok := r.(map[string]string); ok {
				for pkg, msg := range err {
					ApiPkgCache[pkg].Ext.Error = msg
				}
				return
			}
			panic(r)
		}

	}()
	for _, ext := range exts {
		if ext.Api == "2" {
			LoadApiV2(&ext, ScriptV2)
		} else {
			LoadApiV1(&ext, ScriptV1)

		}
	}
}

func compileScript(ext *Ext) *goja.Program {
	compile, e := goja.Compile(ext.Pkg+".js", *ext.Context, true)
	if e != nil {
		panic(map[string]string{ext.Pkg: fmt.Sprintf("Error compiling extension %s: %v", ext.Pkg, e)})
	}
	return compile
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

					// Create a new extension runtime
					switch ext.Api {
					case "2":
						LoadApiV2(&ext, ScriptV2)
					case "1", "":
						LoadApiV1(&ext, ScriptV1)
					}

					// // Compile the extension file and update the cache
					// compile, e := goja.Compile(ext.Pkg+".js", *ext.Context, true)
					// if e != nil {
					// 	ApiPkgCache[ext.Pkg].Ext.Error = fmt.Sprintf("Error compiling extension %s: %v", ext.Pkg, e)
					// }

					// ApiPkgCache[ext.Pkg] = &ExtApi{
					// 	Ext:     &ext,
					// 	service: &ExtBaseService{program: compile}}

					// Reload the extension
					lop, ok := extMemMap.Load(ext.Pkg)
					if !ok {
						log.Println("Extension not found in memory map:", ext.Pkg)
						continue
					}

					loop := lop.(*eventloop.EventLoop)
					loop.Run(func(vm *goja.Runtime) {
						file := (errorhandle.HandleFatal(os.ReadFile(event.Name)))
						script := ReplaceClassExtendsDeclaration(string(file))
						if _, e := vm.RunString(script); e != nil {
							log.Println("Error running extension script:", e)
							return
						}

						if ext.Api == "1" || ext.Api == "" {
							vm.RunString(`ext = new Ext();
											ext.load();`)
						}

					})
					log.Println("Reloaded extension:", ext.Name, "-", ext.Pkg)
					ApiPkgCache[ext.Pkg].Ext.Error = ""
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
	re := regexp.MustCompile(`export\s+default\s+class.+extends\s+Extension\s*{`)
	return re.ReplaceAllString(jsCode, `Ext = class extends Extension {`)
}

func filterExts(dir string) []Ext {
	_, err := os.Stat(dir)
	if os.IsNotExist(err) {
		if e := os.Mkdir(dir, os.ModePerm); e != nil {
			log.Println("Failed to create directory:", dir)
			return nil
		}
	}
	files := errorhandle.HandleFatal(os.ReadDir(dir))
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
	f := errorhandle.HandleFatal(os.ReadFile(fileLoc))
	ext, err := ParseExtMetadata(string(f), name)
	if err != nil {
		log.Println(err)
		return Ext{}, false
	}
	return ext, true
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
			ext.Name = value
		case "version":
			ext.Version = value
		case "author":
			ext.Author = value
		case "license":
			ext.License = value
		case "lang":
			ext.Lang = value
		case "icon":
			ext.Icon = value
		case "package":
			ext.Pkg = value
		case "webSite":
			ext.Website = value
		case "description":
			ext.Description = value
		case "api":
			ext.Api = value
		case "type":
			ext.WatchType = value
		case "tags":
			// Split tags by comma and trim whitespace
			tagList := strings.Split(value, ",")
			for i, tag := range tagList {
				tagList[i] = strings.TrimSpace(tag)
			}
			ext.Tags = tagList
		}
	}

	// make sure package name + .js equals file name

	if ext.Pkg+".js" != fileName {
		err = errors.New("package name does not match the file name \r\n file name:" + fileName + "\r\n package name:" + ext.Pkg)
	}

	ext.Context = &content
	return ext, err
}

func isV2(pkg string) bool {
	api, ok := ApiPkgCache[pkg]
	return ok && api.Ext != nil && api.Ext.Api == "2"
}

func isV1(pkg string) bool {
	api, ok := ApiPkgCache[pkg]
	return ok && api.Ext != nil
}

// Extension latest should contain V1 and V2 api
func Latest(pkg string, page int) (ExtensionListItems, error) {
	if isV2(pkg) {
		evalStr := fmt.Sprintf("latest(%d)", page)
		return AsyncCallBackV2[ExtensionListItems](ApiPkgCache[pkg], pkg, evalStr)
	}
	if isV1(pkg) {
		evalStr := fmt.Sprintf("ext.latest(%d)", page)
		return AsyncCallBackV1[ExtensionListItems](ApiPkgCache[pkg], pkg, evalStr)
	}
	return ExtensionListItems{}, errors.New("extension not found")
}

// Extension search should contain V1 and V2 api
func Search(pkg string, page int, kw string, filter string) (ExtensionListItems, error) {
	evalStr := fmt.Sprintf("search(`%s`,%d,%s)", kw, page, filter)
	return AsyncCallBackV2[ExtensionListItems](ApiPkgCache[pkg], pkg, evalStr)

}

// Extension watch should contain V1 and V2 api
func Watch(pkg string, url string) (any, error) {

	api := ApiPkgCache[pkg]
	watchType := api.Ext.WatchType
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
	o, e := AsyncCallBackV2[ExtensionDetail](ApiPkgCache[pkg], pkg, evalStr)

	if e != nil {
		return ExtensionDetail{}, e
	}

	return o, nil
}
