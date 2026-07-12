package network

import (
	"bytes"
	"io"
	"net/http"
	"testing"

	"github.com/andybalholm/brotli"
)

// TestReadDecompressedMisleadingBr guards the regression where a server
// advertises Content-Encoding: br but actually sends base64/plain text
// (miruro.tv does this). readDecompressed must NOT try to brotli-decode the
// text (which raised "brotli: HUFFMAN_SPACE"); it must return the raw body so
// the caller can decode it itself.
func TestReadDecompressedMisleadingBr(t *testing.T) {
	raw := []byte("bh4YNPj7z1PYnmhREE7Z6-R7r-Y6VnXhb52vQ2uBbIixElS-135vdxM2YXqIm0NF")
	resp := &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Encoding": []string{"br"}},
		Body:       io.NopCloser(bytes.NewReader(raw)),
	}
	got, err := readDecompressed(resp)
	if err != nil {
		t.Fatalf("readDecompressed returned error on misleading br: %v", err)
	}
	if !bytes.Equal(got, raw) {
		t.Fatalf("expected raw body returned unchanged, got %q", got)
	}
}

// TestReadDecompressedRealBrotli confirms a genuinely brotli-compressed body
// is still decompressed correctly through the same code path.
func TestReadDecompressedRealBrotli(t *testing.T) {
	const want = `{"ok":true}`
	var buf bytes.Buffer
	bw := brotli.NewWriter(&buf)
	if _, err := bw.Write([]byte(want)); err != nil {
		t.Fatal(err)
	}
	if err := bw.Close(); err != nil {
		t.Fatal(err)
	}
	resp := &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Encoding": []string{"br"}},
		Body:       io.NopCloser(bytes.NewReader(buf.Bytes())),
	}
	got, err := readDecompressed(resp)
	if err != nil {
		t.Fatalf("readDecompressed real brotli: %v", err)
	}
	if string(got) != want {
		t.Fatalf("brotli decode mismatch: got %q want %q", got, want)
	}
}
