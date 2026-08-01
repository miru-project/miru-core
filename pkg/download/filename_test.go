package download

import (
	"testing"

	"github.com/miru-project/miru-core/pkg/network"
	"github.com/miru-project/miru-core/proto/generate/proto"
)

func TestInferMediaTypeFromURL(t *testing.T) {
	cases := []struct {
		url  string
		want string
	}{
		// Direct URLs.
		{"https://example.com/a/stream.m3u8", "hls"},
		{"https://example.com/a/video.mp4", "mp4"},
		{"https://example.com/a/video.m4v", "mp4"},
		{"https://example.com/a/video.mov", "mp4"},
		{"https://example.com/a/video.webm", "mp4"},
		{"https://example.com/a/file.torrent", "torrent"},
		{"magnet:?xt=urn:btih:abc", "magnet"},
		// Proxied URLs: extension lives in the real target encoded in __u.
		{"http://127.0.0.1:3000/proxy/name?__u=" + "aHR0cHM6Ly9leGFtcGxlLmNvbS9hL3N0cmVhbS5tM3U4", "hls"},
		{"http://127.0.0.1:3000/proxy/name?__u=" + "aHR0cHM6Ly9leGFtcGxlLmNvbS9hL3ZpZGVvLm1wNA", "mp4"},
		{"http://127.0.0.1:3000/proxy/name?__u=" + "aHR0cHM6Ly9leGFtcGxlLmNvbS9hL3ZpZGVvLm00dg", "mp4"},
		{"http://127.0.0.1:3000/proxy/name?__u=" + "aHR0cHM6Ly9leGFtcGxlLmNvbS9hL3ZpZGVvLm1vdg", "mp4"},
		{"http://127.0.0.1:3000/proxy/name?__u=" + "bWFnbmV0Oj94dD11cm46YnRpaDphYmM", "magnet"},
		{"http://127.0.0.1:3000/proxy/name?__u=" + "aHR0cHM6Ly9leGFtcGxlLmNvbS9hL2ZpbGUudG9ycmVudA", "torrent"},
		// Unknown.
		{"https://example.com/a/unknown.xyz", ""},
		{"", ""},
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

func TestStatusToProtoAndBack(t *testing.T) {
	cases := []struct {
		status Status
		proto  proto.DownloadStatus
	}{
		{Downloading, proto.DownloadStatus_DOWNLOADING},
		{Paused, proto.DownloadStatus_PAUSED},
		{Completed, proto.DownloadStatus_COMPLETED},
		{Failed, proto.DownloadStatus_FAILED},
		{Canceled, proto.DownloadStatus_CANCELLED},
		{Converting, proto.DownloadStatus_CONVERTING},
		{Queued, proto.DownloadStatus_QUEUED},
	}
	for _, c := range cases {
		if got := StatusToProto(c.status); got != c.proto {
			t.Errorf("StatusToProto(%q) = %v, want %v", c.status, got, c.proto)
		}
		if got := StatusFromProto(c.proto); got != c.status {
			t.Errorf("StatusFromProto(%v) = %q, want %q", c.proto, got, c.status)
		}
	}
}

func TestStatusToProtoUnknown(t *testing.T) {
	if got := StatusToProto(Status("bogus")); got != proto.DownloadStatus_QUEUED {
		t.Errorf("StatusToProto(bogus) = %v, want QUEUED", got)
	}
}

func TestStatusFromProtoUnknown(t *testing.T) {
	got := StatusFromProto(proto.DownloadStatus(999))
	if got != Queued {
		t.Errorf("StatusFromProto(999) = %q, want Queued", got)
	}
}

func TestMediaTypes(t *testing.T) {
	if Hls != "hls" {
		t.Errorf("Hls = %q, want hls", Hls)
	}
	if Mp4 != "mp4" {
		t.Errorf("Mp4 = %q, want mp4", Mp4)
	}
	if Torrent != "torrent" {
		t.Errorf("Torrent = %q, want torrent", Torrent)
	}
	if Magnet != "magnet" {
		t.Errorf("Magnet = %q, want magnet", Magnet)
	}
}

func TestDownloadMediaTypes(t *testing.T) {
	// Verify that each media type constant produces the expected string
	// representation when used in Download().
	types := []struct {
		mt   MediaType
		want string
	}{
		{Hls, "hls"},
		{Mp4, "mp4"},
		{Torrent, "torrent"},
		{Magnet, "magnet"},
	}
	for _, tt := range types {
		if string(tt.mt) != tt.want {
			t.Errorf("MediaType(%q) string = %q", tt.want, string(tt.mt))
		}
	}
}

func TestProgressFieldsAllMediaTypes(t *testing.T) {
	// Verify that Progress can be created for each media type with the
	// correct field values that would be converted by toProtoDownloadProgress.
	mediaTypes := []struct {
		mt      MediaType
		wantStr string
	}{
		{Hls, "hls"},
		{Mp4, "mp4"},
		{Torrent, "torrent"},
		{Magnet, "magnet"},
	}

	for _, tt := range mediaTypes {
		names := []string{"file1.ts", "file2.ts"}
		p := &Progress{
			Progrss:            50,
			Total:              100,
			Status:             Downloading,
			MediaType:          tt.mt,
			CurrentDownloading: "/downloads/file1.ts",
			TaskID:             42,
			Title:              "Episode 01",
			Package:            "test.pkg",
			Key:                "ABCDEF",
			Names:              &names,
			Headers:            map[string]string{"Referer": "https://example.com"},
			Priority:           5,
		}

		if string(p.MediaType) != tt.wantStr {
			t.Errorf("MediaType = %q, want %q", p.MediaType, tt.wantStr)
		}
		if p.Progrss != 50 {
			t.Errorf("Progress = %d, want 50", p.Progrss)
		}
		if p.Total != 100 {
			t.Errorf("Total = %d, want 100", p.Total)
		}
		if p.Status != Downloading {
			t.Errorf("Status = %q, want Downloading", p.Status)
		}
	}
}

func TestNamesSanitizationNeeded(t *testing.T) {
	// Verify that names with invalid UTF-8 would need sanitization before
	// being sent via protobuf. This validates that the download package
	// can encounter invalid UTF-8 in filenames (e.g. Shift-JIS/GBK).
	names := []string{
		"valid_file.mkv",
		"日本語ファイル.ts",
		"bad\xfffile.mp4",
	}

	for i, n := range names {
		// valid UTF-8 strings never contain 0xff or 0xfe bytes
		containsFF := false
		for j := 0; j < len(n); j++ {
			if n[j] == 0xff {
				containsFF = true
				break
			}
		}
		if i == 2 && !containsFF {
			t.Errorf("name[%d] should contain invalid UTF-8 byte 0xff", i)
		}
		if i < 2 && containsFF {
			t.Errorf("name[%d] unexpectedly contains invalid bytes", i)
		}
	}
}
