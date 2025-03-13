package extension

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"runtime/debug"
	"strings"

	"github.com/adrg/xdg"
	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/eventloop"
	"github.com/dop251/goja_nodejs/require"
)

type ExtBaseService struct {
	// User extension compile into goja program
	program *goja.Program
	// Base runtime (v1 or v2) compiles into goja program
	base *goja.Program
}

var SharedRegistry *require.Registry = require.NewRegistry()

var miruDir = xdg.UserDirs.Documents + "/miru"

// Entry point of miru extension runtime
func InitRuntime() {
	// vm := goja.New()

	exts := filterExt(miruDir)
	scriptV2 := string(handlerror(os.ReadFile("./assets/runtime_v2.js")))

	for _, ext := range exts {
		// V2
		if ext.api == "2" {
			scriptV2 := fmt.Sprintf(scriptV2, ext.pkg, ext.name, ext.website)
			// log.Println(scriptV2)
			compile := handlerror(goja.Compile(ext.pkg+".js", *ext.context, true))
			runtimeV2 := handlerror(goja.Compile("runtime_v2.js", scriptV2, true))
			api := &ExtApiV2{ext: &ext, service: &ExtBaseService{program: compile, base: runtimeV2}}

			Api2Cache[ext.name] = api
			api.loadApiV2()

		} else {
			// V1
			// api := &ExtApiV1{&ExtBaseService{ext: &ext, runtime: goja.New()}}
			// api.loadApiV1()
		}
	}
}

func request(url string, headers map[string]string) (string, error) {
	log.Println("Making request to:", url)

	// Create a new request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	// Add headers if provided in options
	for key, value := range headers {
		req.Header.Add(key, value)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// Return response as string
	return string(body), nil
}

func filterExt(dir string) []Ext {
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
func initModule(module *require.RequireModule, vm *goja.Runtime) {

	// init cryptoJs  and  linkedom

	module.Require("./assets/linkedom/worker.js")
	module.Require("./assets/crypto-js/aes.js")
	vm.RunString(`var CryptoJS = require('./assets/crypto-js/aes.js');`)
	vm.RunString(`var {parseHTML} = require('./assets/linkedom/worker.js');`)

}

// Promise for creating a single channel
func (ser *ExtBaseService) resolvePromise(resolve func(any) error, reason any, job *Job, loop *eventloop.EventLoop) {
	loop.RunOnLoop(func(r *goja.Runtime) {
		// 減少一個等待事件
		job.Done()
		// 完成異步方法
		resolve(reason)
	})
}

// Create a single channel
func (ser *ExtBaseService) createSingleChannel(vm *goja.Runtime, name string, job *Job, loop *eventloop.EventLoop, fun func(call goja.FunctionCall, resolve func(any) error) any) {
	vm.Set(name, func(call goja.FunctionCall) goja.Value {
		promise, resolve, _ := vm.NewPromise()
		// 增加一個等待事件
		job.Add()
		// 異步方法
		go func() {
			// Use RunOnLoop to dispatch function to event goroutine
			reason := fun(call, resolve)
			ser.resolvePromise(resolve, reason, job, loop)
		}()
		// 返回 promise
		return vm.ToValue(promise)
	})
}
