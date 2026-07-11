package js

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"encoding/base64"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/andybalholm/brotli"
	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/require"
	"github.com/klauspost/compress/zstd"
)

const maxDecompressedSize = 50 << 20

func boundedReadAll(r io.Reader) ([]byte, error) {
	var buf bytes.Buffer
	_, err := buf.ReadFrom(io.LimitReader(r, maxDecompressedSize+1))
	if err != nil {
		return nil, err
	}
	if buf.Len() > maxDecompressedSize {
		return nil, fmt.Errorf("decompressed size exceeds %d bytes", maxDecompressedSize)
	}
	return buf.Bytes(), nil
}

// RegisterZlibModule registers the zlib module as a require-able native module
// in the shared require registry, so it can be loaded via
// `const zlib = require('zlib')` in extension scripts.
//
// It also exposes the same module globally as `globalThis.zlib` for convenience.
//
// Available functions:
//   - zlib.gzipSync(input) -> Uint8Array
//   - zlib.gunzipSync(input) -> Uint8Array
//   - zlib.deflateSync(input) -> Uint8Array
//   - zlib.inflateSync(input) -> Uint8Array
//   - zlib.brotliCompressSync(input) -> Uint8Array
//   - zlib.brotliDecompressSync(input) -> Uint8Array
//   - zlib.zstdCompressSync(input) -> Uint8Array
//   - zlib.zstdDecompressSync(input) -> Uint8Array
//   - zlib.bytesFromBase64(str) -> Uint8Array  // base64url decode that preserves binary data
func RegisterZlibModule(registry *require.Registry) {
	registry.RegisterNativeModule("zlib", func(vm *goja.Runtime, module *goja.Object) {
		zlibObj := buildZlibObject(vm)
		module.Set("exports", zlibObj)
		vm.Set("zlib", zlibObj)
	})
}

// buildZlibObject constructs a fresh goja Object with all zlib functions attached.
func buildZlibObject(vm *goja.Runtime) *goja.Object {
	zlibObj := vm.NewObject()

	// bytesFromBase64 decodes a base64url string to a Uint8Array.
	// This bypasses JS atob() which corrupts binary data through UTF-8 encoding.
	// The input can be base64url (with - and _) or standard base64 (with + and /).
	// Padding is added automatically.
	zlibObj.Set("bytesFromBase64", func(call goja.FunctionCall) goja.Value {
		arg := call.Argument(0)
		if goja.IsUndefined(arg) || goja.IsNull(arg) {
			panic(vm.ToValue("TypeError: zlib.bytesFromBase64: input is required"))
		}
		str := strings.TrimRight(arg.String(), "\n")
		b64std := strings.NewReplacer("-", "+", "_", "/").Replace(str)
		pad := 4 - (len(b64std) % 4)
		if pad != 4 {
			b64std += strings.Repeat("=", pad)
		}
		decoded, err := base64.StdEncoding.DecodeString(b64std)
		if err != nil {
			panic(vm.ToValue(fmt.Sprintf("Error: zlib.bytesFromBase64: %v", err)))
		}
		return bytesToUint8Array(vm, decoded)
	})

	// gzipSync compresses data using gzip
	zlibObj.Set("gzipSync", func(call goja.FunctionCall) goja.Value {
		input := call.Argument(0)
		if goja.IsUndefined(input) || goja.IsNull(input) {
			panic(vm.ToValue("TypeError: zlib.gzipSync: input is required"))
		}

		data := jsValueToBytes(input)
		var buf bytes.Buffer
		w := gzip.NewWriter(&buf)
		if _, err := w.Write(data); err != nil {
			panic(vm.ToValue(fmt.Sprintf("Error: zlib.gzipSync: %v", err)))
		}
		if err := w.Close(); err != nil {
			panic(vm.ToValue(fmt.Sprintf("Error: zlib.gzipSync: %v", err)))
		}

		return bytesToUint8Array(vm, buf.Bytes())
	})

	// gunzipSync decompresses gzip data
	// Tries gzip.NewReader first; on failure, manually skips gzip header and uses flate.NewReader
	zlibObj.Set("gunzipSync", func(call goja.FunctionCall) goja.Value {
		input := call.Argument(0)
		if goja.IsUndefined(input) || goja.IsNull(input) {
			panic(vm.ToValue("TypeError: zlib.gunzipSync: input is required"))
		}

		data := jsValueToBytes(input)

		if len(data) < 2 || data[0] != 0x1f || data[1] != 0x8b {
			panic(vm.ToValue("Error: zlib.gunzipSync: not a gzip stream"))
		}

		// Try standard gzip.NewReader first
		r, err := gzip.NewReader(bytes.NewReader(data))
		if err == nil {
			defer r.Close()
			result, readErr := boundedReadAll(r)
			if readErr == nil {
				return bytesToUint8Array(vm, result)
			}
		}

		// Fallback: manually skip gzip header, then flate.NewReader (handles truncated gzip)
		flg := data[3]
		p := 10
		if flg&0x04 != 0 {
			xlen := int(data[p]) | (int(data[p+1]) << 8)
			p += 2 + xlen
		}
		if flg&0x08 != 0 {
			for p < len(data) && data[p] != 0 {
				p++
			}
			p++
		}
		if flg&0x10 != 0 {
			for p < len(data) && data[p] != 0 {
				p++
			}
			p++
		}
		if flg&0x02 != 0 {
			p += 2
		}

		if p >= len(data) {
			panic(vm.ToValue("Error: zlib.gunzipSync: corrupt gzip header"))
		}

		fr := flate.NewReader(bytes.NewReader(data[p:]))
		if fr == nil {
			panic(vm.ToValue("Error: zlib.gunzipSync: flate: failed to create reader"))
		}
		defer fr.Close()

		result, err := boundedReadAll(fr)
		if err != nil {
			panic(vm.ToValue(fmt.Sprintf("Error: zlib.gunzipSync: %v", err)))
		}

		return bytesToUint8Array(vm, result)
	})

	// deflateSync compresses data using zlib (deflate)
	zlibObj.Set("deflateSync", func(call goja.FunctionCall) goja.Value {
		input := call.Argument(0)
		if goja.IsUndefined(input) || goja.IsNull(input) {
			panic(vm.ToValue("TypeError: zlib.deflateSync: input is required"))
		}

		data := jsValueToBytes(input)
		var buf bytes.Buffer
		w := zlib.NewWriter(&buf)
		if _, err := w.Write(data); err != nil {
			panic(vm.ToValue(fmt.Sprintf("Error: zlib.deflateSync: %v", err)))
		}
		if err := w.Close(); err != nil {
			panic(vm.ToValue(fmt.Sprintf("Error: zlib.deflateSync: %v", err)))
		}

		return bytesToUint8Array(vm, buf.Bytes())
	})

	// inflateSync decompresses zlib (deflate) data
	zlibObj.Set("inflateSync", func(call goja.FunctionCall) goja.Value {
		input := call.Argument(0)
		if goja.IsUndefined(input) || goja.IsNull(input) {
			panic(vm.ToValue("TypeError: zlib.inflateSync: input is required"))
		}

		data := jsValueToBytes(input)
		r, err := zlib.NewReader(bytes.NewReader(data))
		if err != nil {
			panic(vm.ToValue(fmt.Sprintf("Error: zlib.inflateSync: %v", err)))
		}
		defer r.Close()

		result, err := boundedReadAll(r)
		if err != nil {
			panic(vm.ToValue(fmt.Sprintf("Error: zlib.inflateSync: %v", err)))
		}

		return bytesToUint8Array(vm, result)
	})

	// brotliCompressSync compresses data using brotli
	zlibObj.Set("brotliCompressSync", func(call goja.FunctionCall) goja.Value {
		input := call.Argument(0)
		if goja.IsUndefined(input) || goja.IsNull(input) {
			panic(vm.ToValue("TypeError: zlib.brotliCompressSync: input is required"))
		}

		data := jsValueToBytes(input)
		var buf bytes.Buffer
		w := brotli.NewWriter(&buf)
		if _, err := w.Write(data); err != nil {
			panic(vm.ToValue(fmt.Sprintf("Error: zlib.brotliCompressSync: %v", err)))
		}
		if err := w.Close(); err != nil {
			panic(vm.ToValue(fmt.Sprintf("Error: zlib.brotliCompressSync: %v", err)))
		}

		return bytesToUint8Array(vm, buf.Bytes())
	})

	// brotliDecompressSync decompresses brotli data
	zlibObj.Set("brotliDecompressSync", func(call goja.FunctionCall) goja.Value {
		input := call.Argument(0)
		if goja.IsUndefined(input) || goja.IsNull(input) {
			panic(vm.ToValue("TypeError: zlib.brotliDecompressSync: input is required"))
		}

		data := jsValueToBytes(input)
		r := brotli.NewReader(bytes.NewReader(data))

		result, err := boundedReadAll(r)
		if err != nil {
			panic(vm.ToValue(fmt.Sprintf("Error: zlib.brotliDecompressSync: %v", err)))
		}

		return bytesToUint8Array(vm, result)
	})

	// zstdCompressSync compresses data using zstd
	zlibObj.Set("zstdCompressSync", func(call goja.FunctionCall) goja.Value {
		input := call.Argument(0)
		if goja.IsUndefined(input) || goja.IsNull(input) {
			panic(vm.ToValue("TypeError: zlib.zstdCompressSync: input is required"))
		}

		data := jsValueToBytes(input)
		encoder, err := zstd.NewWriter(nil)
		if err != nil {
			panic(vm.ToValue(fmt.Sprintf("Error: zlib.zstdCompressSync: %v", err)))
		}
		defer encoder.Close()

		result := encoder.EncodeAll(data, nil)
		return bytesToUint8Array(vm, result)
	})

	// zstdDecompressSync decompresses zstd data
	zlibObj.Set("zstdDecompressSync", func(call goja.FunctionCall) goja.Value {
		input := call.Argument(0)
		if goja.IsUndefined(input) || goja.IsNull(input) {
			panic(vm.ToValue("TypeError: zlib.zstdDecompressSync: input is required"))
		}

		data := jsValueToBytes(input)
		decoder, err := zstd.NewReader(nil)
		if err != nil {
			panic(vm.ToValue(fmt.Sprintf("Error: zlib.zstdDecompressSync: %v", err)))
		}
		defer decoder.Close()

		result, err := decoder.DecodeAll(data, nil)
		if err != nil {
			panic(vm.ToValue(fmt.Sprintf("Error: zlib.zstdDecompressSync: %v", err)))
		}

		return bytesToUint8Array(vm, result)
	})

	// Constants for compression levels
	zlibObj.Set("constants", vm.NewObject())

	return zlibObj
}

// jsValueToBytes converts a goja value (Uint8Array, ArrayBuffer, or string) to a byte slice.
func jsValueToBytes(val goja.Value) []byte {
	if val == nil || goja.IsUndefined(val) || goja.IsNull(val) {
		return nil
	}

	if exported := val.Export(); exported != nil {
		switch v := exported.(type) {
		case []byte:
			return v
		case string:
			return []byte(v)
		case []interface{}:
			buf := make([]byte, len(v))
			for i, item := range v {
				switch n := item.(type) {
				case int64:
					buf[i] = byte(n)
				case int:
					buf[i] = byte(n)
				case float64:
					buf[i] = byte(int(n))
				case uint8:
					buf[i] = n
				}
			}
			return buf
		}
	}

	obj := val.ToObject(nil)
	if obj == nil {
		return nil
	}
	lengthVal := obj.Get("byteLength")
	if lengthVal == nil || goja.IsUndefined(lengthVal) {
		lengthVal = obj.Get("length")
	}
	if lengthVal == nil || goja.IsUndefined(lengthVal) {
		return nil
	}
	length := int(lengthVal.ToInteger())
	if length <= 0 {
		return []byte{}
	}
	return readTypedArrayBytes(obj, length)
}

// readTypedArrayBytes extracts bytes from a JS typed array by reading each index.
func readTypedArrayBytes(obj *goja.Object, length int) []byte {
	buf := make([]byte, length)
	for i := 0; i < length; i++ {
		idx := obj.Get(strconv.Itoa(i))
		if idx == nil || goja.IsUndefined(idx) {
			continue
		}
		num := idx.ToInteger()
		if num < 0 {
			buf[i] = byte(int(num) & 0xff)
		} else if num > 255 {
			buf[i] = byte(int(num) & 0xff)
		} else {
			buf[i] = byte(num)
		}
	}
	return buf
}

// bytesToUint8Array converts a Go byte slice to a JS Uint8Array in the goja runtime.
func bytesToUint8Array(vm *goja.Runtime, b []byte) goja.Value {
	if b == nil || len(b) == 0 {
		empty, _ := vm.RunString("new Uint8Array(0)")
		return empty
	}

	var sb strings.Builder
	sb.Grow(len(b)*4 + 20)
	sb.WriteString("new Uint8Array([")
	for i, c := range b {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(strconv.Itoa(int(c)))
	}
	sb.WriteString("])")
	val, err := vm.RunString(sb.String())
	if err != nil {
		empty, _ := vm.RunString("new Uint8Array(0)")
		return empty
	}
	return val
}
