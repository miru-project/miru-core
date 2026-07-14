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
	// TLSConfig configures browser-impersonating (tls-client) requests.
	TLSConfig = runtime.TLSConfig
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
