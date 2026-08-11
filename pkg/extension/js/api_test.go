package js

import (
	"testing"

	"github.com/miru-project/miru-core/proto/generate/proto"
	"github.com/stretchr/testify/assert"
)

func TestGojaExtensionSearch(t *testing.T) {
	registerMockExt(t, "test", "1", func(api *ExtApi, pkg string, evalStr string) (any, error) {
		return []any{
			map[string]any{"title": "Test Result 1", "url": "https://example.com/1"},
			map[string]any{"title": "Test Result 2", "url": "https://example.com/2"},
		}, nil
	})

	result, err := Search("test", 1, "test", nil)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result, 2)
	assert.Equal(t, "Test Result 1", result[0].Title)
	assert.Equal(t, "Test Result 2", result[1].Title)
}

func TestGojaExtensionLatest(t *testing.T) {
	registerMockExt(t, "test_latest", "1", func(api *ExtApi, pkg string, evalStr string) (any, error) {
		return []any{
			map[string]any{"title": "Latest Test 1", "url": "https://example.com/latest/1"},
		}, nil
	})

	result, err := Latest[proto.ExtensionListItem]("test_latest", 1)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result, 1)
	assert.Equal(t, "Latest Test 1", result[0].Title)
}

func TestGojaExtensionDetail(t *testing.T) {
	registerMockExt(t, "test_detail", "1", func(api *ExtApi, pkg string, evalStr string) (any, error) {
		return map[string]any{
			"title": "Test Detail",
			"url":   "https://example.com/1",
			"desc":  "Test detail description",
		}, nil
	})

	result, err := Detail("test_detail", "https://example.com/1")
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotNil(t, result.Title)
	assert.Equal(t, "Test Detail", *result.Title)
}

func TestGojaExtensionWatch(t *testing.T) {
	registerMockExt(t, "test_watch", "1", func(api *ExtApi, pkg string, evalStr string) (any, error) {
		return map[string]any{
			"type": "manga",
			"url":  "https://example.com/1",
			"pages": []string{
				"https://example.com/page/1.jpg",
				"https://example.com/page/2.jpg",
			},
		}, nil
	})

	result, meta, err := Watch("test_watch", "https://example.com/1")
	assert.NoError(t, err)
	assert.NotNil(t, meta)
	assert.NotNil(t, result)
}

// TestGojaExtensionMirror confirms the V1 contract: V1 extensions have NO mirror
// step (watch() returns the link directly), so calling Mirror() on a V1 package
// is rejected. The V2 runtime is the one that supports watch()->mirror().
func TestGojaExtensionMirror(t *testing.T) {
	registerMockExt(t, "test_mirror", "1", func(api *ExtApi, pkg string, evalStr string) (any, error) {
		return []any{
			map[string]any{"name": "Mirror 1", "url": "https://mirror1.example.com/1"},
			map[string]any{"name": "Mirror 2", "url": "https://mirror2.example.com/1"},
		}, nil
	})

	_, err := Mirror("test_mirror", "https://example.com/1")
	assert.Error(t, err, "V1 extensions must not support Mirror()")
}
