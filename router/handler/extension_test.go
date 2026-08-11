package handler

import (
	"os"
	"path/filepath"
	"testing"

	endpoint "github.com/miru-project/miru-core/pkg/extension/endpoint"
	golang "github.com/miru-project/miru-core/pkg/extension/golang"
	js "github.com/miru-project/miru-core/pkg/extension/js"
	"github.com/miru-project/miru-core/proto/generate/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setExampleExtensionDir(t *testing.T) {
	t.Helper()
	golangDir, err := filepath.Abs(filepath.Join("..", "..", "pkg", "extension", "golang", "testdata", "example"))
	assert.NoError(t, err)
	golang.ExtensionDir = golangDir
	t.Cleanup(func() { golang.ExtensionDir = "" })
}

// decodeFilter casts one entry of the handler's result map into a
// proto.ExtensionFilter so tests can assert on the typed oneof.
func decodeFilter(t *testing.T, raw any) *proto.ExtensionFilter {
	t.Helper()
	f, ok := raw.(*proto.ExtensionFilter)
	require.True(t, ok, "filter entry should be a *proto.ExtensionFilter")
	return f
}

func TestGetRuntimeUnknownReturnsError(t *testing.T) {
	_, err := endpoint.GetRuntime("missing-extension")
	assert.Error(t, err)
}

func TestGetRuntimeDetectsGolang(t *testing.T) {
	setExampleExtensionDir(t)
	rt, err := endpoint.GetRuntime("example")
	assert.NoError(t, err)
	assert.IsType(t, golang.ExtensionRuntime{}, rt)
}

func TestGetRuntimeDetectsJavaScript(t *testing.T) {
	jsDir, err := os.MkdirTemp("", "miru-js-ext")
	assert.NoError(t, err)
	defer os.RemoveAll(jsDir)

	origPath := js.ExtPath
	defer func() { js.ExtPath = origPath }()

	js.ExtPath = jsDir
	assert.NoError(t, os.WriteFile(filepath.Join(jsDir, "example.js"), []byte("// ==MiruExtension==\n// @package example\n// ==/MiruExtension=="), 0644))
	rt, err := endpoint.GetRuntime("example")
	assert.NoError(t, err)
	assert.IsType(t, js.Runtime{}, rt)
}

// TestHandlerSearch tests the handler-level Search function, which is the
// entry point called by the gRPC server when the frontend performs a search.
//
// In the frontend flow, when a filter is TAPPED AND APPLIED, the selected
// filter values are serialized to JSON (e.g. {"type":"manga"}) and passed as
// the `filter` parameter to search(). This test verifies the handler
// correctly forwards the filter string through endpoint.GetRuntime ->
// golang.Search -> callExtension -> the extension's Search function, and
// returns success.
func TestHandlerSearch(t *testing.T) {
	setExampleExtensionDir(t)

	t.Run("search without filter (empty string)", func(t *testing.T) {
		res := Search("1", "example", "test", nil)
		assert.Equal(t, 200, res.Code)
		assert.NotNil(t, res.Data)
		assert.Len(t, res.Data, 2)
		assert.Equal(t, "Example Result 1", res.Data[0].Title)
		assert.Equal(t, "https://example.com/1", res.Data[0].Url)
	})

	t.Run("search with applied filter (proto selection)", func(t *testing.T) {
		// When filter is applied in the frontend, filterSelectionJson is
		// passed as the filter parameter. The golang extension receives it
		// verbatim as a string.
		res := Search("1", "example", "test",
			&proto.FilterSelection{Selections: map[string]*proto.FilterSelectionValue{
				"type": {Values: []string{"manga"}},
			}},
		)
		assert.Equal(t, 200, res.Code)
		assert.NotNil(t, res.Data)
		assert.Len(t, res.Data, 2)
		assert.Equal(t, "Example Result 1", res.Data[0].Title)
	})

	t.Run("search with multi-field applied filter", func(t *testing.T) {
		// Multiple filter fields applied: {"type":"manga","language":"en"}
		res := Search("1", "example", "test",
			&proto.FilterSelection{Selections: map[string]*proto.FilterSelectionValue{
				"type":     {Values: []string{"manga"}},
				"language": {Values: []string{"en"}},
			}},
		)
		assert.Equal(t, 200, res.Code)
		assert.NotNil(t, res.Data)
		assert.Len(t, res.Data, 2)
	})

	t.Run("search with array-valued filter (multi-select)", func(t *testing.T) {
		// When a filter allows multiple selections, the frontend sends an
		// array: {"type":["manga","bangumi"]}
		res := Search("1", "example", "test",
			&proto.FilterSelection{Selections: map[string]*proto.FilterSelectionValue{
				"type": {Values: []string{"manga", "bangumi"}},
			}},
		)
		assert.Equal(t, 200, res.Code)
		assert.NotNil(t, res.Data)
		assert.Len(t, res.Data, 2)
	})

	t.Run("search page 2", func(t *testing.T) {
		res := Search("2", "example", "test", nil)
		assert.Equal(t, 200, res.Code)
		assert.NotNil(t, res.Data)
		assert.Len(t, res.Data, 2)
	})

	t.Run("search invalid page number", func(t *testing.T) {
		res := Search("invalid", "example", "test", nil)
		assert.NotEqual(t, 200, res.Code)
		assert.Equal(t, "Invalid page number", res.Message)
	})

	t.Run("search unknown package", func(t *testing.T) {
		res := Search("1", "unknown-pkg", "test", nil)
		assert.NotEqual(t, 200, res.Code)
	})
}

// TestHandlerCreateFilter tests the handler-level CreateFilter function.
//
// The frontend selects filter options and sends them via proto.FilterSelection.
// The handler forwards this selection to the extension's CreateFilter function
// and returns the filter definitions.
func TestHandlerCreateFilter(t *testing.T) {
	setExampleExtensionDir(t)

	t.Run("createFilter without selection (initial load)", func(t *testing.T) {
		res := CreateFilter("example", nil)
		assert.Equal(t, 200, res.Code)
		assert.NotNil(t, res.Data)
		// The example extension has "type" and "language" filters, both
		// single-select (select variant of the ExtensionFilter oneof).
		assert.Contains(t, res.Data, "type")
		assert.Contains(t, res.Data, "language")

		typeFilter := decodeFilter(t, res.Data["type"])
		langFilter := decodeFilter(t, res.Data["language"])
		require.NotNil(t, typeFilter.GetSelect())
		require.NotNil(t, langFilter.GetSelect())

		assert.Equal(t, "Type", typeFilter.GetSelect().Title)
		assert.Equal(t, "Language", langFilter.GetSelect().Title)
		assert.Equal(t, "all", typeFilter.GetSelect().Default)
		assert.Equal(t, "en", langFilter.GetSelect().Default)
		assert.Contains(t, typeFilter.GetSelect().Options, "all")
		assert.Contains(t, typeFilter.GetSelect().Options, "manga")
		assert.Contains(t, typeFilter.GetSelect().Options, "bangumi")
	})

	t.Run("createFilter with tapped selection (not yet applied)", func(t *testing.T) {
		// send single-valued selection via proto
		res := CreateFilter("example", &proto.FilterSelection{Selections: map[string]*proto.FilterSelectionValue{
			"type": {Values: []string{"manga"}},
		}})
		assert.Equal(t, 200, res.Code)
		assert.NotNil(t, res.Data)
		assert.Contains(t, res.Data, "type")
		assert.Contains(t, res.Data, "language")
	})

	t.Run("Create_filter with multi-field selection", func(t *testing.T) {
		res := CreateFilter("example", &proto.FilterSelection{Selections: map[string]*proto.FilterSelectionValue{
			"type":     {Values: []string{"manga"}},
			"language": {Values: []string{"en"}},
		}})
		assert.Equal(t, 200, res.Code)
		assert.NotNil(t, res.Data)
		assert.Contains(t, res.Data, "type")
		assert.Contains(t, res.Data, "language")
	})

	t.Run("Create_filter with array-valued selection (multi-select)", func(t *testing.T) {
		res := CreateFilter("example", &proto.FilterSelection{Selections: map[string]*proto.FilterSelectionValue{
			"type": {Values: []string{"manga", "bangumi"}},
		}})
		assert.Equal(t, 200, res.Code)
		assert.NotNil(t, res.Data)
		assert.Contains(t, res.Data, "type")
		assert.Contains(t, res.Data, "language")
	})

	t.Run("Create_filter unknown package", func(t *testing.T) {
		res := CreateFilter("unknown-pkg", nil)
		assert.NotEqual(t, 200, res.Code)
	})
}


// TestHandlerLatest tests the handler-level Latest function, the entry point
// called by the gRPC server when the frontend requests the latest content for
// a package.
func TestHandlerLatest(t *testing.T) {
	setExampleExtensionDir(t)

	t.Run("latest page 1", func(t *testing.T) {
		res := Latest("1", "example")
		assert.Equal(t, 200, res.Code)
		assert.NotNil(t, res.Data)
		assert.Len(t, res.Data, 1)
		assert.Equal(t, "Latest Example 1", res.Data[0].Title)
		assert.Equal(t, "https://example.com/latest/1", res.Data[0].Url)
	})

	t.Run("latest page 2", func(t *testing.T) {
		res := Latest("2", "example")
		assert.Equal(t, 200, res.Code)
		assert.NotNil(t, res.Data)
		assert.Len(t, res.Data, 1)
	})

	t.Run("latest invalid page number", func(t *testing.T) {
		res := Latest("invalid", "example")
		assert.NotEqual(t, 200, res.Code)
		assert.Equal(t, "Invalid page number", res.Message)
	})

	t.Run("latest unknown package", func(t *testing.T) {
		res := Latest("1", "unknown-pkg")
		assert.NotEqual(t, 200, res.Code)
	})
}

