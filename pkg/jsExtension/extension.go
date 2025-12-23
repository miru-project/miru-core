package jsExtension

import (
	"embed"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	log "github.com/miru-project/miru-core/pkg/logger"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/eventloop"
	"github.com/dop251/goja_nodejs/require"
	"github.com/fsnotify/fsnotify"
	errorhandle "github.com/miru-project/miru-core/pkg/errorHandle"
)

// To complete an extension runtime, first it must compile base runtime then compile extension runtime
type ExtBaseService struct {
	// Base runtime (v1 or v2) compiles into goja program
	base *goja.Program
	// Extension program compiles into goja program
	program *goja.Program
}

var SharedRegistry *require.Registry = require.NewRegistry()
var fs embed.FS

// jsRoot is the root directory for JavaScript files copy from the embedded filesystem
var jsRoot string

var ScriptV1 string
var ScriptV2 string

var ExtPath string

type ExtApi struct {
	Ext           *Ext
	service       *ExtBaseService
	asyncCallBack func(api *ExtApi, pkg string, evalStr string) (any, error)
	latestEval    string
	searchEval    string
	detailEval    string
	watchEval     string
	lock          sync.Mutex
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
	exts := filterExts(extPath)
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
	readEmbedFileToDisk("assets", jsRoot)
	WatchDir(extPath)
	ScriptV2 = string(errorhandle.HandleFatal(fs.ReadFile("assets/runtime_v2.js")))
	ScriptV1 = string(errorhandle.HandleFatal(fs.ReadFile("assets/runtime_v1.js")))

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
		switch ext.Api {
		case "2":
			go LoadApiV2(ext, ScriptV2)
		default:
			go LoadApiV1(ext, ScriptV1)
		}
	}
}

func compileExtension(ext *Ext) (*goja.Program, error) {
	compile, e := goja.Compile(ext.Pkg+".js", *ext.Context, true)
	return compile, e
}

// Watch the extension directory for changes
func WatchDir(dir string) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal("Failed to start fsnotiy: ", err)
	}

	go func() {
		locked := false
		for {
			select {
			case event := <-watcher.Events:

				// Write or Modify event
				if event.Has(fsnotify.Write|fsnotify.Create) && !locked {
					log.Println("Modified file:", event.Name)
					locked = true
					ext := &Ext{Name: filepath.Base(event.Name)}
					err := ext.filterExt(event.Name)
					if err != nil {
						log.Println("File is not a valid extension:", event.Name)
						locked = false
						continue
					}

					ext.ReloadExtension()
					locked = false
				}

				// Call when file is missing
				if event.Has(fsnotify.Rename | fsnotify.Remove) {
					log.Println("Removed file:", event.Name)
					name := filepath.Base(event.Name)
					pkg := strings.TrimSuffix(name, ".js")
					ApiPkgCache.Delete(pkg)
				}
			case err := <-watcher.Errors:
				if err != nil {
					log.Println("Error:", err)
				}
			}
		}
	}()

	err = watcher.Add(dir)
	log.Println("Watching directory:", dir)
	if err != nil {
		log.Fatal(err)
	}
}
func (ext *Ext) ReloadExtension() error {

	// Create a new extension runtime
	switch ext.Api {
	case "2":
		LoadApiV2(ext, ScriptV2)
	case "1", "":
		LoadApiV1(ext, ScriptV1)
	}

	return nil
}

// ReplaceClassExtendsDeclaration replaces `class X extends Extension` with `X = class extends Extension {`
func ReplaceClassExtendsDeclaration(jsCode string) string {
	re := regexp.MustCompile(`(?m)^.*class.+extends\s+Extension\s*{.*$`)
	return re.ReplaceAllString(jsCode, "globalThis.Ext = class extends Extension {")
}

func filterExts(dir string) []*Ext {
	_, err := os.Stat(dir)
	if os.IsNotExist(err) {
		if e := os.Mkdir(dir, os.ModePerm); e != nil {
			log.Println("Failed to create directory:", dir)
			return nil
		}
	}
	files := errorhandle.HandleFatal(os.ReadDir(dir))
	var exts []*Ext
	for _, file := range files {

		if file.IsDir() {
			continue
		}
		name := file.Name()
		ext := &Ext{Name: name}
		if err := ext.filterExt(dir + "/" + name); err == nil {
			exts = append(exts, ext)
		} else {
			ApiPkgCache.Store(name, &ExtApi{Ext: &Ext{Name: name}, service: nil})
		}
	}
	return exts
}

func (ext *Ext) filterExt(fileLoc string) error {
	name := filepath.Base(fileLoc)
	re := regexp.MustCompile(`\w.+\.\w+\.js$`)
	if !(re.MatchString(name)) {
		return errors.New("invalid file name")
	}
	if _, e := os.Stat(fileLoc); os.IsNotExist(e) {
		return e
	}
	f := errorhandle.HandleFatal(os.ReadFile(fileLoc))
	err := ext.ParseExtMetadata(string(f), name)
	if err != nil {
		log.Println(err)
		return err
	}
	return nil
}

func (ext *Ext) ParseExtMetadata(content string, fileName string) error {
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
	return err
}

func getPkgFromCache(pkg string) (*ExtApi, error) {
	api, ok := ApiPkgCache.Map.Load(pkg)
	if ok {
		return api.(*ExtApi), nil
	}
	ext := &Ext{Name: pkg + ".js"}
	fileLoc, e := os.ReadFile(filepath.Join(ExtPath, pkg+".js"))
	if e != nil {
		return nil, e
	}
	e = ext.filterExt(string(fileLoc))
	if e != nil {
		return nil, e
	}

	if e := ext.ReloadExtension(); e != nil {
		return nil, e
	}
	return ApiPkgCache.Load(pkg), nil
}
