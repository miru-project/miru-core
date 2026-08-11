package golang

import (
	"fmt"
	"sync"
	"time"

	"github.com/miru-project/miru-core/pkg/event"
	runtime "github.com/miru-project/miru-core/pkg/extension/golang/runtime"
	log "github.com/miru-project/miru-core/pkg/logger"
	"github.com/miru-project/miru-core/proto/generate/proto"
	"github.com/open2b/scriggo/native"
)

var pkgPackages sync.Map

func packagesForPkg(pkg string) native.Packages {
	if pkg == "" {
		return packages
	}
	if v, ok := pkgPackages.Load(pkg); ok {
		return v.(native.Packages)
	}
	pkgs := clonePackagesMap(packages)
	replaceFmtForPkg(pkgs, pkg)
	replaceFetchForPkg(pkgs, pkg, "github.com/miru-project/miru-core/pkg/extension/golang/sdk")
	replaceFetchForPkg(pkgs, pkg, "github.com/miru-project/miru-core/pkg/extension/golang/runtime")
	pkgPackages.Store(pkg, pkgs)
	return pkgs
}

func clonePackagesMap(src native.Packages) native.Packages {
	dst := make(native.Packages, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// packageDecls extracts and clones the Declarations map from a native.Package inside the
// ImportablePackage value. Returns a copy of the native.Package with cloned
// Declarations so callers can mutate it safely.
func implPkg(sourcePkgs native.Packages, path string) native.Package {
	ip, ok := sourcePkgs[path]
	if !ok {
		return native.Package{}
	}
	p, ok := ip.(native.Package)
	if !ok {
		return native.Package{}
	}
	decls := make(native.Declarations, len(p.Declarations))
	for k, v := range p.Declarations {
		decls[k] = v
	}
	return native.Package{Name: p.Name, Declarations: decls}
}

func replaceFmtForPkg(pkgs native.Packages, pkg string) {
	p := implPkg(pkgs, "fmt")
	decls := p.Declarations

	if orig, ok := decls["Print"].(func(...any) (int, error)); ok {
		decls["Print"] = func(a ...any) (int, error) {
			sendDevLog(pkg, "info", fmt.Sprint(a...))
			return orig(a...)
		}
	}
	if orig, ok := decls["Printf"].(func(string, ...any) (int, error)); ok {
		decls["Printf"] = func(format string, a ...any) (int, error) {
			sendDevLog(pkg, "info", fmt.Sprintf(format, a...))
			return orig(format, a...)
		}
	}
	if orig, ok := decls["Println"].(func(...any) (int, error)); ok {
		decls["Println"] = func(a ...any) (int, error) {
			sendDevLog(pkg, "info", fmt.Sprintln(a...))
			return orig(a...)
		}
	}

	pkgs["fmt"] = p
}

func replaceFetchForPkg(pkgs native.Packages, pkg string, path string) {
	p := implPkg(pkgs, path)
	if p.Name == "" {
		return
	}
	decls := p.Declarations

	fetch := decls["Fetch"]
	if fetch == nil {
		return
	}

	type FetchT = func(url, method string, headers map[string]string, body string, tls *runtime.TLSConfig) (string, int, string)
	var realFetch FetchT
	switch v := fetch.(type) {
	case *FetchT:
		realFetch = *v
	case FetchT:
		realFetch = v
	default:
		return
	}

	decls["Fetch"] = func(url, method string, headers map[string]string, body string, tls *runtime.TLSConfig) (string, int, string) {
		start := time.Now()
		respBody, status, errStr := realFetch(url, method, headers, body, tls)
		duration := time.Since(start).Milliseconds()

		if event.GlobalBus.HasSubscribers() {
			respBodyCap := respBody
			if len(respBodyCap) > 100<<10 {
				respBodyCap = respBodyCap[:100<<10]
			}
			event.SendDevNetwork(&proto.DevNetworkEvent{
				Package:         pkg,
				Url:             url,
				Method:          method,
				Status:          int32(status),
				Duration:        duration,
				Timestamp:       time.Now().UnixMilli(),
				RequestHeaders:  fmt.Sprintf("%v", headers),
				RequestBody:     body,
				ResponseHeaders: "",
				ResponseBody:    respBodyCap,
			})
		}

		return respBody, status, errStr
	}

	pkgs[path] = p
}

func sendDevLog(pkg, level, msg string) {
	event.SendDevLog(&proto.DevLogEvent{
		Package:   pkg,
		Message:   msg,
		Level:     level,
		Timestamp: time.Now().UnixMilli(),
	})
	log.Println(fmt.Sprintf("[%s] %s", pkg, msg))
}