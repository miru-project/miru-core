package js

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/miru-project/miru-core/pkg/extension"
	"github.com/miru-project/miru-core/proto/generate/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestJSV2WatchReturnsMirrorList drives the V2 example extension (v2watch) through
// the REAL runtime (InitRuntime + event loop) and asserts that:
//
//  1. watch() returns a proto.ExtensionWatch carrying the mirror groups (NOT the
//     final link) -- the V2 contract.
//  2. mirror() then resolves the chosen mirror into the final per-type watch
//     (here a bangumi { type, url, headers } object).
//
// This proves JS V2 and Go V2 share the same watch()->mirror() contract.
func TestJSV2WatchReturnsMirrorList(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("extensions", "v2watch_example.js"))
	require.NoError(t, err)

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "v2watch.js"), src, 0644))

	InitRuntime(dir, AssetsFS)
	// Let the async init (load hooks) settle before sending traffic.
	time.Sleep(500 * time.Millisecond)

	api := ApiPkgCache.Load("v2watch")
	require.NotNil(t, api, "the v2watch extension should be registered after InitRuntime")
	assert.Empty(t, api.Ext.Error, "the v2watch extension should load without an error")
	require.Equal(t, "2", api.Ext.ApiVersion, "the example must be API v2")
	require.Equal(t, extension.WatchTypeBangumi, api.Ext.WatchType, "the example must be @type bangumi")

	// --- watch() => mirror list (proto.ExtensionWatch) ---
	res, meta, err := Watch("v2watch", "https://example.com/detail/1")
	require.NoError(t, err)
	require.NotNil(t, meta)
	require.NotNil(t, res)

	watch, ok := res.(*proto.ExtensionWatch)
	require.True(t, ok, "V2 watch() must return *proto.ExtensionWatch, got %T", res)
	require.Len(t, watch.Groups, 2, "watch() should expose both mirror groups")

	// Groups come back sorted by title for determinism ("Server 1" before
	// "Server 2").
	assert.Equal(t, "Server 1", watch.Groups[0].Title)
	require.Len(t, watch.Groups[0].Mirrors, 2)
	assert.Equal(t, "Mirror 1", watch.Groups[0].Mirrors[0].Name)
	assert.Equal(t, "https://mirror1.example.com/stream.m3u8", watch.Groups[0].Mirrors[0].Url)

	assert.Equal(t, "Server 2", watch.Groups[1].Title)
	require.Len(t, watch.Groups[1].Mirrors, 1)
	mirrorURL := watch.Groups[0].Mirrors[0].Url

	// --- mirror(url) => final per-type watch (bangumi) ---
	mres, err := Mirror("v2watch", mirrorURL)
	require.NoError(t, err)
	require.NotNil(t, mres)

	bangumi, ok := mres.(map[string]any)
	require.True(t, ok, "V2 mirror() should return the per-type watch object, got %T", mres)
	assert.Equal(t, "hls", bangumi["type"])
	assert.Equal(t, mirrorURL, bangumi["url"])
	require.True(t, ok, "mirror() should carry headers")
}

// TestJSV2WatchObjectGroupsForm exercises toJSV2Watch directly with the object
// form { groups: { title: [mirrors] } } produced by the example extension, so
// the conversion is covered without booting the full runtime.
func TestJSV2WatchObjectGroupsForm(t *testing.T) {
	raw := map[string]any{
		"groups": map[string]any{
			"Server 1": []any{
				map[string]any{"name": "Mirror 1", "url": "https://m1.test/x", "headers": map[string]any{"Referer": "https://r.test/"}},
			},
			"Server 2": []any{
				map[string]any{"name": "Backup", "url": "https://bk.test/x"},
			},
		},
	}

	watch := toJSV2Watch(raw)
	require.Len(t, watch.Groups, 2)
	// Deterministic order: "Server 1" < "Server 2".
	assert.Equal(t, "Server 1", watch.Groups[0].Title)
	require.Len(t, watch.Groups[0].Mirrors, 1)
	assert.Equal(t, "Mirror 1", watch.Groups[0].Mirrors[0].Name)
	assert.Equal(t, "https://m1.test/x", watch.Groups[0].Mirrors[0].Url)
	assert.Equal(t, "https://r.test/", watch.Groups[0].Mirrors[0].Headers["Referer"])

	assert.Equal(t, "Server 2", watch.Groups[1].Title)
	require.Len(t, watch.Groups[1].Mirrors, 1)
	assert.Equal(t, "Backup", watch.Groups[1].Mirrors[0].Name)
}

// TestJSV2WatchArrayGroupsForm exercises toJSV2Watch with the array form
// [{ title, mirrors }] and the bare-mirror fallback.
func TestJSV2WatchArrayGroupsForm(t *testing.T) {
	// Array form.
	arr := map[string]any{
		"groups": []any{
			map[string]any{
				"title":   "Group A",
				"mirrors": []any{map[string]any{"name": "M", "url": "https://a.test/1"}},
			},
		},
	}
	watch := toJSV2Watch(arr)
	require.Len(t, watch.Groups, 1)
	assert.Equal(t, "Group A", watch.Groups[0].Title)
	require.Len(t, watch.Groups[0].Mirrors, 1)

	// Bare-mirror fallback (top-level { name, url }).
	bare := map[string]any{"name": "Solo", "url": "https://solo.test/1"}
	watch = toJSV2Watch(bare)
	require.Len(t, watch.Groups, 1)
	require.Len(t, watch.Groups[0].Mirrors, 1)
	assert.Equal(t, "Solo", watch.Groups[0].Mirrors[0].Name)
}

// TestJSV1MirrorRejected confirms that the V1 runtime has no mirror step:
// calling Mirror() on a V1 package returns an error rather than resolving a
// link. (V1 watch() returns the link directly.)
func TestJSV1MirrorRejected(t *testing.T) {
	registerMockExt(t, "test_v1_mirror", "1", func(api *ExtApi, pkg string, evalStr string) (any, error) {
		return "https://example.com/link", nil
	})

	_, err := Mirror("test_v1_mirror", "https://example.com/link")
	assert.Error(t, err, "V1 extension must not support Mirror()")
}
