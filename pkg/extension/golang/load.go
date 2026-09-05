package golang

import (
	"log"
	"os"
	"strings"

	"github.com/miru-project/miru-core/pkg/extension"
	"github.com/miru-project/miru-core/pkg/extension/golang/runtime"
)

// LoadExtensions scans ExtensionDir and eagerly loads every Go/Scriggo
// extension through its Load entry point. This mirrors the JavaScript
// runtime's startup scan (jsext.InitRuntime) so that Go extensions are
// visibly loaded at boot -- each prints "Extension loaded (Vx) [GO]: ..." --
// and any compile error is surfaced early.
//
// Extensions that have no Load entry point (they are driven solely by the
// endpoint functions, e.g. Search/Latest, such as the sample example
// extension) cannot use this eager path and continue to be compiled lazily on
// first request; they are skipped here without error.
func LoadExtensions() {
	if ExtensionDir == "" {
		return
	}
	entries, err := os.ReadDir(ExtensionDir)
	if err != nil {
		log.Println("Failed to read extension directory:", ExtensionDir, err)
		return
	}
	for _, f := range entries {
		if f.IsDir() {
			continue
		}
		// Only handle Go extensions; the JS runtime is responsible for .js.
		if extension.DetectLanguage(f.Name()) != extension.LanguageGolang {
			continue
		}
		pkg := strings.TrimSuffix(f.Name(), string(extension.LanguageGolang))
		ext, perr := ParseExtensionMetadata(pkg)
		if perr != nil {
			log.Println("Extension load error:", pkg, perr)
			continue
		}
		rt := NewRuntime(NewScriggoVM(nil))
		if lerr := rt.LoadExtension(ext); lerr != nil {
			// No Load entry point (lazy-only extension): loaded on demand.
			continue
		}
	}
}

// GetExtensions returns the parsed metadata of every Go/Scriggo extension
// available in ExtensionDir. Unlike LoadExtensions (which only eagerly runs
// extensions that declare a Load entry point), this lists all discoverable
// .go extensions so the frontend can present them alongside the JavaScript
// extensions. The returned metadata has Context (source) stripped to keep the
// payload small. A file that fails to parse is returned with its Error field
// set rather than being omitted.
func GetExtensions() []*extension.Extension {
	out := make([]*extension.Extension, 0)
	if ExtensionDir == "" {
		return out
	}
	entries, err := os.ReadDir(ExtensionDir)
	if err != nil {
		return out
	}
	for _, f := range entries {
		if f.IsDir() {
			continue
		}
		if extension.DetectLanguage(f.Name()) != extension.LanguageGolang {
			continue
		}
		pkg := strings.TrimSuffix(f.Name(), string(extension.LanguageGolang))
		ext, perr := ParseExtensionMetadata(pkg)
		if perr != nil {
			ext = &extension.Extension{Name: pkg, Pkg: pkg, Error: perr.Error()}
		}
		ext.Context = nil
		out = append(out, ext)
	}
	return out
}

// OnExtensionUpdate is invoked whenever the set of Go/Scriggo extensions on
// disk may have changed, so the gRPC layer can republish the merged extension
// list to the frontend. Mirrors js.OnExtensionUpdate; the gRPC server wires it
// at startup, so it is nil during the initial boot scan (no subscribers yet).
var OnExtensionUpdate func()

// HandleReload is the Go-specific reload handler invoked by the unified
// extension watcher when a .go file changes. It invalidates the per-package
// cross-call variable cache and the per-package native packages map so the
// next request recompiles against the updated source.
func HandleReload(pkg string) {
	runtime.DeleteCache(pkg)
	pkgPackages.Delete(pkg)
	log.Println("Go extension changed, reloading:", pkg)
	if OnExtensionUpdate != nil {
		OnExtensionUpdate()
	}
}
