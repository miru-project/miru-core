package golang

import (
	"strings"

	"github.com/miru-project/miru-core/pkg/extension/golang/runtime"
	"github.com/miru-project/miru-core/pkg/torrent"
)

// ---------------------------------------------------------------------------
// Media policy: how a resolved watch URL is classified and whether the host
// rewrites it before handing it to the client. Kept apart from the value
// converters so the "what shape is this?" logic stays separate from the
// "how is this fetched?" logic.
// ---------------------------------------------------------------------------

// contentTypeFromURL derives the V2 content type (hls|mp4|torrent|magnet) from a
// resolved watch URL so the per-type watch's Type field always carries a CONTENT
// type, never the extension type. This is the same vocabulary V1 watch() used
// (e.g. a stream URL is "hls", a magnet:/torrent link is "magnet"/"torrent").
func contentTypeFromURL(url string) string {
	switch {
	case strings.HasPrefix(url, "magnet:"):
		return "magnet"
	case torrent.IsTorrentLink(url):
		return "torrent"
	case strings.HasSuffix(strings.ToLower(url), ".mp4"):
		return "mp4"
	default:
		return "hls"
	}
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
