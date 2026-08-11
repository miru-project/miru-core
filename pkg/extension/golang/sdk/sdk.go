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
	// FilterOption is a single key/label pair inside SelectFilter /
	// MultiSelectFilter options.
	FilterOption = runtime.FilterOption
	// SelectFilter is a single-value filter (one option chosen).
	SelectFilter = runtime.SelectFilter
	// MultiSelectFilter is a multi-value filter (Min..Max options chosen).
	MultiSelectFilter = runtime.MultiSelectFilter
	// RangeFilter is a numeric range filter.
	RangeFilter = runtime.RangeFilter
	// FilterDefinition is the typed union an extension's CreateFilter entry
	// point returns. Exactly one of its Select/MultiSelect/Range fields is set.
	FilterDefinition = runtime.FilterDefinition
	// FilterBuilder builds a SelectFilter fluently.
	SelectFilterBuilder = runtime.SelectFilterBuilder
	// MultiSelectBuilder builds a MultiSelectFilter fluently.
	MultiSelectBuilder = runtime.MultiSelectBuilder
	// RangeBuilder builds a RangeFilter fluently.
	RangeBuilder = runtime.RangeBuilder
	// FilterSelectionBuilder builds the Filter selection sent by the frontend.
	FilterSelectionBuilder = runtime.FilterSelectionBuilder
	// Filter is the strongly-typed filter selection passed to Search /
	// CreateFilter. It is the runtime Filter struct.
	Filter = runtime.Filter
	// FilterSelection is a single filter's selected option keys.
	FilterSelection = runtime.FilterSelection
	// ExtensionSetting is the strongly typed definition of an extension
	// setting, registered with RegisterSetting.
	ExtensionSetting = runtime.ExtensionSetting
	// ExtensionSettingType is the UI control type of an extension setting
	// (input/radio/toggle).
	ExtensionSettingType = runtime.ExtensionSettingType
	// TLSConfig configures browser-impersonating (tls-client) requests.
	// Use it on ExtensionBangumiWatchMirror to let the backend auto-proxy
	// all URLs with the specified TLS fingerprint profile.
	TLSConfig = runtime.TLSConfig
)

// BangumiWatchType is the content type of a bangumi stream/mirror
// (hls/mp4/torrent/magnet). It is the runtime.BangumiWatchType type alias.
type BangumiWatchType = runtime.BangumiWatchType

// SettingType constants select the UI control type of an extension setting.
var (
	// SettingInput is a plain text/value input.
	SettingInput = runtime.SettingInput
	// SettingRadio is a radio group of the setting's options.
	SettingRadio = runtime.SettingRadio
	// SettingToggle is an on/off toggle.
	SettingToggle = runtime.SettingToggle
)

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

// Filter builders. These are the ergonomic constructors an extension's
// CreateFilter entry point uses to return sdk.FilterDefinition values
// without the verbose map[string]FilterOption{...} literal.
var (
	NewSelect      = runtime.NewSelect
	NewMultiSelect = runtime.NewMultiSelect
	NewRange       = runtime.NewRange
	// NewFilterSelection builds a Filter (the frontend's filter selection).
	NewFilterSelection = runtime.NewFilterSelection
)

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

// Filter helpers let extensions read individual filters from the strongly-typed
// sdk.Filter passed to Search / CreateFilter, by the filter name declared in
// CreateFilter. They are standalone functions (Scriggo does not support method
// declarations on user-defined types).
var (
	// HasSelection reports whether the named filter has a non-empty selection.
	HasSelection = runtime.HasSelection
	// FirstSelection returns the first selected value ("" if unset); use for
	// single-select filters (max == 1).
	FirstSelection = runtime.FirstSelection
	// SelectionsOf returns all selected values (nil if unset); use for
	// multi-select filters (max > 1).
	SelectionsOf = runtime.SelectionsOf
)

// RegisterSetting registers a setting definition for this extension package.
// The setting is a strongly typed sdk.ExtensionSetting: it carries the same
// fields as the JavaScript registerSetting({...}) API -- key, title, type,
// value, defaultValue, description and options -- without the untyped map.
var RegisterSetting = runtime.RegisterSetting

// GetSetting reads a previously registered setting value for this package,
// returned as a plain string. An empty string means the setting is unset.
var GetSetting = runtime.GetSetting

// SetSetting writes a setting value for a package. The key must have been
// registered with RegisterSetting first.
var SetSetting = runtime.SetSetting

// GetCookies returns the cookies currently stored for a URL as a slice of
// "name=value" strings, mirroring the JavaScript getCookies().
var GetCookies = runtime.GetCookies

// SetCookies stores the given cookies for a URL, mirroring the JavaScript
// setCookies().
var SetCookies = runtime.SetCookies
