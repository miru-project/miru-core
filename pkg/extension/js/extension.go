package js

import (
	"embed"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"github.com/miru-project/miru-core/pkg/extension"
	log "github.com/miru-project/miru-core/pkg/logger"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/eventloop"
	"github.com/dop251/goja_nodejs/require"
	errorhandle "github.com/miru-project/miru-core/pkg/errorHandle"
)

// To complete an extension runtime, first it must compile base runtime then compile extension runtime
type ExtBaseService struct {
	// Extension program compiles into goja program
	program *goja.Program
}

// Base runtime (v1 or v2) compiles into goja program
var baseV1 *goja.Program
var baseV2 *goja.Program
var sharedRegistry *require.Registry
var fs embed.FS

// jsRoot is the root directory for JavaScript files copy from the embedded filesystem
var jsRoot string

var ExtPath string

type Ext = extension.Extension

type ExtApi struct {
	Ext              *Ext
	service          *ExtBaseService
	asyncCallBack    func(api *ExtApi, pkg string, evalStr string) (any, error)
	latestEval       string
	searchEval       string
	detailEval       string
	watchEval        string
	mirrorEval       string
	createFilterEval string
	lock             sync.Mutex
}

type Job struct {
	loop  *eventloop.EventLoop
	flag  *eventloop.Interval
	count uint64
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

	exts, invalid := extension.FilterExtensions(extPath)
	for name, msg := range invalid {
		ApiPkgCache.Store(name, &ExtApi{Ext: &Ext{Name: name}, service: nil})
		log.Println("Extension load error:", name, msg)
	}
	fs = f
	ExtPath = extPath

	jsRoot = filepath.Join(extPath, "root")

	// create js root directory if not exist
	if _, err := os.Stat(jsRoot); os.IsNotExist(err) {
		if err := os.Mkdir(jsRoot, os.ModePerm); err != nil {
			log.Println("Failed to create directory:", jsRoot)
			return
		}
	}

	// Embeded file are externel js library that need to copy to jsRoot so that
	// goja can require them as node js module
	// readEmbedFileToDisk("assets", jsRoot)
	ScriptV1 := string(errorhandle.HandleFatal(fs.ReadFile("assets/runtime_v1.js")))
	ScriptV2 := string(errorhandle.HandleFatal(fs.ReadFile("assets/runtime_v2.js")))
	baseV1 = errorhandle.HandleFatal(goja.Compile("runtime_v1.js", ScriptV1, true))
	baseV2 = errorhandle.HandleFatal(goja.Compile("runtime_v2.js", ScriptV2, true))

	sharedRegistry = require.NewRegistry()
	initModule()
	defer func() {
		if r := recover(); r != nil {
			if err, ok := r.(map[string]string); ok {
				for pkg, msg := range err {
					ApiPkgCache.SetError(pkg, msg)
				}
				return
			}
			panic(r)
		}

	}()
	for _, ext := range exts {
		if ext.FileLang != extension.LanguageJS {
			continue
		}
		loadExtApi(ext)
	}
}

func loadExtApi(ext *Ext) {
	switch ext.ApiVersion {
	case "2":
		go LoadApiV2(ext)
	default:
		go LoadApiV1(ext)
	}
}

// Compile the js before evaluating it
func compileExtension(ext *Ext) (*goja.Program, error) {
	compile, e := goja.Compile(ext.Pkg+".js", *ext.Context, true)
	if e != nil {
		log.Println("Error compiling extension:", e)
		ext.Error = e.Error()
		ApiPkgCache.Store(ext.Pkg, &ExtApi{Ext: ext, service: nil})
		return nil, e
	}
	return compile, e
}

// HandleReload is the JS-specific reload handler invoked by the unified
// extension watcher when a .js file changes.
func HandleReload(pkg string) {
	fileLoc := filepath.Join(ExtPath, pkg+string(extension.LanguageJS))
	if _, statErr := os.Stat(fileLoc); os.IsNotExist(statErr) {
		ApiPkgCache.Delete(pkg)
		return
	}
	f, readErr := os.ReadFile(fileLoc)
	if readErr != nil {
		log.Println("File is not a valid extension:", fileLoc)
		return
	}
	ext, parseErr := extension.ParseExtensionMetadata(string(f), pkg+string(extension.LanguageJS))
	if parseErr != nil {
		log.Println("File is not a valid extension:", fileLoc, parseErr)
		return
	}
	deleteExtVarCache(pkg)
	loadExtApi(ext)
}

// replaceClassExtendsDeclaration replaces `class X extends Extension` with `X = class extends Extension {`
func replaceClassExtendsDeclaration(jsCode string) string {
	re := regexp.MustCompile(`(?m)^.*class.+extends\s+Extension\s*{.*$`)
	return re.ReplaceAllString(jsCode, "globalThis.Ext = class extends Extension {")
}

// filterExts, filterExt and ParseExtMetadata have moved to the shared
// pkg/extension package (see extension.FilterExtensions and
// extension.ParseExtensionMetadata), which parses metadata universally for
// both the JavaScript and Golang runtimes and routes by file extension.

func getPkgFromCache(pkg string) (*ExtApi, error) {
	api, ok := ApiPkgCache.Map.Load(pkg)
	if ok {
		return api.(*ExtApi), nil
	}
	path := filepath.Join(ExtPath, pkg+string(extension.LanguageJS))
	f, e := os.ReadFile(path)
	if e != nil {
		return nil, e
	}
	ext, e := extension.ParseExtensionMetadata(string(f), pkg+string(extension.LanguageJS))
	if e != nil {
		return nil, e
	}

	loadExtApi(ext)
	return ApiPkgCache.Load(pkg), nil
}
