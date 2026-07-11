package js

import (
	"fmt"
	"strings"
	"testing"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/require"
)

func runZlibTest(t *testing.T, compressFn, decompressFn, input string) {
	registry := require.NewRegistry()
	RegisterZlibModule(registry)

	vm := goja.New()
	registry.Enable(vm)
	vm.RunString(`var zlib = require('zlib');`)

	bytes := []byte(input)
	var jsBytes strings.Builder
	jsBytes.WriteString("new Uint8Array([")
	for i, c := range bytes {
		if i > 0 {
			jsBytes.WriteString(",")
		}
		jsBytes.WriteString(fmt.Sprintf("%d", c))
	}
	jsBytes.WriteString("])")

	script := `(function() {
		var zlib = require('zlib');
		var input = ` + jsBytes.String() + `;
		var compressed = zlib.` + compressFn + `(input);
		var decompressed = zlib.` + decompressFn + `(compressed);
		var arr = [];
		for (var i = 0; i < decompressed.length; i++) {
			arr[i] = decompressed[i];
		}
		return arr;
	})()`

	result, err := vm.RunString(script)
	if err != nil {
		t.Fatalf("[%s+%s] JavaScript error: %v", compressFn, decompressFn, err)
	}

	exported := result.Export()
	var got []byte
	switch v := exported.(type) {
	case []interface{}:
		got = make([]byte, len(v))
		for i, item := range v {
			switch n := item.(type) {
			case int64:
				got[i] = byte(n)
			case float64:
				got[i] = byte(n)
			}
		}
	default:
		t.Fatalf("[%s+%s] unexpected result type: %T", compressFn, decompressFn, exported)
	}

	expected := []byte(input)
	if len(got) != len(expected) {
		t.Fatalf("[%s+%s] length mismatch: expected %d, got %d\n  expected: %v\n  got:      %v",
			compressFn, decompressFn, len(expected), len(got), expected, got)
	}
	for i := range expected {
		if got[i] != expected[i] {
			t.Fatalf("[%s+%s] byte %d mismatch: expected %d, got %d\n  expected: %v\n  got:      %v",
				compressFn, decompressFn, i, expected[i], got[i], expected, got)
		}
	}
}

var testVectors = []struct {
	name  string
	input string
}{
	{"simple", "Hello, World!"},
	{"empty", ""},
	{"single_byte", "X"},
	{"binary_data", "\x00\x01\x02\x03\xff\xfe\xfd"},
	{"multibyte_utf8", "Hello 世界 🌍"},
	{"newlines", "line1\nline2\nline3\n"},
	{"long_text", "The quick brown fox jumps over the lazy dog. " +
		"Pack my box with five dozen liquor jugs. " +
		"How vexingly quick daft zebras jump! 0123456789"},
}

func TestZlibAllAlgorithms(t *testing.T) {
	algorithms := []struct {
		name       string
		compress   string
		decompress string
	}{
		{"gzip", "gzipSync", "gunzipSync"},
		{"deflate", "deflateSync", "inflateSync"},
		{"brotli", "brotliCompressSync", "brotliDecompressSync"},
		{"zstd", "zstdCompressSync", "zstdDecompressSync"},
	}

	for _, algo := range algorithms {
		t.Run(algo.name, func(t *testing.T) {
			for _, vec := range testVectors {
				t.Run(vec.name, func(t *testing.T) {
					runZlibTest(t, algo.compress, algo.decompress, vec.input)
				})
			}
		})
	}
}

func TestZlibModuleExposed(t *testing.T) {
	registry := require.NewRegistry()
	RegisterZlibModule(registry)

	vm := goja.New()
	registry.Enable(vm)

	// Verify that zlib is NOT available as a global unless require'd
	vm.RunString(`
		if (typeof zlib !== 'undefined') {
			throw new Error('zlib should not be globally accessible');
		}
	`)

	// Verify that zlib IS accessible via require('zlib')
	result, err := vm.RunString(`require('zlib')`)
	if err != nil {
		t.Fatalf("require('zlib') failed: %v", err)
	}

	// Check all 8 functions are present
	functions := []string{
		"gzipSync", "gunzipSync",
		"deflateSync", "inflateSync",
		"brotliCompressSync", "brotliDecompressSync",
		"zstdCompressSync", "zstdDecompressSync",
	}

	obj := result.ToObject(vm)
	for _, fn := range functions {
		if val := obj.Get(fn); val == nil || goja.IsUndefined(val) {
			t.Errorf("zlib.%s is not defined", fn)
		}
	}
}
