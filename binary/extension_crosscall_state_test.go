package binary

import (
	"os"
	"path/filepath"
	"testing"

	jsext "github.com/miru-project/miru-core/pkg/extension/js"
	"github.com/miru-project/miru-core/proto/generate/proto"
	"github.com/stretchr/testify/assert"
)

// crossCallStateExtension is a minimal V2 JS extension that parks a value on
// its globals from load() and reads it back from latest(). Because the goja VM
// is disposed after every call, the host's state snapshot (instance_state.go)
// is the only way state survives between load() and latest(); this exercises
// that path end to end. settingKeys / Miru are already declared by
// runtime_v2.js, so the extension must not redeclare them.
const crossCallStateExtension = `// ==MiruExtension==
// @name         Cross Call State Test
// @package      crosscallstate
// @apiVersion   2
// ==/MiruExtension==

async function load() {
    globalThis.token = "secret";
}

async function latest(page) {
    return [{ title: globalThis.token, url: "https://example.com" }];
}
`

// TestJSCrossCallStateRoundTrip proves that a value written to the extension's
// globals in load() (executed on one fresh goja VM) is still readable from
// latest() (executed on a different fresh goja VM) via the state snapshot. It
// is the JavaScript counterpart of golang's TestLoadSeedsCacheAndSearchReadsIt
// test.
func TestJSCrossCallStateRoundTrip(t *testing.T) {
	dir := t.TempDir()
	jsPath := filepath.Join(dir, "crosscallstate.js")
	if err := os.WriteFile(jsPath, []byte(crossCallStateExtension), 0644); err != nil {
		t.Fatalf("write crosscallstate.js: %v", err)
	}

	jsext.InitRuntime(dir, jsext.AssetsFS)

	requireLoaded(t, "crosscallstate")

	results, err := jsext.Latest[proto.ExtensionListItem]("crosscallstate", 1)
	assert.NoError(t, err)
	assert.NotNil(t, results)
	assert.Len(t, results, 1, "latest() should return the snapshotted value as a single item")
	assert.Equal(t, "secret", results[0].Title)
}
