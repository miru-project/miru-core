package golang

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSearchEndpoint(t *testing.T) {
	ExtensionDir = filepath.Join("extensions", "example")
	items, err := Search("example", 1, "test", "")
	assert.NoError(t, err)
	assert.Len(t, items, 2)
	assert.Equal(t, "Example Result 1", items[0].Title)
	assert.Equal(t, "https://example.com/1", items[0].Url)
}

func TestLatestEndpoint(t *testing.T) {
	ExtensionDir = filepath.Join("extensions", "example")
	items, err := Latest("example", 1)
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, "Latest Example 1", items[0].Title)
	assert.Equal(t, "https://example.com/latest/1", items[0].Url)
}

func TestDetailEndpoint(t *testing.T) {
	ExtensionDir = filepath.Join("extensions", "example")
	detail, err := Detail("example", "https://example.com/1")
	assert.NoError(t, err)
	assert.NotNil(t, detail)
	assert.Equal(t, "Example Detail", *detail.Title)
}

func TestWatchEndpoint(t *testing.T) {
	ExtensionDir = filepath.Join("extensions", "example")
	watch, _, err := Watch("example", "https://example.com/1")
	assert.NoError(t, err)
	assert.NotNil(t, watch)
}

func TestMirrorEndpoint(t *testing.T) {
	ExtensionDir = filepath.Join("extensions", "example")
	mirrors, err := Mirror("example", "https://example.com/1")
	assert.NoError(t, err)
	assert.NotNil(t, mirrors)
}

func TestStdLibMD5(t *testing.T) {
	ExtensionDir = filepath.Join("extensions", "example")
	vm := NewScriggoVM(nil)
	prog, err := vm.Compile("md5_test", `package main

import "fmt"
import "crypto/md5"

func main() {
    hash := md5.Sum([]byte("hello"))
    fmt.Printf("%x", hash)
}
`)
	assert.NoError(t, err)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	_, err = vm.Run(prog, nil)
	w.Close()
	os.Stdout = oldStdout

	var out strings.Builder
	data, _ := io.ReadAll(r)
	out.WriteString(string(data))
	r.Close()

	assert.NoError(t, err)
	assert.Contains(t, out.String(), "5d41402abc4b2a76b9719d911017c592")
}

func TestStdLibGoquery(t *testing.T) {
	ExtensionDir = filepath.Join("extensions", "example")
	vm := NewScriggoVM(nil)
	prog, err := vm.Compile("goquery_test", `package main

import "fmt"
import "strings"
import "github.com/PuerkitoBio/goquery"

func main() {
    doc, _ := goquery.NewDocumentFromReader(strings.NewReader("<html><body><div class='test'>Hello</div></body></html>"))
    text := doc.Find(".test").Text()
    fmt.Println(text)
}
`)
	assert.NoError(t, err)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	_, err = vm.Run(prog, nil)
	w.Close()
	os.Stdout = oldStdout

	var out strings.Builder
	data, _ := io.ReadAll(r)
	out.WriteString(string(data))
	r.Close()

	assert.NoError(t, err)
	assert.Contains(t, out.String(), "Hello")
}

func TestEndpointJSONHelpers(t *testing.T) {
	ExtensionDir = filepath.Join("extensions", "example")
	jsonStr, err := SearchJSON("example", 1, "test", "")
	assert.NoError(t, err)
	assert.Contains(t, jsonStr, "Example Result 1")

	jsonStr, err = LatestJSON("example", 1)
	assert.NoError(t, err)
	assert.Contains(t, jsonStr, "Latest Example 1")

	jsonStr, err = DetailJSON("example", "https://example.com/1")
	assert.NoError(t, err)
	assert.Contains(t, jsonStr, "Example Detail")

	jsonStr, err = WatchJSON("example", "https://example.com/1")
	assert.NoError(t, err)
	assert.Contains(t, jsonStr, "Group 1")

	jsonStr, err = MirrorJSON("example", "https://example.com/1")
	assert.NoError(t, err)
	assert.Contains(t, jsonStr, "Mirror 1")
}

func TestParseExtensionMetadata(t *testing.T) {
	ExtensionDir = filepath.Join("extensions", "example")
	meta, err := ParseExtensionMetadata("example")
	assert.NoError(t, err)
	assert.Equal(t, "Example", meta.Name)
	assert.Equal(t, "v0.1.0", meta.Version)
	assert.Equal(t, "Miru", meta.Author)
	assert.Equal(t, "MIT", meta.License)
	assert.Equal(t, "all", meta.Lang)
	assert.Equal(t, "example", meta.Pkg)
	assert.Equal(t, "bangumi", meta.WatchType)
	assert.Equal(t, "v0.1.0", meta.Version)
}
