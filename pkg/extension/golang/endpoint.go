// Package golang implements the Go (Scriggo) extension runtime.
//
// This file holds the public endpoint surface only -- the six entry points the
// host calls on an extension, plus metadata access. The supporting machinery
// lives alongside it:
//
//	invoke.go          compiling an extension and calling an entry point
//	filter_bridge.go   proto <-> runtime filter conversion
//	convert_reflect.go generic reflection field readers
//	convert_list.go    list/detail/mirror-group converters
//	convert_watch.go   watch-shape routing and per-type watch converters
//	media_policy.go    content-type classification, TLS profile, proxy URLs
package golang

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/miru-project/miru-core/pkg/extension"
	"github.com/miru-project/miru-core/proto/generate/proto"
)

// ExtensionDir is the directory where Scriggo extensions are stored.
var ExtensionDir string

// ParseExtensionMetadata parses metadata from a Scriggo extension source file.
// It delegates to the shared, runtime-agnostic parser in the extension package.
func ParseExtensionMetadata(pkg string) (*extension.Extension, error) {
	extPath := filepath.Join(ExtensionDir, pkg+string(extension.LanguageGolang))
	source, err := os.ReadFile(extPath)
	if err != nil {
		return nil, fmt.Errorf("read extension source %s: %w", extPath, err)
	}
	return extension.ParseExtensionMetadata(string(source), pkg+string(extension.LanguageGolang))
}

// GetExtensionMeta returns the parsed metadata for a package.
func GetExtensionMeta(pkg string) (*extension.Extension, error) {
	return ParseExtensionMetadata(pkg)
}

// Search searches for content by keyword.
func Search(pkg string, page int, kw string, filter *proto.FilterSelection) ([]*proto.ExtensionListItem, error) {
	res, err := callExtension(pkg, "Search", pkg, kw, page, filterFromProto(filter))
	if err != nil {
		return nil, err
	}
	return toExtensionListItems(res), nil
}

// Latest returns the latest content for a package.
func Latest(pkg string, page int) ([]*proto.ExtensionListItem, error) {
	res, err := callExtension(pkg, "Latest", pkg, page)
	if err != nil {
		return nil, err
	}
	return toExtensionListItems(res), nil
}

// Detail returns detailed information about a content item.
func Detail(pkg string, url string) (*proto.ExtensionDetail, error) {
	res, err := callExtension(pkg, "Detail", pkg, url)
	if err != nil {
		return nil, err
	}
	return toDetail(res), nil
}

// CreateFilter creates filter options for a package. It invokes the
// extension's CreateFilter entry point (mirroring the JavaScript V2
// createFilter() contract) and converts the returned
// map[string]runtime.FilterDefinition into the proto representation. Extensions
// that do not declare CreateFilter produce nil with no error.
func CreateFilter(pkg string, filter *proto.FilterSelection) (map[string]*proto.ExtensionFilter, error) {
	src, err := readExtensionPkgSource(pkg)
	if err != nil {
		return nil, err
	}
	// Only call the extension when it actually declares CreateFilter; otherwise
	// return nil,nil so a missing filter function is not an error (the same way
	// a JS extension that never overrides createFilter yields no filters).
	if !strings.Contains(string(src), "func CreateFilter") {
		return nil, nil
	}

	res, err := callExtension(pkg, "CreateFilter", pkg, filterFromProto(filter))
	if err != nil {
		return nil, err
	}
	return toExtensionFilters(res), nil
}

// Watch returns watch/stream information for a content item. The returned
// *extension.Extension carries the ApiVersion/WatchType used by callers to
// pick a response shape.
//
// The Golang (Scriggo) V2 backend emits exactly two watch shapes from Watch():
// the generic V2 ExtensionWatch (the source/group list, resolved via Mirror),
// or the combined ExtensionAllMirror (carrying the manga/fikushon/bangumi members
// behind @type all). A standalone per-type watch (manga/fikushon/bangumi) is not
// a valid Watch() return for golang extensions; those shapes only appear as
// members of ExtensionAllMirror. toWatchValue routes the result to the right proto
// message based on the concrete script struct.
func Watch(pkg string, url string) (any, *extension.Extension, error) {
	res, err := callExtension(pkg, "Watch", pkg, url)
	if err != nil {
		return nil, nil, err
	}
	meta, err := GetExtensionMeta(pkg)
	if err != nil {
		return nil, nil, err
	}
	// The extension returns the final per-type watch already (just like the JS
	// V1 watch() shape { type, url }). Torrent/magnet links are passed through
	// untouched -- resolving the torrent (fetching metainfo, building the file
	// tree) is the FRONTEND's job, not the host's. See runtime_v1.js and the JS
	// V1 watch() contract.
	return toWatchValue(pkg, res), meta, nil
}

// Mirror resolves the chosen source (the mirror URL the user picked from the
// Watch list) into the final watchable resource. Under the V2 layout Watch()
// only yields the list of mirrors; the actual stream link -- and any torrent
// that needs resolving -- is produced here, mirroring the JavaScript V2 step.
// The returned value is a proto per-type watch (bangumi/manga/fikushon/all)
// whose concrete shape follows the extension's declared @type.
func Mirror(pkg string, url string) (any, error) {
	res, err := callExtension(pkg, "Mirror", pkg, url)
	if err != nil {
		return nil, err
	}
	meta, err := GetExtensionMeta(pkg)
	if err != nil {
		return nil, err
	}
	// Mirror returns the final per-type watch (the JS V1 watch() shape
	// { type, url }). Torrent/magnet links pass through untouched -- the
	// frontend resolves them, exactly like the JS V1 watch() contract.
	return toMirrorValue(res, meta), nil
}
