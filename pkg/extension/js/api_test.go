package js

import (
	"testing"

	"github.com/miru-project/miru-core/pkg/extension"
	"github.com/miru-project/miru-core/proto/generate/proto"
	"github.com/stretchr/testify/assert"
)

func TestGojaExtensionSearch(t *testing.T) {
	ext := &extension.Extension{
		Name:       "Test Extension",
		Pkg:        "test",
		ApiVersion: "1",
		Website:    "https://example.com",
	}

	api := &ExtApi{
		Ext: ext,
		asyncCallBack: func(api *ExtApi, pkg string, evalStr string) (any, error) {
			return []any{
				map[string]any{"title": "Test Result 1", "url": "https://example.com/1"},
				map[string]any{"title": "Test Result 2", "url": "https://example.com/2"},
			}, nil
		},
	}
	ApiPkgCache.Store(ext.Pkg, api)
	ApiPkgCache.SetError(ext.Pkg, "")

	result, err := Search(ext.Pkg, 1, "test", "")
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result, 2)
	assert.Equal(t, "Test Result 1", result[0].Title)
	assert.Equal(t, "Test Result 2", result[1].Title)
}

func TestGojaExtensionLatest(t *testing.T) {
	ext := &extension.Extension{
		Name:       "Test Extension",
		Pkg:        "test_latest",
		ApiVersion: "1",
		Website:    "https://example.com",
	}

	api := &ExtApi{
		Ext: ext,
		asyncCallBack: func(api *ExtApi, pkg string, evalStr string) (any, error) {
			return []any{
				map[string]any{"title": "Latest Test 1", "url": "https://example.com/latest/1"},
			}, nil
		},
	}
	ApiPkgCache.Store(ext.Pkg, api)
	ApiPkgCache.SetError(ext.Pkg, "")

	result, err := Latest[proto.ExtensionListItem](ext.Pkg, 1)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result, 1)
	assert.Equal(t, "Latest Test 1", result[0].Title)
}

func TestGojaExtensionDetail(t *testing.T) {
	ext := &extension.Extension{
		Name:       "Test Extension",
		Pkg:        "test_detail",
		ApiVersion: "1",
		Website:    "https://example.com",
	}

	api := &ExtApi{
		Ext: ext,
		asyncCallBack: func(api *ExtApi, pkg string, evalStr string) (any, error) {
			return map[string]any{
				"title": "Test Detail",
				"url":   "https://example.com/1",
				"desc":  "Test detail description",
			}, nil
		},
	}
	ApiPkgCache.Store(ext.Pkg, api)
	ApiPkgCache.SetError(ext.Pkg, "")

	result, err := Detail(ext.Pkg, "https://example.com/1")
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotNil(t, result.Title)
	assert.Equal(t, "Test Detail", *result.Title)
}

func TestGojaExtensionWatch(t *testing.T) {
	ext := &extension.Extension{
		Name:       "Test Extension",
		Pkg:        "test_watch",
		ApiVersion: "1",
		Website:    "https://example.com",
	}

	api := &ExtApi{
		Ext: ext,
		asyncCallBack: func(api *ExtApi, pkg string, evalStr string) (any, error) {
			return map[string]any{
				"type": "manga",
				"url":  "https://example.com/1",
				"pages": []string{
					"https://example.com/page/1.jpg",
					"https://example.com/page/2.jpg",
				},
			}, nil
		},
	}
	ApiPkgCache.Store(ext.Pkg, api)
	ApiPkgCache.SetError(ext.Pkg, "")

	result, meta, err := Watch(ext.Pkg, "https://example.com/1")
	assert.NoError(t, err)
	assert.NotNil(t, meta)
	assert.NotNil(t, result)
}

func TestGojaExtensionMirror(t *testing.T) {
	ext := &extension.Extension{
		Name:       "Test Extension",
		Pkg:        "test_mirror",
		ApiVersion: "1",
		Website:    "https://example.com",
	}

	api := &ExtApi{
		Ext: ext,
		asyncCallBack: func(api *ExtApi, pkg string, evalStr string) (any, error) {
			return []any{
				map[string]any{"name": "Mirror 1", "url": "https://mirror1.example.com/1"},
				map[string]any{"name": "Mirror 2", "url": "https://mirror2.example.com/1"},
			}, nil
		},
	}
	ApiPkgCache.Store(ext.Pkg, api)
	ApiPkgCache.SetError(ext.Pkg, "")

	result, err := Mirror(ext.Pkg, "https://example.com/1")
	assert.NoError(t, err)
	assert.NotNil(t, result)
}
