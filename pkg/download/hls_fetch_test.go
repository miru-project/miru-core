package download

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/grafov/m3u8"
)

// TestFetchHlsResource_RejectsErrorPages verifies that an HTTP error status
// is surfaced as an error instead of returning the error-page body as if it
// were a valid segment (the root cause of corrupt HLS segment files).
func TestFetchHlsResource_RejectsErrorPages(t *testing.T) {
	resetSchedulerState()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ok.ts":
			_, _ = w.Write([]byte("segment-bytes"))
		case "/missing.ts":
			http.Error(w, "not found", http.StatusNotFound)
		case "/empty.ts":
			w.WriteHeader(http.StatusOK)
		default:
			http.Error(w, "boom", http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	body, err := fetchHlsResource(srv.URL+"/ok.ts", nil)
	if err != nil || string(body) != "segment-bytes" {
		t.Fatalf("expected success, got body=%q err=%v", body, err)
	}

	// Permanent 4xx must fail immediately (no retry sleeps).
	if _, err := fetchHlsResource(srv.URL+"/missing.ts", nil); err == nil {
		t.Fatal("expected 404 to be reported as an error")
	}

	// A 200 with an empty body is a corrupt segment, not a success.
	if _, err := fetchHlsResource(srv.URL+"/empty.ts", nil); err == nil {
		t.Fatal("expected empty body to be reported as an error")
	}
}

// TestGetIV_ParsesHexLiteral verifies the EXT-X-KEY IV is decoded from its
// hex representation instead of using the ASCII bytes of the string.
func TestGetIV_ParsesHexLiteral(t *testing.T) {
	key := &m3u8.Key{Method: "AES-128", IV: "0x00112233445566778899aabbccddeeff"}
	iv := getIV(key, 0)
	want := []byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77,
		0x88, 0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}
	if !bytes.Equal(iv, want) {
		t.Fatalf("getIV hex = %x, want %x", iv, want)
	}

	// Without an explicit IV the sequence number is used (big-endian tail).
	iv = getIV(&m3u8.Key{Method: "AES-128"}, 7)
	if iv[15] != 7 {
		t.Fatalf("getIV seq fallback = %x, want last byte 7", iv)
	}
}

// TestDecodePlaylist_ReportsNonM3u8Body verifies that a 200 response whose
// body is not an m3u8 (e.g. an HTML interstitial) produces an error naming
// the source URL and a body snippet, instead of a bare "#EXTM3U absent".
func TestDecodePlaylist_ReportsNonM3u8Body(t *testing.T) {
	html := "<!DOCTYPE html><html><head><title>Access Denied</title></head><body>403</body></html>"
	_, _, err := decodePlaylist(html, "http://cdn/playlist.m3u8")
	if err == nil {
		t.Fatal("expected decode error for HTML body")
	}
	msg := err.Error()
	if !strings.Contains(msg, "http://cdn/playlist.m3u8") || !strings.Contains(msg, "Access Denied") {
		t.Fatalf("error should name the URL and show a body snippet, got: %s", msg)
	}

	// A valid playlist still decodes cleanly.
	ok := "#EXTM3U\n#EXT-X-TARGETDURATION:10\n#EXTINF:10.0,\ns0.ts\n#EXT-X-ENDLIST\n"
	if _, _, err := decodePlaylist(ok, "http://cdn/ok.m3u8"); err != nil {
		t.Fatalf("valid playlist rejected: %v", err)
	}
}

// TestHlsKeyResolver verifies NONE methods skip fetching, AES-128 keys are
// downloaded once and cached per URI, and short keys are rejected.
func TestHlsKeyResolver(t *testing.T) {
	resetSchedulerState()
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/key1":
			hits++
			_, _ = w.Write(bytes.Repeat([]byte{0xAB}, 16))
		case "/shortkey":
			_, _ = w.Write([]byte("tooshort"))
		}
	}))
	defer srv.Close()

	r := newHlsKeyResolver(srv.URL+"/playlist.m3u8", nil)

	k, err := r.resolve(&m3u8.Key{Method: "NONE"})
	if err != nil || k != nil {
		t.Fatalf("NONE should resolve to nil key, got %v %v", k, err)
	}

	meta := &m3u8.Key{Method: "AES-128", URI: srv.URL + "/key1"}
	k, err = r.resolve(meta)
	if err != nil || len(k) != 16 || k[0] != 0xAB {
		t.Fatalf("AES-128 resolve = %x err=%v", k, err)
	}
	if _, err := r.resolve(meta); err != nil {
		t.Fatal(err)
	}
	if hits != 1 {
		t.Fatalf("expected cached key (1 fetch), got %d fetches", hits)
	}

	if _, err := r.resolve(&m3u8.Key{Method: "AES-128", URI: srv.URL + "/shortkey"}); err == nil {
		t.Fatal("expected invalid key size to be an error")
	}

	if _, err := r.resolve(&m3u8.Key{Method: "SAMPLE-AES", URI: "x"}); err == nil {
		t.Fatal("expected unsupported method to be an error")
	}
}
