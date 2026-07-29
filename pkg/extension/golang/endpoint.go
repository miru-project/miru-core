package golang

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/miru-project/miru-core/pkg/extension"
	"github.com/miru-project/miru-core/pkg/extension/golang/runtime"
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

// CreateFilter creates filter options for a package.
func CreateFilter(pkg string, filter string) (map[string]*proto.ExtensionFilter, error) {
	return nil, nil
}

// GetExtensionMeta returns the parsed metadata for a package.
func GetExtensionMeta(pkg string) (*extension.Extension, error) {
	return ParseExtensionMetadata(pkg)
}

// readExtensionPkgSource reads the source of an extension package from disk.
func readExtensionPkgSource(pkg string) ([]byte, error) {
	extPath := filepath.Join(ExtensionDir, pkg+".go")
	return os.ReadFile(extPath)
}

// callExtension compiles an extension package and invokes the named function by
// name using the local Scriggo build that supports calling a function by name.
//
// The extension source is compiled as-is: the package declaration, any imports
// and the // ==MiruExtension== metadata header are all valid Go that the
// compiler handles, so there is no need to strip anything. The first return
// value of the called function (index 0) is returned; if the function also
// returns an error (index 1) it is propagated to the caller.
func callExtension(pkg, fn string, args ...any) (any, error) {
	vm := NewScriggoVM(nil)
	src, err := readExtensionPkgSource(pkg)
	if err != nil {
		return nil, err
	}

	prog, err := vm.Compile(pkg+"_"+fn, string(src))
	if err != nil {
		return nil, fmt.Errorf("compile extension %s: %w", pkg, err)
	}

	res, err := prog.program.Call(fn, args...)
	if err != nil {
		return nil, withStackTrace("extension "+pkg+"."+fn, err)
	}
	if len(res) >= 2 {
		if e, ok := res[1].(error); ok && e != nil {
			return nil, e
		}
	}
	if len(res) == 0 {
		return nil, nil
	}
	return res[0], nil
}

// Search searches for content by keyword.
func Search(pkg string, page int, kw string, filter string) ([]*proto.ExtensionListItem, error) {
	res, err := callExtension(pkg, "Search", pkg, kw, page, filter)
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
	return toWatchValue(res), meta, nil
}

// isTorrentLink reports whether a watch URL refers to a torrent resource that
// the host should resolve: a magnet: link or a .torrent file URL.
func isTorrentLink(link string) bool {
	if link == "" {
		return false
	}
	if strings.HasPrefix(link, "magnet:") {
		return true
	}
	return strings.HasSuffix(strings.ToLower(link), ".torrent")
}

// toWatchValue converts a script Watch return value into the appropriate proto
// watch message. The Go (Scriggo) V2 runtime only emits two watch shapes:
//
//   - *ExtensionAllMirror -> proto.ExtensionAllWatch (the @type all bundle)
//   - anything else (incl. the V2 *ExtensionWatch) -> proto.ExtensionWatch
//
// A standalone per-type watch (*ExtensionMangaWatchMirror / *ExtensionFikushonWatchMirror /
// *ExtensionBangumiWatchMirror) is NOT a valid Watch() return for golang extensions;
// those shapes only occur as members of ExtensionAllMirror (built from the
// internal runtime types). The match is by the script type's name.
func toWatchValue(res any) any {
	if res == nil {
		return toWatch(res)
	}
	rv := reflect.ValueOf(res)
	for rv.Kind() == reflect.Ptr || rv.Kind() == reflect.Interface {
		if rv.IsNil() {
			return toWatch(res)
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return toWatch(res)
	}
	switch rv.Type().Name() {
	case "ExtensionAllMirror":
		return toAllWatch(res)
	default:
		return toWatch(res)
	}
}

// toMirrorValue converts a script Mirror() return value into the final proto
// per-type watch. Under the V2 layout the user has already picked a mirror from
// the Watch list, so Mirror() yields the actual stream link (and any torrent that
// needs resolving) rather than another list.
//
// The script may return either:
//   - a per-type watch struct (ExtensionBangumiWatchMirror / ExtensionMangaWatchMirror /
//     ExtensionFikushonWatchMirror / ExtensionAllMirror) -- converted directly, or
//   - a single ExtensionMirror (or a bare URL string) -- the chosen source; we
//     wrap it into the per-type watch dictated by the extension's declared @type.
//
// The concrete output shape follows meta.WatchType exactly, so the gRPC Mirror
// handler can route it into the matching MirrorResponse oneof variant (same as
// the JavaScript V2 runtime).
func toMirrorValue(res any, meta *extension.Extension) any {
	if res == nil {
		return toPerTypeWatch(meta, "", nil)
	}
	rv := reflect.ValueOf(res)
	for rv.Kind() == reflect.Ptr || rv.Kind() == reflect.Interface {
		if rv.IsNil() {
			return toPerTypeWatch(meta, "", nil)
		}
		rv = rv.Elem()
	}
	// A per-type watch struct: hand it to the matching converter.
	if rv.Kind() == reflect.Struct {
		switch rv.Type().Name() {
		case "ExtensionBangumiWatchMirror":
			if meta.WatchType == extension.WatchTypeAll {
				// An "all" extension may return the same per-type shape a
				// single-type extension returns; nest it into the matching
				// all member (e.g. a torrent link lands in Bangumi) so the
				// content is not dropped.
				return allWatchOf("bangumi", res)
			}
			return toBangumiWatch(res)
		case "ExtensionMangaWatchMirror":
			if meta.WatchType == extension.WatchTypeAll {
				return allWatchOf("manga", res)
			}
			return toMangaWatch(res)
		case "ExtensionFikushonWatchMirror":
			if meta.WatchType == extension.WatchTypeAll {
				return allWatchOf("fikushon", res)
			}
			return toFikushonWatch(res)
		case "ExtensionAllMirror":
			return toAllWatch(res)
		}
	}
	// A single ExtensionMirror object: take its URL as the chosen source.
	if rv.Kind() == reflect.Struct && rv.Type().Name() == "ExtensionMirror" {
		return toPerTypeWatch(meta, strField(rv, "URL"), nil)
	}
	// A bare URL string: the chosen source directly.
	if s, ok := res.(string); ok {
		return toPerTypeWatch(meta, s, nil)
	}
	// Fallback: treat as a generic watch (a list of mirrors is not valid here).
	return toWatch(res)
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

// ---------------------------------------------------------------------------
// Reflection-based conversion from script-defined extension return values to
// proto types.
//
// Extensions are written in a Go subset and return plain structs whose types
// are defined inside the compiled program; they are not the generated proto
// types. These converters read the returned values by field name and build the
// corresponding proto values. Field names are matched case-insensitively so
// that the script's "URL" maps onto proto's "Url", and the few structural
// differences between the two type families (for example the script's
// ExtensionDetail.Chapters and ExtensionEpisodeGroup.URLs) are handled
// explicitly below.
// ---------------------------------------------------------------------------

func fieldByName(rv reflect.Value, name string) reflect.Value {
	for rv.Kind() == reflect.Ptr || rv.Kind() == reflect.Interface {
		if rv.IsNil() {
			return reflect.Value{}
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return reflect.Value{}
	}
	lower := strings.ToLower(name)
	t := rv.Type()
	for i := 0; i < t.NumField(); i++ {
		if strings.ToLower(t.Field(i).Name) == lower {
			return rv.Field(i)
		}
	}
	return reflect.Value{}
}

func strField(rv reflect.Value, name string) string {
	f := fieldByName(rv, name)
	if !f.IsValid() || f.Kind() != reflect.String {
		return ""
	}
	return f.String()
}

func anyField(rv reflect.Value, name string) any {
	f := fieldByName(rv, name)
	if !f.IsValid() {
		return nil
	}
	return f.Interface()
}

// mapStrField reads a map[string]string field by name (e.g. a mirror's
// Headers). Templates return nil when the field is absent, so extensions that
// don't set headers simply produce an empty map.
func mapStrField(rv reflect.Value, name string) map[string]string {
	f := fieldByName(rv, name)
	if !f.IsValid() || f.Kind() != reflect.Map {
		return nil
	}
	out := make(map[string]string, f.Len())
	for _, k := range f.MapKeys() {
		if k.Kind() != reflect.String {
			continue
		}
		v := f.MapIndex(k)
		if v.Kind() == reflect.String {
			out[k.String()] = v.String()
		}
	}
	return out
}

func toExtensionListItems(v any) []*proto.ExtensionListItem {
	if v == nil {
		return nil
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Slice {
		return nil
	}
	items := make([]*proto.ExtensionListItem, 0, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		elem := rv.Index(i)
		items = append(items, &proto.ExtensionListItem{
			Title:   strField(elem, "Title"),
			Url:     strField(elem, "URL"),
			Cover:   strField(elem, "Cover"),
			Update:  strField(elem, "Update"),
			Headers: mapStrField(elem, "Headers"),
		})
	}
	return items
}

func toEpisodeGroups(v any) []*proto.ExtensionEpisodeGroup {
	if v == nil {
		return nil
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Slice {
		return nil
	}
	groups := make([]*proto.ExtensionEpisodeGroup, 0, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		elem := rv.Index(i)
		urls := fieldByName(elem, "URLs")
		var episodes []*proto.ExtensionEpisode
		if urls.IsValid() && urls.Kind() == reflect.Slice {
			for j := 0; j < urls.Len(); j++ {
				u := urls.Index(j)
				// Extensions may supply episodes either as bare URL strings
				// (the original model, kept for backwards compatibility with
				// existing extensions such as example) or as structs carrying
				// both a Name and a URL. The latter is what makes episode text
				// appear in the UI; the former yields unlabelled episodes.
				if u.Kind() == reflect.String {
					episodes = append(episodes, &proto.ExtensionEpisode{Url: u.String()})
					continue
				}
				uv := u
				if uv.Kind() == reflect.Interface && !uv.IsNil() {
					uv = uv.Elem()
				}
				if uv.Kind() == reflect.Struct {
					episodes = append(episodes, &proto.ExtensionEpisode{
						Name: strField(uv, "Name"),
						Url:  strField(uv, "URL"),
					})
				}
			}
		}
		groups = append(groups, &proto.ExtensionEpisodeGroup{
			Title: strField(elem, "Title"),
			Urls:  episodes,
		})
	}
	return groups
}

func toDetail(v any) *proto.ExtensionDetail {
	if v == nil {
		return nil
	}
	rv := reflect.ValueOf(v)
	title := strField(rv, "Title")
	cover := strField(rv, "Cover")
	desc := strField(rv, "Desc")
	if desc == "" {
		desc = strField(rv, "Description")
	}
	return &proto.ExtensionDetail{
		Title:    &title,
		Cover:    &cover,
		Desc:     &desc,
		Episodes: toEpisodeGroups(anyField(rv, "Chapters")),
		Headers:  mapStrField(rv, "Headers"),
	}
}

func toMirrorGroups(v any) []*proto.ExtensionMirrorGroup {
	if v == nil {
		return nil
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Slice {
		return nil
	}
	groups := make([]*proto.ExtensionMirrorGroup, 0, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		elem := rv.Index(i)
		groups = append(groups, &proto.ExtensionMirrorGroup{
			Title:   strField(elem, "Title"),
			Mirrors: toMirrors(anyField(elem, "Mirrors")),
		})
	}
	return groups
}

func toMirrors(v any) []*proto.ExtensionMirror {
	if v == nil {
		return nil
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Slice {
		return nil
	}
	mirrors := make([]*proto.ExtensionMirror, 0, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		elem := rv.Index(i)
		mirrors = append(mirrors, &proto.ExtensionMirror{
			Name:    strField(elem, "Name"),
			Url:     strField(elem, "URL"),
			Headers: mapStrField(elem, "Headers"),
		})
	}
	return mirrors
}

// toWatch converts a script ExtensionWatch into the proto equivalent. The script
// type carries its groups directly in the Groups field; when an extension only
// provides a top-level Title/URL (no groups), a single group is synthesised so
// the result is never empty, mirroring the previous behaviour.
func toWatch(v any) *proto.ExtensionWatch {
	if v == nil {
		return nil
	}
	rv := reflect.ValueOf(v)
	groups := toMirrorGroups(anyField(rv, "Groups"))
	if len(groups) == 0 {
		title := strField(rv, "Title")
		url := strField(rv, "URL")
		if title != "" || url != "" {
			groups = []*proto.ExtensionMirrorGroup{
				{
					Title: title,
					Mirrors: []*proto.ExtensionMirror{
						{Name: title, Url: url},
					},
				},
			}
		}
	}
	return &proto.ExtensionWatch{Groups: groups}
}

// strSliceField reads a []string field by name (e.g. a manga watch's URLs).
// Templates return nil when the field is absent, so extensions that don't set
// the slice simply produce an empty slice.
func strSliceField(rv reflect.Value, name string) []string {
	f := fieldByName(rv, name)
	if !f.IsValid() || f.Kind() != reflect.Slice {
		return nil
	}
	out := make([]string, 0, f.Len())
	for i := 0; i < f.Len(); i++ {
		if s := f.Index(i); s.Kind() == reflect.String {
			out = append(out, s.String())
		}
	}
	return out
}

// optionalStrField reads a string field by name and returns it as a *string, or
// nil when the field is absent or empty. proto marks these fields `optional`, so
// an empty value must be omitted rather than sent as "".
func optionalStrField(rv reflect.Value, name string) *string {
	f := fieldByName(rv, name)
	if !f.IsValid() || f.Kind() != reflect.String || f.String() == "" {
		return nil
	}
	s := f.String()
	return &s
}

// int64Field reads an int-like field by name and returns it as int64 (0 when
// absent or non-numeric).
func int64Field(rv reflect.Value, name string) int64 {
	f := fieldByName(rv, name)
	if !f.IsValid() || !f.CanInt() && !f.CanUint() {
		return 0
	}
	return f.Int()
}

// optInt32Field reads an int-like field by name and returns it as a *int32, or
// nil when absent. proto marks these fields `optional`.
func optInt32Field(rv reflect.Value, name string) *int32 {
	f := fieldByName(rv, name)
	if !f.IsValid() || !f.CanInt() && !f.CanUint() {
		return nil
	}
	v := int32(f.Int())
	return &v
}

// optInt64Field reads an int-like field by name and returns it as a *int64, or
// nil when absent. proto marks these fields `optional`.
func optInt64Field(rv reflect.Value, name string) *int64 {
	f := fieldByName(rv, name)
	if !f.IsValid() || !f.CanInt() && !f.CanUint() {
		return nil
	}
	v := f.Int()
	return &v
}

// derefStruct normalises a possibly-pointer/interface value to its underlying
// struct Value, returning an invalid Value when the value is nil or not a
// struct. It is the shared prelude for the per-type watch converters.
func derefStruct(v any) reflect.Value {
	if v == nil {
		return reflect.Value{}
	}
	rv := reflect.ValueOf(v)
	for rv.Kind() == reflect.Ptr || rv.Kind() == reflect.Interface {
		if rv.IsNil() {
			return reflect.Value{}
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return reflect.Value{}
	}
	return rv
}

// toPerTypeWatch builds the proto per-type watch dictated by the extension's
// declared @type from a single resolved URL (the mirror the user picked). It is
// used when a Mirror() script returns a bare URL / ExtensionMirror rather than a
// full watch struct. The concrete shape follows meta.WatchType so the gRPC Mirror
// handler routes it into the matching MirrorResponse oneof variant.
// contentTypeFromURL derives the V2 content type (hls|mp4|torrent|magnet) from a
// resolved watch URL so the per-type watch's Type field always carries a CONTENT
// type, never the extension type. This is the same vocabulary V1 watch() used
// (e.g. a stream URL is "hls", a magnet:/torrent link is "magnet"/"torrent").
func contentTypeFromURL(url string) string {
	switch {
	case strings.HasPrefix(url, "magnet:"):
		return "magnet"
	case isTorrentLink(url):
		return "torrent"
	case strings.HasSuffix(strings.ToLower(url), ".mp4"):
		return "mp4"
	default:
		return "hls"
	}
}

// toPerTypeWatch builds the proto per-type watch dictated by the extension's
// declared @type from a single resolved URL (the mirror the user picked). It is
// used when a Mirror() script returns a bare URL / ExtensionMirror rather than a
// full watch struct. The concrete shape follows meta.WatchType so the gRPC Mirror
// handler routes it into the matching MirrorResponse oneof variant.
//
// The per-type watch's Type field carries the CONTENT type (hls|mp4|torrent|
// magnet) derived from the URL -- never the extension type ("bangumi"). This is
// exactly what V1 watch() produced, and what V2 mirror() must produce so the
// frontend can pick a player/handler uniformly.
func toPerTypeWatch(meta *extension.Extension, url string, headers map[string]string) any {
	cType := contentTypeFromURL(url)
	switch meta.WatchType {
	case extension.WatchTypeBangumi:
		return &proto.ExtensionBangumiWatch{Type: cType, Url: url, Headers: headers}
	case extension.WatchTypeManga:
		return &proto.ExtensionMangaWatch{Urls: []string{url}, Headers: headers}
	case extension.WatchTypeFikushon:
		return &proto.ExtensionFikushonWatch{Content: []string{url}}
	case extension.WatchTypeAll:
		// A single URL with no declared sub-shape is ambiguous for "all"; expose
		// it as the bangumi member so the frontend still receives a usable resource.
		return &proto.ExtensionAllWatch{Bangumi: &proto.ExtensionBangumiWatch{Type: cType, Url: url, Headers: headers}}
	default:
		// Unknown @type: fall back to a generic watch carrying the URL.
		return &proto.ExtensionWatch{Groups: []*proto.ExtensionMirrorGroup{
			{Mirrors: []*proto.ExtensionMirror{{Name: url, Url: url, Headers: headers}}},
		}}
	}
}

// toMangaWatch converts a script ExtensionMangaWatchMirror into the proto equivalent.
func toMangaWatch(v any) *proto.ExtensionMangaWatch {
	rv := derefStruct(v)
	if !rv.IsValid() {
		return nil
	}
	return &proto.ExtensionMangaWatch{
		Urls:    strSliceField(rv, "URLs"),
		Headers: mapStrField(rv, "Headers"),
	}
}

// toFikushonWatch converts a script ExtensionFikushonWatchMirror into the proto
// equivalent.
func toFikushonWatch(v any) *proto.ExtensionFikushonWatch {
	rv := derefStruct(v)
	if !rv.IsValid() {
		return nil
	}
	return &proto.ExtensionFikushonWatch{
		Content:  strSliceField(rv, "Content"),
		Title:    strField(rv, "Title"),
		Subtitle: optionalStrField(rv, "Subtitle"),
	}
}

// toBangumiWatch converts a script ExtensionBangumiWatchMirror into the proto
// equivalent, descending into its subtitles. When the extension provides a
// TLSConfig, every URL in the mirror (main URL + subtitle URLs) is rewritten
// into a host-relative proxy URL so the client fetches through the backend's
// tls-client proxy transparently. Torrent/magnet links are passed through
// untouched -- the frontend resolves them.
func toBangumiWatch(v any) *proto.ExtensionBangumiWatch {
	rv := derefStruct(v)
	if !rv.IsValid() {
		return nil
	}
	w := &proto.ExtensionBangumiWatch{
		Type:       strField(rv, "Type"),
		Url:        strField(rv, "URL"),
		Subtitles:  toBangumiSubtitles(anyField(rv, "Subtitles")),
		Headers:    mapStrField(rv, "Headers"),
		AudioTrack: optionalStrField(rv, "AudioTrack"),
	}
	// Apply TLSConfig-based proxying: when the extension provides a TLSConfig,
	// the backend wraps all URLs (main + subtitles) into proxy URLs so the
	// client/player never sees raw upstream URLs.
	if profile := extractTLSProfile(anyField(rv, "TLSConfig")); profile != "" {
		w.Url = proxyMirrorURL(w.Url, w.Headers, profile)
		for _, sub := range w.Subtitles {
			sub.Url = proxyMirrorURL(sub.Url, nil, profile)
		}
	}
	return w
}

// extractTLSProfile reads the Profile field from a runtime TLSConfig struct
// via reflection. Returns "" when the config is nil or has no profile.
func extractTLSProfile(v any) string {
	rv := derefStruct(v)
	if !rv.IsValid() {
		return ""
	}
	return strField(rv, "Profile")
}

// proxyMirrorURL converts a raw mirror URL into a host-relative proxy URL
// using the runtime's ProxyURL helper. Headers are forwarded only for the
// main media URL; subtitle URLs are proxied without extra headers.
func proxyMirrorURL(target string, headers map[string]string, tlsProfile string) string {
	return runtime.ProxyURL(target, headers, tlsProfile)
}

func toBangumiSubtitles(v any) []*proto.ExtensionBangumiWatchSubtitle {
	if v == nil {
		return nil
	}
	rv := reflect.ValueOf(v)
	// Dereference pointers/interfaces to get the underlying slice.
	for rv.Kind() == reflect.Ptr || rv.Kind() == reflect.Interface {
		if rv.IsNil() {
			return nil
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Slice {
		return nil
	}
	out := make([]*proto.ExtensionBangumiWatchSubtitle, 0, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		elem := derefStruct(rv.Index(i).Interface())
		if !elem.IsValid() {
			continue
		}
		out = append(out, &proto.ExtensionBangumiWatchSubtitle{
			Language: optionalStrField(elem, "Language"),
			Title:    strField(elem, "Title"),
			Url:      strField(elem, "URL"),
		})
	}
	return out
}

// toAllWatch converts a script ExtensionAllMirror into the proto equivalent,
// delegating each member to its own converter.
func toAllWatch(v any) *proto.ExtensionAllWatch {
	rv := derefStruct(v)
	if !rv.IsValid() {
		return nil
	}
	return &proto.ExtensionAllWatch{
		Manga:    toMangaWatch(anyField(rv, "Manga")),
		Fikushon: toFikushonWatch(anyField(rv, "Fikushon")),
		Bangumi:  toBangumiWatch(anyField(rv, "Bangumi")),
	}
}

// allWatchOf wraps a single per-type watch result into an "all" watch, used when
// an @type all extension returns a bare per-type shape (the same struct a
// single-type extension would return). The content is nested into the matching
// member so it is not dropped -- e.g. a torrent/magnet link (a bangumi-shaped
// result) lands in the Bangumi member and the frontend still receives
// { type: "torrent"|"magnet", url } to resolve/stream itself.
func allWatchOf(kind string, res any) *proto.ExtensionAllWatch {
	switch kind {
	case "bangumi":
		return &proto.ExtensionAllWatch{Bangumi: toBangumiWatch(res)}
	case "manga":
		return &proto.ExtensionAllWatch{Manga: toMangaWatch(res)}
	case "fikushon":
		return &proto.ExtensionAllWatch{Fikushon: toFikushonWatch(res)}
	default:
		return &proto.ExtensionAllWatch{Bangumi: toBangumiWatch(res)}
	}
}
