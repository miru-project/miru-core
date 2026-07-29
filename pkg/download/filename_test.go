package download

import (
	"testing"

	"github.com/miru-project/miru-core/pkg/network"
)

func TestInferMediaTypeFromURL(t *testing.T) {
	cases := []struct {
		url  string
		want string
	}{
		// Direct URLs.
		{"https://example.com/a/stream.m3u8", "hls"},
		{"https://example.com/a/video.mp4", "mp4"},
		{"https://example.com/a/file.torrent", "torrent"},
		{"magnet:?xt=urn:btih:abc", "torrent"},
		// Proxied URLs: extension lives in the real target encoded in __u.
		{"http://127.0.0.1:3000/proxy/name?__u=" + "aHR0cHM6Ly9leGFtcGxlLmNvbS9hL3N0cmVhbS5tM3U4", "hls"},
		{"http://127.0.0.1:3000/proxy/name?__u=" + "aHR0cHM6Ly9leGFtcGxlLmNvbS9hL3ZpZGVvLm1wNA", "mp4"},
		// Unknown.
		{"https://example.com/a/unknown.xyz", ""},
	}
	for _, c := range cases {
		if got := inferMediaTypeFromURL(c.url); got != c.want {
			t.Errorf("inferMediaTypeFromURL(%q) = %q; want %q", c.url, got, c.want)
		}
	}
}

func TestProxyURLFileNameUsed(t *testing.T) {
	// Regression: a proxied download URL must not produce a file name containing
	// the proxy query string. We mirror the downloadMp4 naming here.
	proxy := "http://127.0.0.1:3000/proxy/file-16-f2-v1-a1.xls?__mh=eyJ9&__tls=1&__tlsp=chrome_110&__u=" + "aHR0cHM6Ly9obHMuYW5pZGIuYXBwL3N0cmVhbS9maWxlLTE2LWYyLXYxLWExLnhscw"
	name := network.ProxyURLTargetName(proxy)
	if name != "file-16-f2-v1-a1.xls" {
		t.Fatalf("expected original file name, got %q", name)
	}
}
