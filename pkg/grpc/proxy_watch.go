package grpc

import (
	"github.com/miru-project/miru-core/pkg/network"
	"github.com/miru-project/miru-core/pkg/torrent"
	"github.com/miru-project/miru-core/proto/generate/proto"
)

// miruTLSProfileHeader is a reserved header key an extension may set on a
// resolved watch URL to request that the backend fetch the upstream through the
// browser-impersonating tls-client (the __tls=1&__tlsp=<profile> route). It is
// stripped before the URL is proxied so it never reaches the real upstream.
const miruTLSProfileHeader = "Miru-TLS-Profile"

// proxyWatchURL ensures every resolved stream/mirror URL is fetched through
// miru-core (just like torrents are always resolved server-side). If the URL is
// already a proxy URL, or it is a torrent/magnet link (resolved server-side into
// a file tree rather than streamed), it is returned untouched (no double-wrapping
// and no /proxy wrapping); otherwise it is wrapped with network.BuildProxyURL so
// the player/downloader simply requests it from the backend, which applies the
// mirror headers and optional tls fingerprint server-side.
//
// A TLS profile, when present, is taken from the reserved Miru-TLS-Profile
// header (removed from the forwarded headers) and routed through the
// tls-client-aware parse path.
func proxyWatchURL(url string, headers map[string]string) string {
	if url == "" || network.IsProxyURL(url) || torrent.IsTorrentLink(url) {
		return url
	}
	tlsProfile := ""
	if headers != nil {
		if p, ok := headers[miruTLSProfileHeader]; ok {
			tlsProfile = p
			delete(headers, miruTLSProfileHeader)
		}
	}
	return network.BuildProxyURL(network.ProxyOrigin(), url, headers, tlsProfile)
}

// proxyBangumiWatch applies proxyWatchURL to a bangumi watch result in place.
func proxyBangumiWatch(w *proto.ExtensionBangumiWatch) {
	if w == nil {
		return
	}
	w.Url = proxyWatchURL(w.Url, w.Headers)
}

// proxyMangaWatch applies proxyWatchURL to every page URL of a manga watch
// result in place.
func proxyMangaWatch(w *proto.ExtensionMangaWatch) {
	if w == nil {
		return
	}
	for i, u := range w.Urls {
		w.Urls[i] = proxyWatchURL(u, w.Headers)
	}
}

// proxyFikushonWatch is a no-op: fikushon watches carry prose content rather
// than a stream/mirror URL, so there is nothing to proxy.
func proxyFikushonWatch(w *proto.ExtensionFikushonWatch) {}

// proxyAllWatch applies proxyWatchURL to every stream URL inside an all-watch
// result in place.
func proxyAllWatch(w *proto.ExtensionAllWatch) {
	if w == nil {
		return
	}
	if w.Bangumi != nil {
		proxyBangumiWatch(w.Bangumi)
	}
	if w.Manga != nil {
		proxyMangaWatch(w.Manga)
	}
	if w.Fikushon != nil {
		proxyFikushonWatch(w.Fikushon)
	}
}

// proxyMirrorResponse applies proxyWatchURL to the URL(s) inside a MirrorResponse
// based on its oneof variant. It is a no-op for unknown/empty variants.
func proxyMirrorResponse(resp *proto.MirrorResponse) {
	if resp == nil || resp.Data == nil {
		return
	}
	switch d := resp.Data.(type) {
	case *proto.MirrorResponse_Bangumi:
		proxyBangumiWatch(d.Bangumi)
	case *proto.MirrorResponse_Manga:
		proxyMangaWatch(d.Manga)
	case *proto.MirrorResponse_Fikushon:
		// fikushon carries prose, nothing to proxy.
	case *proto.MirrorResponse_All:
		proxyAllWatch(d.All)
	}
}
