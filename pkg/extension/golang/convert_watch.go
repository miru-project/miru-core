package golang

import (
	"reflect"

	"github.com/miru-project/miru-core/pkg/extension"
	"github.com/miru-project/miru-core/proto/generate/proto"
)

// ---------------------------------------------------------------------------
// Watch-shape routing and the per-type watch converters.
//
// Watch() and Mirror() are the only endpoints whose return shape is not fixed:
// the concrete proto message depends on the script struct the extension built
// and on the extension's declared @type. The routers below pick the target
// shape; the to*Watch converters fill it in.
// ---------------------------------------------------------------------------

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
func toWatchValue(pkg string, res any) any {
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
		return toAllWatch(pkg, res)
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
				return allWatchOf(meta.Pkg, "bangumi", res)
			}
			return toBangumiWatch(meta.Pkg, res)
		case "ExtensionMangaWatchMirror":
			if meta.WatchType == extension.WatchTypeAll {
				return allWatchOf(meta.Pkg, "manga", res)
			}
			return toMangaWatch(res)
		case "ExtensionFikushonWatchMirror":
			if meta.WatchType == extension.WatchTypeAll {
				return allWatchOf(meta.Pkg, "fikushon", res)
			}
			return toFikushonWatch(res)
		case "ExtensionAllMirror":
			return toAllWatch(meta.Pkg, res)
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
		w := &proto.ExtensionBangumiWatch{Type: cType, Url: url, Headers: headers}
		resolveTorrentForWatch(meta.Pkg, url, cType, w)
		return w
	case extension.WatchTypeManga:
		return &proto.ExtensionMangaWatch{Urls: []string{url}, Headers: headers}
	case extension.WatchTypeFikushon:
		return &proto.ExtensionFikushonWatch{Content: []string{url}}
	case extension.WatchTypeAll:
		// A single URL with no declared sub-shape is ambiguous for "all"; expose
		// it as the bangumi member so the frontend still receives a usable resource.
		w := &proto.ExtensionBangumiWatch{Type: cType, Url: url, Headers: headers}
		resolveTorrentForWatch(meta.Pkg, url, cType, w)
		return &proto.ExtensionAllWatch{Bangumi: w}
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
// tls-client proxy transparently. When the content type is a torrent or magnet
// link, the host resolves it into a downloadable file-tree handle (shared with
// the JavaScript runtime) so the frontend can pick files -- exactly like JS.
func toBangumiWatch(pkg string, v any) *proto.ExtensionBangumiWatch {
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
	resolveTorrentForWatch(pkg, w.Url, w.Type, w)
	return w
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
func toAllWatch(pkg string, v any) *proto.ExtensionAllWatch {
	rv := derefStruct(v)
	if !rv.IsValid() {
		return nil
	}
	return &proto.ExtensionAllWatch{
		Manga:    toMangaWatch(anyField(rv, "Manga")),
		Fikushon: toFikushonWatch(anyField(rv, "Fikushon")),
		Bangumi:  toBangumiWatch(pkg, anyField(rv, "Bangumi")),
	}
}

// allWatchOf wraps a single per-type watch result into an "all" watch, used when
// an @type all extension returns a bare per-type shape (the same struct a
// single-type extension would return). The content is nested into the matching
// member so it is not dropped -- e.g. a torrent/magnet link (a bangumi-shaped
// result) lands in the Bangumi member and the host resolves its file tree the
// same way the JavaScript runtime would.
func allWatchOf(pkg, kind string, res any) *proto.ExtensionAllWatch {
	switch kind {
	case "bangumi":
		return &proto.ExtensionAllWatch{Bangumi: toBangumiWatch(pkg, res)}
	case "manga":
		return &proto.ExtensionAllWatch{Manga: toMangaWatch(res)}
	case "fikushon":
		return &proto.ExtensionAllWatch{Fikushon: toFikushonWatch(res)}
	default:
		return &proto.ExtensionAllWatch{Bangumi: toBangumiWatch(pkg, res)}
	}
}
