package js

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/miru-project/miru-core/pkg/extension"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// v1Fixture is the packaged V1 fixture: a class-based extension with NO
// @apiVersion tag, which is the shape that must always route to V1. All of its
// metadata and URLs use example.com so the fixture carries no third-party
// branding.
const v1Fixture = "v1class.js"

// v1FixturePkg is the package name declared by the v1Fixture header. The JS
// runtime requires @package + ".js" to equal the file name.
const v1FixturePkg = "v1class"

// TestApiVersionMissingTagDefaultsToV1 is the routing guard for the apiVersion
// rule: a .js extension WITHOUT @apiVersion is V1 (the class-based runtime that
// keeps its instance state across calls). Only an explicit @apiVersion 2
// switches to the per-call VM model.
func TestApiVersionMissingTagDefaultsToV1(t *testing.T) {
	dir := t.TempDir()
	src, err := os.ReadFile(filepath.Join("testdata", v1Fixture))
	require.NoError(t, err)
	require.NotContains(t, string(src), "@apiVersion",
		"the fixture must exercise the default: no apiVersion tag")
	require.Contains(t, string(src), "export default class extends Extension",
		"the fixture must be a V1 class-based extension")
	require.NoError(t, os.WriteFile(filepath.Join(dir, v1Fixture), src, 0644))

	InitRuntime(dir, AssetsFS)
	api := waitForPkg(t, v1FixturePkg)
	require.Equal(t, "1", api.Ext.ApiVersion,
		"a .js extension without @apiVersion must be routed to V1")
}

// TestApiVersionExplicitTagTwoOptsIntoV2 pins the explicit opt-in: the tag (any
// surrounding whitespace) is what makes the runtime recycle the VM after
// load().
func TestApiVersionExplicitTagTwoOptsIntoV2(t *testing.T) {
	for _, tag := range []string{"@apiVersion   2", "@apiVersion 2", "@apiVersion 2  "} {
		t.Run(tag, func(t *testing.T) {
			dir := t.TempDir()
			src := "// ==MiruExtension==\n" +
				"// @name         V2 Ext\n" +
				"// @package      v2optin\n" +
				"// " + tag + "\n" +
				"// ==/MiruExtension==\n\n" +
				"async function load() {}\nasync function latest(page) { return []; }\n"
			require.NoError(t, os.WriteFile(filepath.Join(dir, "v2optin.js"), []byte(src), 0644))

			InitRuntime(dir, AssetsFS)
			api := waitForPkg(t, "v2optin")
			require.Equal(t, "2", api.Ext.ApiVersion,
				"%q must be recognised as API v2", tag)
		})
	}
}

// TestApiVersionExplicitTagOneStaysV1 confirms an explicit @apiVersion 1 is
// honoured too, so a stale V1-tagged extension is not silently promoted.
func TestApiVersionExplicitTagOneStaysV1(t *testing.T) {
	dir := t.TempDir()
	src := "// ==MiruExtension==\n" +
		"// @name         V1 Ext\n" +
		"// @package      v1optin\n" +
		"// @apiVersion   1\n" +
		"// ==/MiruExtension==\n\n" +
		"class V1Opt extends Extension { async latest(page) { return []; } }\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "v1optin.js"), []byte(src), 0644))

	InitRuntime(dir, AssetsFS)
	api := waitForPkg(t, "v1optin")
	require.Equal(t, "1", api.Ext.ApiVersion)
}

// TestApiVersionUnrecognisedValueDefaultsToV1 guards the fallback: any value
// that is not "2" (including empty, junk, "v2", "3") is V1.
func TestApiVersionUnrecognisedValueDefaultsToV1(t *testing.T) {
	// The parser itself is the single decision point; assert directly.
	for _, raw := range []string{"", "1", "v2", "V2", "3", "junk", " 2 "} {
		content := "// ==MiruExtension==\n// @name         X\n// @package      x\n// @apiVersion      " +
			raw + "\n// ==/MiruExtension==\n\nclass X extends Extension {}\n"
		ext, err := extension.ParseExtensionMetadata(content, "x.js")
		require.NoError(t, err)
		want := "1"
		if raw == "2" || raw == " 2 " {
			want = "2" // any surrounding whitespace is trimmed
		}
		assert.Equal(t, want, ext.ApiVersion,
			"apiVersion %q must resolve to %s", raw, want)
	}
}
