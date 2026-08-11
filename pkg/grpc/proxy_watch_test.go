package grpc

import "testing"

// Torrent/magnet links are resolved server-side into an ExtensionBangumiWatchTorrent
// file tree, so proxyWatchURL must NOT wrap them into a generic /proxy stream URL.
func TestProxyWatchURLTorrentNotProxied(t *testing.T) {
	cases := []string{
		"https://example.com/download/2143519.torrent",
		"https://example.com/files/Some.Show.S01E01.1080p.torrent",
		"magnet:?xt=urn:btih:abcdef1234567890",
	}
	for _, tc := range cases {
		if got := proxyWatchURL(tc, nil); got != tc {
			t.Errorf("proxyWatchURL(%q) = %q, want unchanged (torrent/magnet is a proxy exception)", tc, got)
		}
	}
}

func TestProxyWatchURLPlainStreamProxied(t *testing.T) {
	raw := "https://cdn.example.com/stream/playlist.m3u8"
	got := proxyWatchURL(raw, nil)
	if got == raw {
		t.Errorf("proxyWatchURL(%q) returned unchanged, want a wrapped /proxy URL", raw)
	}
}
