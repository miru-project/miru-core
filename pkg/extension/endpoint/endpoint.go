package endpoint

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/miru-project/miru-core/pkg/extension"
	golang "github.com/miru-project/miru-core/pkg/extension/golang"
	js "github.com/miru-project/miru-core/pkg/extension/js"
	"github.com/miru-project/miru-core/proto/generate/proto"
)

// Language identifies which runtime backs a given extension package.
type Language int

const (
	Golang     Language = iota // 0
	JavaScript                 // 1
	Unknown    Language = -1
)

// Runtime abstracts a single extension runtime (JS or Golang). Both runtimes
// implement it; the JS implementation additionally handles the legacy v1 API,
// while the Golang implementation is v2-only.
//
// Note: JS and Golang satisfy this interface structurally (Go duck typing), so
// they do NOT import this package. That keeps the dependency graph acyclic:
// this package imports js and golang, while js/golang only depend on extension.
type Runtime interface {
	// GetExtensionMeta returns the parsed metadata for a package.
	GetExtensionMeta(pkg string) (*extension.Extension, error)
	// Latest returns the latest content for a package.
	Latest(pkg string, page int) ([]*proto.ExtensionListItem, error)
	// Search searches for content by keyword.
	Search(pkg string, page int, kw string, filter *proto.FilterSelection) ([]*proto.ExtensionListItem, error)
	// Watch returns watch/stream information. The returned *extension.Extension
	// carries the ApiVersion/WatchType used by callers to pick a response shape.
	Watch(pkg string, url string) (any, *extension.Extension, error)
	// Detail returns detailed information about a content item.
	Detail(pkg string, url string) (*proto.ExtensionDetail, error)
	// Mirror returns mirror/alternative URLs for a content item.
	Mirror(pkg string, url string) (any, error)
	// CreateFilter creates filter options for a package.
	CreateFilter(pkg string, filter *proto.FilterSelection) (map[string]*proto.ExtensionFilter, error)
}

// Unmarshal decodes an arbitrary value into T. It is a thin re-export of the
// runtime-agnostic helper in the extension package.
func Unmarshal[T any](input any) (*T, error) {
	return extension.Unmarshal[T](input)
}

// UnmarshalList decodes a list of arbitrary values into a slice of *T.
func UnmarshalList[T any](input any) ([]*T, error) {
	return extension.UnmarshalList[T](input)
}

// resolveExtensionRuntime determines which runtime owns a package by checking
// for its source file on disk.
func resolveExtensionRuntime(pkg string) Language {
	golangPath := filepath.Join(golang.ExtensionDir, pkg+string(extension.LanguageGolang))
	if _, err := os.Stat(golangPath); err == nil {
		return Golang
	}

	jsPath := filepath.Join(js.ExtPath, pkg+string(extension.LanguageJS))
	if _, err := os.Stat(jsPath); err == nil {
		return JavaScript
	}

	return Unknown
}

// IsGolang reports whether pkg is backed by the Go (Scriggo) extension runtime.
// The Golang runtime is v2-only, so callers (e.g. the gRPC handlers) use this
// to route it to the V2 response shapes unconditionally.
func IsGolang(pkg string) bool {
	return resolveExtensionRuntime(pkg) == Golang
}

// GetRuntime returns the Runtime implementation that owns the given package,
// selecting between the Golang and JavaScript backends.
func GetRuntime(pkg string) (Runtime, error) {
	switch resolveExtensionRuntime(pkg) {
	case Golang:
		return golang.ExtensionRuntime{}, nil
	case JavaScript:
		return js.Runtime{}, nil
	default:
		return nil, fmt.Errorf("no extension runtime found for package %q", pkg)
	}
}
