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
// API drift) and the two host primitives an extension is allowed to call
// (Fetch / ProxyURL) are surfaced as package-level vars. Keeping the names in
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
	// ExtensionWatch is stream/watch information for a content item.
	ExtensionWatch = runtime.ExtensionWatch
	// ExtensionMirrorGroup groups alternative mirrors under a title.
	ExtensionMirrorGroup = runtime.ExtensionMirrorGroup
	// ExtensionMirror is a single alternative mirror URL.
	ExtensionMirror = runtime.ExtensionMirror
	// ExtensionMangaWatch is the per-type watch shape for manga extensions.
	ExtensionMangaWatch = runtime.ExtensionMangaWatch
	// ExtensionFikushonWatch is the per-type watch shape for novel/fiction.
	ExtensionFikushonWatch = runtime.ExtensionFikushonWatch
	// ExtensionBangumiWatchSubtitle is a single subtitle track for a bangumi watch.
	ExtensionBangumiWatchSubtitle = runtime.ExtensionBangumiWatchSubtitle
	// ExtensionBangumiWatch is the per-type watch shape for bangumi extensions.
	// When URL is a magnet/torrent link the host resolves it (mirroring the JS
	// handleMediaType behaviour) and fills Torrent; an author may also resolve
	// one explicitly via AddMagnet / AddTorrent and set Torrent themselves.
	ExtensionBangumiWatch = runtime.ExtensionBangumiWatch
	// ExtensionAllWatch bundles manga + fikushon + bangumi behind @type all.
	ExtensionAllWatch = runtime.ExtensionAllWatch
	// TLSConfig configures browser-impersonating (tls-client) requests.
	TLSConfig = runtime.TLSConfig
	// Torrent is the resolved torrent handle attached to a bangumi watch.
	Torrent = runtime.Torrent
)

// Fetch performs a single HTTP request and returns the raw response body, the
// HTTP status code, and an error string (empty on success). When tls is
// non-nil the request is routed through the browser-impersonating tls-client
// (e.g. Profile "chrome_133"); an empty Profile falls back to the library
// default. See the host runtime for the full contract.
var Fetch = runtime.Fetch

// ProxyURL converts a raw media/stream URL into a host-relative proxy path
// the Miru backend fetches server-side on behalf of the client. tlsProfile
// optionally selects a tls-client fingerprint profile.
var ProxyURL = runtime.ProxyURL

// AddMagnet resolves a magnet: link (mirroring the JavaScript handleMediaType)
// and returns the resolved Torrent. The second return value is a non-empty
// error string when resolution fails. title may be empty (the torrent's own
// name is used); pkg is the calling extension's package name.
var AddMagnet = runtime.AddMagnet

// AddTorrent resolves a .torrent file URL (mirroring the JavaScript
// handleMediaType). A relative link is resolved against the extension's
// website origin. See AddMagnet for the return-value contract.
var AddTorrent = runtime.AddTorrent

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
