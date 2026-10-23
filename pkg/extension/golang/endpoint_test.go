package golang

import (
	"testing"

	"github.com/miru-project/miru-core/pkg/extension"
	"github.com/miru-project/miru-core/proto/generate/proto"
	"github.com/stretchr/testify/assert"
)

func TestSearchEndpoint(t *testing.T) {
	setExampleExtensionDir()
	items, err := Search("example", 1, "test", nil)
	assert.NoError(t, err)
	assert.Len(t, items, 2)
	assert.Equal(t, "Example Result 1", items[0].Title)
	assert.Equal(t, "https://example.com/1", items[0].Url)
}

// TestSearchFilterRoundTrip verifies the full server-side filter pipeline:
// 1. CreateFilter returns the extension's filter definitions.
// 2. A FilterSelection built from those definitions is passed to Search.
// 3. Search receives the selection and returns results (the example extension
//    ignores filter values, so we assert the results stay deterministic).
func TestSearchFilterRoundTrip(t *testing.T) {
	setExampleExtensionDir()

	// Step 1 – get the filter definitions (simulates the initial UI load).
	filters, err := CreateFilter("example", nil)
	assert.NoError(t, err)
	assert.NotNil(t, filters)
	assert.Contains(t, filters, "type", "example extension must expose a 'type' filter")
	assert.Contains(t, filters, "language", "example extension must expose a 'language' filter")

	// Step 2 – build a filter selection (simulates user picking "manga" from
	// the "type" filter and leaving "language" alone) and Search.
	sel := &proto.FilterSelection{Selections: map[string]*proto.FilterSelectionValue{
		"type": {Values: []string{"manga"}},
	}}
	items, err := Search("example", 1, "test", sel)
	assert.NoError(t, err)
	assert.Len(t, items, 2)
	assert.Equal(t, "Example Result 1", items[0].Title)
	assert.Equal(t, "https://example.com/1", items[0].Url)

	// Step 3 – multi-select and multi-field.
	sel2 := &proto.FilterSelection{Selections: map[string]*proto.FilterSelectionValue{
		"type":     {Values: []string{"manga", "bangumi"}},
		"language": {Values: []string{"ja"}},
	}}
	items2, err := Search("example", 1, "module", sel2)
	assert.NoError(t, err)
	assert.Len(t, items2, 2)
}

// TestCreateFilterVariants verifies every filter variant the runtime can emit
// survives the typed -> proto conversion. A variant that is missing from
// FilterDefinitionToProto silently reaches the UI as an empty ExtensionFilter
// (whichKind() == notSet), which renders as a blank filter row rather than an
// error, so each case is asserted explicitly.
func TestCreateFilterVariants(t *testing.T) {
	setExampleExtensionDir()

	filters, err := CreateFilter("example", nil)
	assert.NoError(t, err)

	// select
	sel, ok := filters["type"].GetKind().(*proto.ExtensionFilter_Select)
	assert.True(t, ok, "type should be a select, got %T", filters["type"].GetKind())
	assert.Equal(t, "Type", sel.Select.Title)
	assert.Len(t, sel.Select.Options, 3)

	// multi_select, including its min/max bounds and default
	multi, ok := filters["quality"].GetKind().(*proto.ExtensionFilter_MultiSelect)
	assert.True(t, ok, "quality should be a multi-select, got %T", filters["quality"].GetKind())
	assert.Equal(t, "Quality", multi.MultiSelect.Title)
	assert.Equal(t, int32(1), multi.MultiSelect.Min)
	assert.Equal(t, int32(3), multi.MultiSelect.Max)
	assert.Equal(t, []string{"1080p"}, multi.MultiSelect.Default)

	// range, including its bounds and defaults
	rng, ok := filters["year"].GetKind().(*proto.ExtensionFilter_Range)
	assert.True(t, ok, "year should be a range, got %T", filters["year"].GetKind())
	assert.Equal(t, int32(1990), rng.Range.Min)
	assert.Equal(t, int32(2025), rng.Range.Max)
	assert.Equal(t, int32(2000), rng.Range.DefaultMin)
	assert.Equal(t, int32(2025), rng.Range.DefaultMax)

	// text: title + optional hint survive; the value shape is a single string
	// so a selection of one element is what the UI sends back.
	txt, ok := filters["author"].GetKind().(*proto.ExtensionFilter_Text)
	assert.True(t, ok, "author should be a text filter, got %T", filters["author"].GetKind())
	assert.Equal(t, "Author", txt.Text.Title)
	assert.Equal(t, "e.g. Fujimoto", txt.Text.Hint)
}

// TestTextFilterSelectionRoundTrip verifies a typed value travels the whole
// path: FilterSelection -> filterFromProto -> sdk.Filter -> the extension.
// The example extension ignores filter values, so the assertion is that the
// typed text reaches it intact and Search still returns deterministic results.
func TestTextFilterSelectionRoundTrip(t *testing.T) {
	setExampleExtensionDir()

	sel := &proto.FilterSelection{Selections: map[string]*proto.FilterSelectionValue{
		"author": {Values: []string{"Fujimoto"}},
	}}

	// The proto -> runtime.Filter conversion is what the extension sees.
	f := filterFromProto(sel)
	assert.Equal(t, []string{"Fujimoto"}, f.Selections["author"].Values)

	items, err := Search("example", 1, "test", sel)
	assert.NoError(t, err)
	assert.Len(t, items, 2)
}

func TestLatestEndpoint(t *testing.T) {
	setExampleExtensionDir()
	items, err := Latest("example", 1)
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, "Latest Example 1", items[0].Title)
	assert.Equal(t, "https://example.com/latest/1", items[0].Url)
}

func TestDetailEndpoint(t *testing.T) {
	setExampleExtensionDir()
	detail, err := Detail("example", "https://example.com/1")
	assert.NoError(t, err)
	assert.NotNil(t, detail)
	assert.Equal(t, "Example Detail", *detail.Title)
	// Tags must survive the Scriggo VM boundary, so Go extensions can feed
	// the detail-page chips exactly like the JS runtime does.
	assert.Equal(t, []string{"example", "fixture"}, detail.Tags)
}

func TestWatchEndpoint(t *testing.T) {
	setExampleExtensionDir()
	watch, _, err := Watch("example", "https://example.com/1")
	assert.NoError(t, err)
	assert.NotNil(t, watch)
}

func TestMirrorEndpoint(t *testing.T) {
	setExampleExtensionDir()
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
	setExampleExtensionDir()
	vm := NewScriggoVM(nil)
	prog, err := vm.Compile("", "md5_test", `package main

import "fmt"
import "crypto/md5"

func main() {
    hash := md5.Sum([]byte("hello"))
    fmt.Printf("%x", hash)
}
`)
	assert.NoError(t, err)

	out := captureStdout(t, func() {
		_, err = vm.Run(prog, nil)
	})

	assert.NoError(t, err)
	assert.Contains(t, out, "5d41402abc4b2a76b9719d911017c592")
}

func TestStdLibGoquery(t *testing.T) {
	setExampleExtensionDir()
	vm := NewScriggoVM(nil)
	prog, err := vm.Compile("", "goquery_test", `package main

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

	out := captureStdout(t, func() {
		_, err = vm.Run(prog, nil)
	})

	assert.NoError(t, err)
	assert.Contains(t, out, "Hello")
}

func TestParseExtensionMetadata(t *testing.T) {
	setExampleExtensionDir()
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
