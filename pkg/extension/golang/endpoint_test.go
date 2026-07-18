package golang

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/miru-project/miru-core/pkg/extension"
	"github.com/miru-project/miru-core/proto/generate/proto"
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
	// Under the V2 layout Mirror() resolves the chosen source into the final
	// per-type watch, not a list of candidates.
	res, err := Mirror("example", "https://example.com/1")
	assert.NoError(t, err)
	assert.NotNil(t, res)
	all, ok := res.(*proto.ExtensionAllWatch)
	assert.True(t, ok, "expected *proto.ExtensionAllWatch, got %T", res)
	assert.NotNil(t, all.Bangumi)
	assert.Equal(t, "https://example.com/1", all.Bangumi.Url)
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
	assert.Equal(t, extension.WatchTypeAll, meta.WatchType)
	assert.Equal(t, "v0.1.0", meta.Version)
}

func TestParseWatchType(t *testing.T) {
	// The four valid kinds parse to their enum constant.
	for _, tc := range []struct {
		raw  string
		want extension.WatchType
	}{
		{"bangumi", extension.WatchTypeBangumi},
		{"manga", extension.WatchTypeManga},
		{"fikushon", extension.WatchTypeFikushon},
		{"all", extension.WatchTypeAll},
	} {
		got, err := extension.ParseWatchType(tc.raw)
		assert.NoError(t, err, "expected %q to be valid", tc.raw)
		assert.Equal(t, tc.want, got)
	}

	// Anything else is rejected so non-conforming extensions fail fast.
	for _, bad := range []string{"video", "novel", "", "Bangumi"} {
		_, err := extension.ParseWatchType(bad)
		assert.Error(t, err, "expected %q to be rejected", bad)
	}
}
