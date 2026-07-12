package network

import (
	"bytes"
	"testing"

	"github.com/andybalholm/brotli"
	"github.com/valyala/fasthttp"
)

// TestReadAllBrotli verifies the brotli decode path in ReadAll decodes a real
// brotli-compressed body (regression guard for the "brotli: HUFFMAN_SPACE"
// error that came from fasthttp's built-in BodyUnbrotli).
func TestReadAllBrotli(t *testing.T) {
	const want = `{"hello":"world","numbers":[1,2,3,4,5],"nested":{"a":{"b":{"c":true}}}}`
	var compressed bytes.Buffer
	bw := brotli.NewWriterLevel(&compressed, brotli.BestCompression)
	if _, err := bw.Write([]byte(want)); err != nil {
		t.Fatal(err)
	}
	if err := bw.Close(); err != nil {
		t.Fatal(err)
	}

	var res fasthttp.Response
	res.Header.SetStatusCode(fasthttp.StatusOK)
	res.Header.Set("Content-Encoding", "br")
	res.SetBody(compressed.Bytes())

	got, err := ReadAll(&res)
	if err != nil {
		t.Fatalf("ReadAll brotli failed: %v", err)
	}
	if string(got) != want {
		t.Fatalf("brotli decode mismatch:\n got: %q\nwant: %q", string(got), want)
	}
}
