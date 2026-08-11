package js

import (
	"fmt"
	"net/url"
	"path/filepath"
	"sort"
	"strings"

	"github.com/miru-project/miru-core/pkg/extension"
	"github.com/miru-project/miru-core/pkg/torrent"
	"github.com/miru-project/miru-core/proto/generate/proto"
)

func GetExtensionMeta(pkg string) (*Ext, error) {
	api, err := getPkgFromCache(pkg)
	if err != nil {
		return nil, err
	}
	return api.Ext, nil
}

// Extension latest should contain V1 and V2 api
func Latest[T any](pkg string, page int) ([]*T, error) {
	api, e := getPkgFromCache(pkg)
	if e != nil {
		return nil, e
	}
	res, err := api.asyncCallBack(api, pkg, fmt.Sprintf(api.latestEval, page))
	if err != nil {
		return nil, err
	}
	return extension.UnmarshalList[T](res)
}

// Extension search should contain V1 and V2 api
func Search[T proto.ExtensionListItem](pkg string, page int, kw string, filter *proto.FilterSelection) ([]*T, error) {
	api, e := getPkgFromCache(pkg)
	if e != nil {
		return nil, e
	}
	res, err := api.asyncCallBack(api, pkg, fmt.Sprintf(api.searchEval, kw, page, filterToJsLiteral(filter)))
	if err != nil {
		return nil, err
	}
	return extension.UnmarshalList[T](res)
}

// Extension watch should contain V1 and V2 api.
//
// The two API versions differ in what watch() returns:
//
//   - V1 (JS): watch() returns the FINAL watchable resource directly (a single
//     per-type watch object whose concrete shape follows the extension's
//     declared @type, e.g. a stream URL for bangumi or a page list for manga).
//     No mirror step exists on V1. Any magnet:/torrent link is resolved by
//     handleMediaType into a downloadable handle.
//
//   - V2 (JS, and the Go/Scriggo runtime): watch() returns a LIST OF MIRRORS
//     (proto.ExtensionWatch carrying one or more groups of mirror URLs), NOT
//     the final link. The chosen mirror is then resolved to the final per-type
//     watch by a separate call to Mirror(). This keeps JS V2 and Go V2 on the
//     same contract so the frontend drives both identically.
func Watch(pkg string, watchLink string) (any, *extension.Extension, error) {
	api, e := getPkgFromCache(pkg)
	if e != nil {
		return nil, nil, e
	}

	o, e := api.asyncCallBack(api, pkg, fmt.Sprintf(api.watchEval, watchLink))
	if e != nil {
		return nil, nil, e
	}

	// V2 returns a mirror list (proto.ExtensionWatch); the frontend picks a
	// mirror and calls Mirror() to get the final link. V1 returns the link
	// directly, so it skips the mirror step and only resolves torrents.
	if api.Ext.ApiVersion == "2" {
		return toJSV2Watch(o), api.Ext, nil
	}

	o, e = handleMediaType(api, pkg, o)
	if e != nil {
		return nil, nil, e
	}

	return o, api.Ext, nil
}

func Detail[T proto.ExtensionDetail](pkg string, url string) (*T, error) {
	api, e := getPkgFromCache(pkg)
	if e != nil {
		return nil, e
	}
	res, err := api.asyncCallBack(api, pkg, fmt.Sprintf(api.detailEval, url))
	if err != nil {
		return nil, err
	}
	return extension.Unmarshal[T](res)
}

// Mirror resolves the chosen mirror (the URL the user picked from the Watch
// list) into the final watchable resource. This is the V2 step that follows
// Watch(): watch() only yields the list of mirrors and Mirror() produces the
// actual per-type watch. The V1 runtime has no mirror step, so a V1 api is
// rejected here.
//
// The script returns the final per-type watch object (e.g. { type, url, headers }
// for bangumi). For bangumi extensions, handleMediaType resolves any
// magnet:/torrent mirror link into a downloadable handle and attaches it as
// `torrent`, exactly like Watch.
func Mirror(pkg string, watchUrl string) (any, error) {
	api, e := getPkgFromCache(pkg)
	if e != nil {
		return "", e
	}
	if api.Ext.ApiVersion != "2" {
		return nil, fmt.Errorf("mirror is only supported on the V2 extension API; package %q uses API %q", pkg, api.Ext.ApiVersion)
	}
	res, err := api.asyncCallBack(api, pkg, fmt.Sprintf(api.mirrorEval, watchUrl))
	if err != nil {
		return "", err
	}
	// For bangumi extensions, resolve any magnet:/torrent mirror link into a
	// downloadable handle and attach it as `torrent`, exactly like Watch.
	return handleMediaType(api, pkg, res)
}

// Unmarshal is retained as a package-level helper (delegating to the shared
// runtime-agnostic implementation) for backwards compatibility with callers
// inside this package and tests.
func Unmarshal[T any](input any) (*T, error) {
	return extension.Unmarshal[T](input)
}

// UnmarshalList is the list variant of Unmarshal.
func UnmarshalList[T any](input any) ([]*T, error) {
	return extension.UnmarshalList[T](input)
}

func CreateFilter(pkg string, filter *proto.FilterSelection) (map[string]*proto.ExtensionFilter, error) {
	api, e := getPkgFromCache(pkg)
	if e != nil {
		return nil, e
	}
	filterLiteral := filterToJsLiteral(filter)
	if filterLiteral == "null" {
		filterLiteral = ""
	}
	res, err := api.asyncCallBack(api, pkg, fmt.Sprintf(api.createFilterEval, filterLiteral))
	if err != nil {
		return nil, err
	}
	decoded, err := Unmarshal[map[string]*proto.ExtensionFilter](res)
	if err != nil {
		return nil, err
	}
	return *decoded, nil
}

// handleMediaType resolves magnet:/torrent links embedded in a watch (v1) or
// mirror (v2) payload for bangumi extensions, mirroring the JavaScript runtime's
// handleMediaType behaviour and the Golang/Scriggo extension logic: it fetches the
// JSON the extension produced, parses it, resolves the torrent into a downloadable
// handle, and attaches it as `torrent` so the frontend can read the file tree and
// decide which files to download.
//
// The per-media `type` field is an enum limited to "magnet" / "torrent" -- any
// other value (mp4, hls, ...) is passed through untouched. The resolved torrent
// is shaped as *proto.ExtensionBangumiWatchTorrent so it round-trips through the
// shared mapstructure Unmarshal used by the gRPC Watch path and matches the
// Golang runtime's output exactly.
func handleMediaType(api *ExtApi, pkg string, o any) (any, error) {
	if api.Ext.WatchType != extension.WatchTypeBangumi {
		return o, nil
	}

	// A mirror (v2) may return an array of mirror objects; a watch (v1) a
	// single object. Process whichever shape we receive.
	if list, ok := o.([]any); ok {
		for _, item := range list {
			if m, ok := item.(map[string]any); ok {
				if _, err := resolveMediaType(api, pkg, m); err != nil {
					return nil, err
				}
			}
		}
		return o, nil
	}

	if obj, ok := o.(map[string]any); ok {
		return resolveMediaType(api, pkg, obj)
	}
	return o, nil
}

// resolveMediaType resolves the torrent on a single watch/mirror object in place
// and returns it. It is the faithful port of the JavaScript handleMediaType
// switch: only the "magnet" and "torrent" type values trigger resolution.
func resolveMediaType(api *ExtApi, pkg string, obj map[string]any) (any, error) {
	vidType, ok := obj["type"].(string)
	if !ok {
		return obj, nil
	}

	switch vidType {
	case "magnet":
		urlInterface, ok := obj["url"]
		if !ok {
			return obj, nil
		}
		link, ok := urlInterface.(string)
		if !ok {
			return obj, nil
		}
		// Share the single torrent/magnet resolver with the Go/Scriggo runtime.
		t, e := torrent.Resolve(link, pkg)
		if e != nil {
			return nil, e
		}
		obj["torrent"] = t
		return obj, nil

	case "torrent":
		urlInterface, ok := obj["url"]
		if !ok {
			return obj, nil
		}
		link, ok := urlInterface.(string)
		if !ok {
			return obj, nil
		}
		if oPath, _ := url.Parse(link); !oPath.IsAbs() {
			web, _ := url.Parse(api.Ext.Website)
			web.Path = filepath.Join(web.Path, link)
			link = web.String()
		}
		// Share the single torrent/magnet resolver with the Go/Scriggo runtime.
		t, e := torrent.Resolve(link, pkg)
		if e != nil {
			return nil, e
		}
		obj["torrent"] = t
		return obj, nil

	default:
		return obj, nil
	}
}

// toJSV2Watch converts a JavaScript V2 watch() return value into the proto
// ExtensionWatch shape (the source/group list of mirrors). The V2 watch()
// contract mirrors the Go/Scriggo V2 runtime: it returns a list of mirrors,
// NOT the final link. The frontend picks a mirror and calls Mirror() to
// resolve it.
//
// The accepted script shapes are:
//
//   - { groups: [{ title, mirrors: [{ name, url, headers }] }] }   (array form)
//   - { groups: { "Server 1": [{ name, url, headers }], ... } }    (object form,
//     keyed by group title, exactly as the example extension builds it)
//
// When an extension supplies neither groups structure and only a bare
// top-level { name, url } (or an array of such), a single synthetic group is
// produced so the result is never empty -- mirroring the Go toWatch fallback.
func toJSV2Watch(raw any) *proto.ExtensionWatch {
	if raw == nil {
		return &proto.ExtensionWatch{}
	}

	obj, ok := raw.(map[string]any)
	if !ok {
		// Not an object: try to treat a bare array as a single group of mirrors.
		if list, ok := raw.([]any); ok {
			if mirrors := toJSV2Mirrors(list); len(mirrors) > 0 {
				return &proto.ExtensionWatch{Groups: []*proto.ExtensionMirrorGroup{
					{Mirrors: mirrors},
				}}
			}
		}
		return &proto.ExtensionWatch{}
	}

	// groups may be either an array of {title, mirrors} or an object keyed by
	// group title.
	if groups := toJSV2Groups(obj["groups"]); len(groups) > 0 {
		return &proto.ExtensionWatch{Groups: groups}
	}

	// Fallback: a single bare mirror object at the top level.
	if name, _ := obj["name"].(string); name != "" || obj["url"] != nil {
		return &proto.ExtensionWatch{Groups: []*proto.ExtensionMirrorGroup{
			{Mirrors: []*proto.ExtensionMirror{toJSV2Mirror(obj)}},
		}}
	}

	return &proto.ExtensionWatch{}
}

// toJSV2Groups converts the `groups` field of a V2 watch() into proto mirror
// groups. It accepts both the array form ([{title, mirrors}]) and the object
// form ({ "Title": [mirrors] }) used by the example extension.
func toJSV2Groups(groups any) []*proto.ExtensionMirrorGroup {
	switch g := groups.(type) {
	case []any:
		out := make([]*proto.ExtensionMirrorGroup, 0, len(g))
		for _, item := range g {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			title, _ := m["title"].(string)
			out = append(out, &proto.ExtensionMirrorGroup{
				Title:   title,
				Mirrors: toJSV2Mirrors(m["mirrors"]),
			})
		}
		return out
	case map[string]any:
		out := make([]*proto.ExtensionMirrorGroup, 0, len(g))
		// Preserve insertion order is not guaranteed by Go maps, but the
		// frontend renders groups in slice order; sort keys for determinism.
		keys := make([]string, 0, len(g))
		for k := range g {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, title := range keys {
			out = append(out, &proto.ExtensionMirrorGroup{
				Title:   title,
				Mirrors: toJSV2Mirrors(g[title]),
			})
		}
		return out
	default:
		return nil
	}
}

// toJSV2Mirrors converts a raw array of mirror objects into proto mirrors.
func toJSV2Mirrors(raw any) []*proto.ExtensionMirror {
	list, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]*proto.ExtensionMirror, 0, len(list))
	for _, item := range list {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, toJSV2Mirror(m))
	}
	return out
}

// toJSV2Mirror converts a single raw mirror object into a proto mirror.
func toJSV2Mirror(m map[string]any) *proto.ExtensionMirror {
	name, _ := m["name"].(string)
	url, _ := m["url"].(string)
	return &proto.ExtensionMirror{
		Name:    name,
		Url:     url,
		Headers: toStringMap(m["headers"]),
	}
}

// toStringMap coerces a raw map (string keys, any values) into a
// map[string]string suitable for proto Headers. Non-string values are rendered
// with fmt.Sprintf so headers like "Connection": keep-alive survive intact.
func toStringMap(raw any) map[string]string {
	src, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	out := make(map[string]string, len(src))
	for k, v := range src {
		if s, ok := v.(string); ok {
			out[k] = s
		} else {
			out[k] = fmt.Sprintf("%v", v)
		}
	}
	return out
}
// filterToJsLiteral converts a proto.FilterSelection into a JavaScript object
// literal string that can be embedded in a goja eval. The shape mirrors
// what the JS extension runtime already expects:
//
//	{ "<filter name>": "<single value>" | ["<val1>", "<val2>", ...] }
//
// A nil or empty proto yields the string "null" (JS null literal).
// This is the ONLY place a string appears in the filter pipeline, and it is
// deliberate: goja needs a JS object literal, and this function generates one
// directly from the proto without any intermediate JSON string parsing.
func filterToJsLiteral(p *proto.FilterSelection) string {
	if p == nil || len(p.Selections) == 0 {
		return "null"
	}
	var buf strings.Builder

	buf.WriteByte('{')
	first := true
	for name, sv := range p.Selections {
		if sv == nil {
			continue
		}
		vals := sv.Values
		if !first {
			buf.WriteByte(',')
		}
		first = false
		fmt.Fprintf(&buf, "%q:", name)
		if len(vals) == 0 {
			buf.WriteString(`""`)
		} else if len(vals) == 1 {
			fmt.Fprintf(&buf, "%q", vals[0])
		} else {
			buf.WriteByte('[')
			for i, v := range vals {
				if i > 0 {
					buf.WriteByte(',')
				}
				fmt.Fprintf(&buf, "%q", v)
			}
			buf.WriteByte(']')
		}
	}
	buf.WriteByte('}')
	return buf.String()
}
