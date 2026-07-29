package network

import (
	"strings"
	"testing"
)

func TestResolveProxyTarget(t *testing.T) {
	target := "https://example.com/stream/file-16-f2-v1-a1.xls"
	proxy := BuildProxyURL("http://127.0.0.1:3000", target, map[string]string{"Referer": "example.com"}, "chrome_110")

	got, isProxy := ResolveProxyTarget(proxy)
	if !isProxy {
		t.Fatalf("expected isProxy=true for %q", proxy)
	}
	if got != target {
		t.Fatalf("ResolveProxyTarget = %q; want %q", got, target)
	}

	// Non-proxy URL returns unchanged, isProxy=false.
	raw := "https://example.com/direct.mp4"
	got2, isProxy2 := ResolveProxyTarget(raw)
	if isProxy2 || got2 != raw {
		t.Fatalf("ResolveProxyTarget(%q) = (%q,%v); want (%q,false)", raw, got2, isProxy2, raw)
	}
}

func TestProxyURLTargetName(t *testing.T) {
	// The buggy behaviour: path.Base(proxyURL) returned the placeholder name +
	// the whole query string. ProxyURLTargetName must resolve the REAL target.
	// Base64 encoded: https://hls.example.org/stream/file-16-f2-v1-a1.xls
	proxy := "http://127.0.0.1:3000/proxy/file-16-f2-v1-a1.xls?__mh=eyJ9&__tls=1&__tlsp=chrome_110&__u=" + "aHR0cHM6Ly9obHMuZXhhbXBsZS5vcmcvYXBwL3N0cmVhbS9maWxlLTE2LWYyLXYxLWExLnhscw"
	name := ProxyURLTargetName(proxy)
	if name != "file-16-f2-v1-a1.xls" {
		t.Fatalf("ProxyURLTargetName = %q; want %q", name, "file-16-f2-v1-a1.xls")
	}
	if strings.Contains(name, "?") || strings.Contains(name, "__u") {
		t.Fatalf("ProxyURLTargetName returned a garbled name: %q", name)
	}

	// Direct (non-proxy) URL still works.
	if got := ProxyURLTargetName("https://example.com/a/b/video.mp4"); got != "video.mp4" {
		t.Fatalf("ProxyURLTargetName(direct) = %q; want video.mp4", got)
	}
}

func TestIsProxyURL(t *testing.T) {
	if !IsProxyURL("http://127.0.0.1:3000/proxy/x?__u=abc") {
		t.Fatal("expected true for proxy url")
	}
	if IsProxyURL("https://example.com/a.mp4") {
		t.Fatal("expected false for direct url")
	}
}

func TestBuildProxyURLRoundTrip(t *testing.T) {
	target := "https://cdn.example.com/path/seg-01.ts"
	origin := ProxyOrigin()
	proxy := BuildProxyURL(origin, target, map[string]string{}, "")
	if !strings.Contains(proxy, "__u=") {
		t.Fatalf("proxy url missing __u param: %q", proxy)
	}
	got, _ := ResolveProxyTarget(proxy)
	if got != target {
		t.Fatalf("round trip = %q; want %q", got, target)
	}
}
