package js

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/miru-project/miru-core/proto/generate/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The detail contract for tags: every runtime (JS V1, JS V2, Go/Scriggo) must
// carry the extension's detail `tags` array into proto.ExtensionDetail, so the
// frontend can render the detail-page chips and the DB layer can cache them.
// The Go/Scriggo half is proven in pkg/extension/golang (TestDetailEndpoint).

// TestV1DetailReturnsTags drives the REAL V1 runtime: the class-based fixture's
// detail() returns a tags array, and the host must map it onto the proto field.
func TestV1DetailReturnsTags(t *testing.T) {
	dir := t.TempDir()
	src, err := os.ReadFile(filepath.Join("testdata", v1Fixture))
	require.NoError(t, err)
	require.Contains(t, string(src), "tags:", "the V1 fixture must exercise the tags field")
	require.NoError(t, os.WriteFile(filepath.Join(dir, v1Fixture), src, 0644))

	// The package name is shared with other fixture tests in this binary; a
	// stale cache entry would make the poll below read the wrong extension.
	ApiPkgCache.Remove(v1FixturePkg)
	InitRuntime(dir, AssetsFS)
	waitForPkg(t, v1FixturePkg)

	detail, err := Detail[proto.ExtensionDetail](v1FixturePkg, "https://example.com/1")
	require.NoError(t, err)
	require.NotNil(t, detail)
	assert.Equal(t, []string{"Action", "Adventure"}, detail.Tags,
		"V1 detail must carry the extension's tags array")
}

// TestV2DetailReturnsTags drives the REAL V2 runtime: the function-based
// fixture's detail() returns a tags array, mapped the same way as V1.
func TestV2DetailReturnsTags(t *testing.T) {
	dir := t.TempDir()
	src, err := os.ReadFile(filepath.Join("testdata", "v2watch_example.js"))
	require.NoError(t, err)
	require.Contains(t, string(src), "async function detail", "the V2 fixture must expose a detail entry point")
	// The runtime requires @package + ".js" to equal the file name, and the
	// fixture declares @package v2watch.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "v2watch.js"), src, 0644))

	ApiPkgCache.Remove("v2watch")
	InitRuntime(dir, AssetsFS)
	waitForPkg(t, "v2watch")

	detail, err := Detail[proto.ExtensionDetail]("v2watch", "https://example.com/1")
	require.NoError(t, err)
	require.NotNil(t, detail)
	assert.Equal(t, []string{"Bangumi", "Example"}, detail.Tags,
		"V2 detail must carry the extension's tags array")
}
