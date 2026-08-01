// Package sdk is the public, importable API surface that Miru Go (Scriggo)
// extension authors use. It is the "pkg endpoint" a freshly-created Go project
// `go get`s and imports to write an extension, e.g.
//
//	go get github.com/miru-project/miru-core/pkg/extension/golang/sdk
//
// and then in an extension source file:
//
//	import sdk "github.com/miru-project/miru-core/pkg/extension/golang/sdk"
//
//	sdk.ExtensionListItem{Title: "...", URL: "..."}
//
// It is a thin, curated re-export of the host implementation in the internal
// `runtime` package: the model types are aliased (single source of truth, no
// API drift) and the host primitive an extension is allowed to call
// (Fetch) is surfaced as a package-level var. Keeping the name in
// `sdk` avoids the `runtime` identifier clashing with the Go standard library
// `runtime` package, which is exactly why callers previously had to alias the
// internal package as `rt`.
//
// Extension entry points (Latest / Search / Detail / Watch / Mirror) are
// declared by the author inside their own extension package, returning these
// types; the host compiles the extension with Scriggo and calls the functions
// by name.
//
// # Load hook
//
// An extension may additionally declare a Load entry point:
//
//	func Load() {
//	    // runs once at startup (mirrors the JavaScript runtime's load())
//	}
//
// Load takes no arguments, exactly like the JavaScript runtime's load() hook.
// It is invoked by the host exactly once, when the extension is first loaded,
// and is the place to do one-time setup. Because every request (Search /
// Latest / ...) compiles and runs a FRESH Scriggo VM, any state Load (or any
// other function) wants to share across calls must be stored through SaveCache
// / GetCache -- keyed by the extension's own package name, which the author
// already knows -- see below.
package sdk

import "github.com/miru-project/miru-core/pkg/extension/golang/runtime"

// Model types returned by an extension's entry points. They are aliases of the
// host runtime types so values cross the Scriggo / host boundary cleanly.
type (
	// ExtensionListItem is a single search / latest result row.
	ExtensionListItem = runtime.ExtensionListItem
	// ExtensionDetail is the full detail page for a content item.
	ExtensionDetail = runtime.ExtensionDetail
	// ExtensionEpisodeGroup groups a set of episode URLs under a title.
	ExtensionEpisodeGroup = runtime.ExtensionEpisodeGroup
	// ExtensionWatch is stream/watch information for a content item. It is the
	// only standalone watch shape a Go (Scriggo) V2 extension may return from
	// Watch(); the resolved stream is fetched via Mirror().
	ExtensionWatch = runtime.ExtensionWatch
	// ExtensionMirrorGroup groups alternative mirrors under a title.
	ExtensionMirrorGroup = runtime.ExtensionMirrorGroup
	// ExtensionMirror is a single alternative mirror URL.
	ExtensionMirror = runtime.ExtensionMirror
	// ExtensionBangumiWatchMirrorSubtitle is a single subtitle track for a bangumi mirror.
	ExtensionBangumiWatchMirrorSubtitle = runtime.ExtensionBangumiWatchMirrorSubtitle
	// ExtensionBangumiWatchMirror is the per-type mirror shape for bangumi
	// (anime) video extensions. A Go (Scriggo) V2 extension that declares
	// @type bangumi returns this from its Mirror() entry point.
	ExtensionBangumiWatchMirror = runtime.ExtensionBangumiWatchMirror
	// ExtensionMangaWatchMirror is the per-type mirror shape for manga
	// extensions. A Go (Scriggo) V2 extension that declares @type manga returns
	// this from its Mirror() entry point.
	ExtensionMangaWatchMirror = runtime.ExtensionMangaWatchMirror
	// ExtensionFikushonWatchMirror is the per-type mirror shape for novel/
	// fiction (fikushon) extensions. A Go (Scriggo) V2 extension that declares
	// @type fikushon returns this from its Mirror() entry point.
	ExtensionFikushonWatchMirror = runtime.ExtensionFikushonWatchMirror
	// ExtensionAllMirror bundles manga + fikushon + bangumi behind @type all. It
	// is the only other shape a Go (Scriggo) V2 extension may return from
	// Watch(). Its members use the per-type runtime types, which are NOT exposed
	// as standalone Watch() return values in the V2 runtime.
	ExtensionAllMirror = runtime.ExtensionAllMirror
	// TLSConfig configures browser-impersonating (tls-client) requests.
	// Use it on ExtensionBangumiWatchMirror to let the backend auto-proxy
	// all URLs with the specified TLS fingerprint profile.
	TLSConfig = runtime.TLSConfig
)

// BangumiWatchType is the content type of a bangumi stream/mirror
// (hls/mp4/torrent/magnet). It is the runtime.BangumiWatchType type alias.
type BangumiWatchType = runtime.BangumiWatchType

// Content-type constants for a bangumi stream/mirror. These mirror what V1
// watch() and V2 mirror() emit as the per-type watch "type" field, and the
// dart ExtensionWatchBangumiType enum (hls/mp4/torrent/magnet).
var (
	HLS     = runtime.HLS
	MP4     = runtime.MP4
	Magnet  = runtime.Magnet
	Torrent = runtime.Torrent
)

// Fetch performs a single HTTP request and returns the raw response body, the
// HTTP status code, and an error string (empty on success). When tls is
// non-nil the request is routed through the browser-impersonating tls-client
// (e.g. Profile "chrome_133"); an empty Profile falls back to the library
// default. See the host runtime for the full contract.
var Fetch = runtime.Fetch

// SaveCache stores a cross-function variable for this package. It is the Go
// counterpart of the JavaScript Miru.saveCache. The store is keyed by package
// name then variable key (matching the JavaScript layout). Values are plain Go
// values (strings, numbers, slices, maps, structs); avoid storing a value that
// would break the Scriggo VM when passed back into a function call -- e.g. a
// function value or a channel tied to a goroutine that has exited.
var SaveCache = runtime.SaveCache

// GetCache reads a cross-function variable previously stored with SaveCache.
// The second return value reports whether the key was present.
var GetCache = runtime.GetCache
