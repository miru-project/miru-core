package grpc

import (
	"strings"
	"unicode/utf8"

	"github.com/miru-project/miru-core/proto/generate/proto"
)

// sanitizeUTF8 replaces any invalid UTF-8 bytes in s with the Unicode
// replacement character (U+FFFD). This prevents gRPC marshaling errors
// when torrent file names or other user-supplied strings contain
// non-UTF-8 encoded data (e.g. Shift-JIS, GBK filenames).
func sanitizeUTF8(s string) string {
	if utf8.ValidString(s) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 8)
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size <= 1 {
			b.WriteRune('\ufffd')
			i++
		} else {
			b.WriteRune(r)
			i += size
		}
	}
	return b.String()
}

// sanitizeUTF8Ptr sanitizes a *string pointer in place. If the pointer is nil
// it is left untouched; otherwise the pointed-to string is sanitized and the
// pointer is updated.
func sanitizeUTF8Ptr(p *string) {
	if p == nil {
		return
	}
	*p = sanitizeUTF8(*p)
}

// sanitizeNames returns a copy of names where every element is
// sanitized for safe protobuf UTF-8 encoding.
func sanitizeNames(names []string) []string {
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = sanitizeUTF8(n)
	}
	return out
}

// sanitizeTags returns a copy of tags where every element is
// sanitized for safe protobuf UTF-8 encoding.
func sanitizeTags(tags []string) []string {
	if tags == nil {
		return nil
	}
	out := make([]string, len(tags))
	for i, t := range tags {
		out[i] = sanitizeUTF8(t)
	}
	return out
}

// sanitizeHeaders returns a copy of the headers map with all keys and values
// sanitized for safe protobuf UTF-8 encoding.
func sanitizeHeaders(h map[string]string) map[string]string {
	if h == nil {
		return nil
	}
	out := make(map[string]string, len(h))
	for k, v := range h {
		out[sanitizeUTF8(k)] = sanitizeUTF8(v)
	}
	return out
}

// sanitizeBangumiWatch sanitizes all string fields in a bangumi watch result
// in place so it can be safely marshaled to protobuf.
func sanitizeBangumiWatch(w *proto.ExtensionBangumiWatch) {
	if w == nil {
		return
	}
	w.Type = sanitizeUTF8(w.Type)
	w.Url = sanitizeUTF8(w.Url)
	w.Headers = sanitizeHeaders(w.Headers)
	sanitizeUTF8Ptr(w.AudioTrack)
	for _, s := range w.Subtitles {
		sanitizeUTF8Ptr(s.Language)
		s.Title = sanitizeUTF8(s.Title)
		s.Url = sanitizeUTF8(s.Url)
	}
	if w.Torrent != nil {
		sanitizeBangumiWatchTorrent(w.Torrent)
	}
}

// sanitizeBangumiWatchTorrent sanitizes all string fields in a torrent result.
func sanitizeBangumiWatchTorrent(t *proto.ExtensionBangumiWatchTorrent) {
	if t == nil {
		return
	}
	t.InfoHash = sanitizeUTF8(t.InfoHash)
	for i, f := range t.Files {
		t.Files[i] = sanitizeUTF8(f)
	}
	if t.Detail != nil {
		sanitizeBangumiWatchTorrentDetail(t.Detail)
	}
}

// sanitizeBangumiWatchTorrentDetail sanitizes all string fields in torrent detail.
func sanitizeBangumiWatchTorrentDetail(d *proto.ExtensionBangumiWatchTorrentDetail) {
	if d == nil {
		return
	}
	sanitizeUTF8Ptr(d.Pieces)
	sanitizeUTF8Ptr(d.Name)
	sanitizeUTF8Ptr(d.NameUtf8)
	sanitizeUTF8Ptr(d.Source)
	sanitizeBangumiWatchTorrentFileTree(d.FileTree)
}

// sanitizeBangumiWatchTorrentFileTree sanitizes map keys in a file tree.
func sanitizeBangumiWatchTorrentFileTree(t *proto.ExtensionBangumiWatchTorrentFileTree) {
	if t == nil {
		return
	}
	if t.File != nil {
		t.File.PiecesRoot = sanitizeUTF8(t.File.PiecesRoot)
	}
	if t.Dir != nil {
		newDir := make(map[string]*proto.ExtensionBangumiWatchTorrentFileTree, len(t.Dir))
		for k, v := range t.Dir {
			sanitizeBangumiWatchTorrentFileTree(v)
			newDir[sanitizeUTF8(k)] = v
		}
		t.Dir = newDir
	}
}

// sanitizeMangaWatch sanitizes all string fields in a manga watch result in place.
func sanitizeMangaWatch(w *proto.ExtensionMangaWatch) {
	if w == nil {
		return
	}
	for i, u := range w.Urls {
		w.Urls[i] = sanitizeUTF8(u)
	}
	w.Headers = sanitizeHeaders(w.Headers)
}

// sanitizeFikushonWatch sanitizes all string fields in a fikushon watch result in place.
func sanitizeFikushonWatch(w *proto.ExtensionFikushonWatch) {
	if w == nil {
		return
	}
	for i, c := range w.Content {
		w.Content[i] = sanitizeUTF8(c)
	}
	w.Title = sanitizeUTF8(w.Title)
	sanitizeUTF8Ptr(w.Subtitle)
}

// sanitizeAllWatch sanitizes all string fields inside an all-watch result.
func sanitizeAllWatch(w *proto.ExtensionAllWatch) {
	if w == nil {
		return
	}
	sanitizeBangumiWatch(w.Bangumi)
	sanitizeMangaWatch(w.Manga)
	sanitizeFikushonWatch(w.Fikushon)
}

// sanitizeWatchResponse sanitizes all string fields in a WatchResponse so it
// can be safely marshaled to protobuf. This is the main entry point called
// before returning from the Watch() gRPC handler.
func sanitizeWatchResponse(resp *proto.WatchResponse) {
	if resp == nil || resp.Data == nil {
		return
	}
	switch d := resp.Data.(type) {
	case *proto.WatchResponse_Bangumi:
		sanitizeBangumiWatch(d.Bangumi)
	case *proto.WatchResponse_Manga:
		sanitizeMangaWatch(d.Manga)
	case *proto.WatchResponse_Fikushon:
		sanitizeFikushonWatch(d.Fikushon)
	case *proto.WatchResponse_Watch:
		sanitizeExtensionWatch(d.Watch)
	case *proto.WatchResponse_All:
		sanitizeAllWatch(d.All)
	}
}

// sanitizeExtensionWatch sanitizes all string fields in a V2 ExtensionWatch.
func sanitizeExtensionWatch(w *proto.ExtensionWatch) {
	if w == nil {
		return
	}
	for _, g := range w.Groups {
		g.Title = sanitizeUTF8(g.Title)
		for _, m := range g.Mirrors {
			m.Name = sanitizeUTF8(m.Name)
			m.Url = sanitizeUTF8(m.Url)
			m.Headers = sanitizeHeaders(m.Headers)
		}
	}
	sanitizeUTF8Ptr(w.DefaultGroup)
}

// sanitizeMirrorResponse sanitizes all string fields in a MirrorResponse so it
// can be safely marshaled to protobuf.
func sanitizeMirrorResponse(resp *proto.MirrorResponse) {
	if resp == nil || resp.Data == nil {
		return
	}
	switch d := resp.Data.(type) {
	case *proto.MirrorResponse_Bangumi:
		sanitizeBangumiWatch(d.Bangumi)
	case *proto.MirrorResponse_Manga:
		sanitizeMangaWatch(d.Manga)
	case *proto.MirrorResponse_Fikushon:
		sanitizeFikushonWatch(d.Fikushon)
	case *proto.MirrorResponse_All:
		sanitizeAllWatch(d.All)
	}
}
