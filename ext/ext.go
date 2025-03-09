package ext

import (
	"errors"
	"log"
	"os"
	"regexp"
	"runtime/debug"
	"strings"

	// "time"

	"github.com/adrg/xdg"
	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/require"
)

var SharedRegistry *require.Registry = require.NewRegistry()

var miruDir = xdg.UserDirs.Documents + "/miru"

func InitRuntime() {
	// vm := goja.New()

	exts := filterExt(miruDir)
	for _, ext := range exts {
		if ext.api == "2" {
			api := &ExtApiV2{runtime: goja.New(), ext: ext}
			api.loadApiV2()
		} else {
			api := &ExtApiV1{runtime: goja.New(), ext: ext}
			api.loadApiV1()
		}
	}
}

func request(url string, options map[string]map[string]string) {
	log.Println(url)

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
		case "website":
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
		err = errors.New("package name does not match the file name")
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

	vm.Set("jsRequest", request)
}

//	func (ext *ExtApiV1) testAsync() {
//		vm := ext.runtime
//		_, e := vm.RunString(`console.log("Hello, world!")`)
//		if e != nil {
//			log.Printf("Error: %v", e)
//		}
//		vm.Set("sleep", ext.jsSleep)
//		_, e = vm.RunString(`
//		async function hello() {
//			console.log(Date.now()/1000);
//			console.log("Hello, world!");
//			await sleep();
//			console.log(Date.now()/1000);
//			return "Hello, world!";
//		}
//		`)
//		if e != nil {
//			log.Printf("Error: %v", e)
//		}
//		call, ok := goja.AssertFunction(vm.Get("hello"))
//		if !ok {
//			log.Printf("Error: %v", e)
//		}
//		_, e = call(nil)
//		log.Println(`end`)
//	}
// func (n *ExtApiV1) jsSleep() {
// 	time.Sleep(5 * time.Second)
// 	log.Println("sleep")
// }
