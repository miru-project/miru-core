package grpc

import (
	"context"
	"strings"
	"testing"

	"github.com/miru-project/miru-core/pkg/extension/golang"
	"github.com/miru-project/miru-core/pkg/network"
	"github.com/miru-project/miru-core/proto/generate/proto"
)

// TestMiruroMirrorHandler reproduces the reported
//
//	expected a map or struct, got "slice"
//
// failure for miruro.org. The gRPC Mirror handler previously forced the flat
// []*proto.ExtensionMirror list returned by the golang extension into a single
// proto.ExtensionBangumiWatch struct (miruro is @type bangumi), which
// mapstructure rejected. The handler now ships the flat list as raw JSON.
//
// The m3u8 URL exercises the extension without any network access: the runtime
// returns it directly as a single stream.
func TestMiruroMirrorHandler(t *testing.T) {
	network.Init()
	golang.ExtensionDir = "/home/noonecare/Documents/miru/extensions"

	s := &MiruCoreServer{}
	const m3u8 = "https://vault-99.owocdn.top/stream/99/01/ec84261e9d8a8ef1a71d03b9335ea03a7e486a1ce78c56fe8e17bb4a7acfe980/uwu.m3u8"

	resp, err := s.Mirror(context.Background(), &proto.MirrorRequest{
		Pkg: "miruro.org",
		Url: m3u8,
	})
	if err != nil {
		t.Fatalf("Mirror handler failed: %v", err)
	}
	if resp == nil || resp.Data == nil {
		t.Fatalf("Mirror handler returned nil response/data")
	}

	raw, ok := resp.Data.(*proto.MirrorResponse_Raw)
	if !ok {
		t.Fatalf("expected raw mirror response, got %T", resp.Data)
	}
	if !strings.Contains(raw.Raw, "uwu.m3u8") {
		t.Fatalf("expected m3u8 in raw mirror response, got: %s", raw.Raw)
	}
	if !strings.Contains(raw.Raw, "Referer") || !strings.Contains(raw.Raw, "User-Agent") {
		t.Fatalf("expected mirror headers (Referer/User-Agent) in raw response, got: %s", raw.Raw)
	}
	t.Logf("Mirror(raw) OK: %s", raw.Raw)
}

// TestMiruroWatchHandler verifies the golang (v2-only) Watch path: the gRPC
// Watch handler must route golang extensions to the generic proto.ExtensionWatch
// (V2) shape regardless of metadata, returning the mirror groups the client
// then feeds into Mirror.
//
// The m3u8 URL exercises the extension without any network access: the runtime
// returns it directly as a single stream.
func TestMiruroWatchHandler(t *testing.T) {
	network.Init()
	golang.ExtensionDir = "/home/noonecare/Documents/miru/extensions"

	s := &MiruCoreServer{}
	const m3u8 = "https://vault-99.owocdn.top/stream/99/01/ec84261e9d8a8ef1a71d03b9335ea03a7e486a1ce78c56fe8e17bb4a7acfe980/uwu.m3u8"

	resp, err := s.Watch(context.Background(), &proto.WatchRequest{
		Pkg: "miruro.org",
		Url: m3u8,
	})
	if err != nil {
		t.Fatalf("Watch handler failed: %v", err)
	}
	if resp == nil || resp.Data == nil {
		t.Fatalf("Watch handler returned nil response/data")
	}

	watch, ok := resp.Data.(*proto.WatchResponse_Watch)
	if !ok {
		t.Fatalf("expected v2 Watch response, got %T", resp.Data)
	}
	if len(watch.Watch.Groups) == 0 || len(watch.Watch.Groups[0].Mirrors) == 0 {
		t.Fatalf("expected at least one mirror group/mirror")
	}
	if !strings.Contains(watch.Watch.Groups[0].Mirrors[0].Url, "uwu.m3u8") {
		t.Fatalf("expected m3u8 in watch mirror url, got: %s", watch.Watch.Groups[0].Mirrors[0].Url)
	}
	h := watch.Watch.Groups[0].Mirrors[0].Headers
	if h["Referer"] == "" || h["User-Agent"] == "" {
		t.Fatalf("expected mirror headers (Referer/User-Agent) in watch response, got: %v", h)
	}
	t.Logf("Watch(v2) OK: groups=%d firstMirror=%s referer=%s", len(watch.Watch.Groups), watch.Watch.Groups[0].Mirrors[0].Url, h["Referer"])
}

// TestMiruroLatestDetailHeaders verifies that the headers the miruro extension
// attaches to list items (Latest covers) and to the detail page (cover/media)
// are passed through to the gRPC response the frontend consumes. It exercises
// the real Scriggo VM load + Live pipe API (network required) and skips when
// the network is unavailable so an offline run is never broken.
func TestMiruroLatestDetailHeaders(t *testing.T) {
	network.Init()
	golang.ExtensionDir = "/home/noonecare/Documents/miru/extensions"

	s := &MiruCoreServer{}
	latest, err := s.Latest(context.Background(), &proto.LatestRequest{Pkg: "miruro.org", Page: 1})
	if err != nil {
		t.Skipf("latest fetch failed (offline?): %v", err)
	}
	if latest == nil || len(latest.Items) == 0 {
		t.Skip("latest returned no items (offline?)")
	}

	var found bool
	for _, it := range latest.Items {
		if it.Headers["Referer"] != "" && it.Headers["User-Agent"] != "" {
			found = true
			t.Logf("latest item %q carries headers (referer=%s, cover=%s)", it.Title, it.Headers["Referer"], it.Cover)
			break
		}
	}
	if !found {
		t.Fatalf("no latest item carried Referer/User-Agent headers: first item headers=%v", latest.Items[0].Headers)
	}

	// Detail: reuse the first item's url (AniList id) to fetch the detail page.
	detail, err := s.Detail(context.Background(), &proto.DetailRequest{Pkg: "miruro.org", Url: latest.Items[0].Url})
	if err != nil {
		t.Skipf("detail fetch failed (offline?): %v", err)
	}
	if detail == nil || detail.Data == nil {
		t.Skip("detail returned no data (offline?)")
	}
	if detail.Data.Headers["Referer"] == "" || detail.Data.Headers["User-Agent"] == "" {
		t.Fatalf("detail (cover/media) missing Referer/User-Agent headers: %v", detail.Data.Headers)
	}
	t.Logf("detail (cover=%s) carries headers (referer=%s)", detail.Data.GetCover(), detail.Data.Headers["Referer"])
}
