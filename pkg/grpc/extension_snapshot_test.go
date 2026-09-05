package grpc

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/miru-project/miru-core/pkg/event"
	"github.com/miru-project/miru-core/pkg/extension"
	golang "github.com/miru-project/miru-core/pkg/extension/golang"
	js "github.com/miru-project/miru-core/pkg/extension/js"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeGoExtension drops a minimal but valid Scriggo extension into dir so
// golang.GetExtensions() can discover it from disk.
func writeGoExtension(t *testing.T, dir, pkg string) {
	t.Helper()
	src := `// ==MiruExtension==
// @name         ` + pkg + `
// @package      ` + pkg + `
// @apiVersion   1
// ==/MiruExtension==

package example

func Load() {}
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, pkg+".go"), []byte(src), 0644))
}

// wireUpdateCallbacks installs the production wiring and restores the previous
// callbacks afterwards. Calling wireExtensionUpdateCallbacks (rather than
// reproducing it here) means the test guards the real startup path.
func wireUpdateCallbacks(t *testing.T) {
	t.Helper()
	jsPrev, golangPrev := js.OnExtensionUpdate, golang.OnExtensionUpdate
	t.Cleanup(func() {
		js.OnExtensionUpdate, golang.OnExtensionUpdate = jsPrev, golangPrev
	})
	wireExtensionUpdateCallbacks()
}

// nextExtensionUpdate returns the next ExtensionUpdate event on the bus,
// failing the test if none arrives.
func nextExtensionUpdate(t *testing.T, ch chan event.Event) event.Event {
	t.Helper()
	for {
		select {
		case ev := <-ch:
			if ev.Type == event.ExtensionUpdate {
				return ev
			}
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for an ExtensionUpdate event")
		}
	}
}

func snapshotPackages(t *testing.T, data any) map[string]bool {
	t.Helper()
	exts, ok := data.([]*js.Ext)
	require.True(t, ok, "payload must be the merged []*js.Ext snapshot, got %T", data)
	pkgs := map[string]bool{}
	for _, e := range exts {
		pkgs[e.Pkg] = true
	}
	return pkgs
}

// TestJSExtensionChangeKeepsGoExtensions is the regression test for Go
// extensions vanishing from the frontend UI.
//
// The frontend replaces its entire extension list whenever an ExtensionUpdate
// event arrives. The event used to carry the JS runtime's own cache, so any
// JS-side change — installing a JS extension, a hot reload, a lazy load on
// first use, or an extension merely reporting an error — pushed a JS-only list
// and silently dropped every Go/Scriggo extension until the app restarted.
func TestJSExtensionChangeKeepsGoExtensions(t *testing.T) {
	dir := t.TempDir()
	writeGoExtension(t, dir, "example.v2")

	prevDir := golang.ExtensionDir
	golang.ExtensionDir = dir
	t.Cleanup(func() { golang.ExtensionDir = prevDir })

	wireUpdateCallbacks(t)

	ch := event.GlobalBus.Subscribe()
	defer event.GlobalBus.Unsubscribe(ch)

	// Adding a JS extension fires the JS runtime's notify().
	jsPkg := "examplejs"
	js.ApiPkgCache.Store(jsPkg, &js.ExtApi{Ext: &extension.Extension{Name: "ExampleJS", Pkg: jsPkg}})
	t.Cleanup(func() { js.ApiPkgCache.Remove(jsPkg) })

	pkgs := snapshotPackages(t, nextExtensionUpdate(t, ch).Data)
	assert.True(t, pkgs["example.v2"], "a JS-side change must not drop the Go extension")
	assert.True(t, pkgs[jsPkg], "the JS extension itself must be present")
}

// TestGoExtensionChangePublishesSnapshot covers the mirror case: a Go extension
// changing on disk must also reach the frontend, which previously had no Go
// notify hook at all.
func TestGoExtensionChangePublishesSnapshot(t *testing.T) {
	dir := t.TempDir()
	writeGoExtension(t, dir, "example.v2")

	prevDir := golang.ExtensionDir
	golang.ExtensionDir = dir
	t.Cleanup(func() { golang.ExtensionDir = prevDir })

	wireUpdateCallbacks(t)

	ch := event.GlobalBus.Subscribe()
	defer event.GlobalBus.Unsubscribe(ch)

	golang.HandleReload("example.v2")

	pkgs := snapshotPackages(t, nextExtensionUpdate(t, ch).Data)
	assert.True(t, pkgs["example.v2"], "a Go extension change must publish a snapshot")
}

// TestToProtoExtensionMetaMapsGoFields guards the shared converter: both
// HelloMiru and the event path publish through it, so a field dropped here is
// invisible everywhere.
func TestToProtoExtensionMetaMapsGoFields(t *testing.T) {
	in := []*js.Ext{{
		Name:       "Nyaa",
		Pkg:        "nyaa.go",
		Version:    "1.2.3",
		ApiVersion: "2",
		WatchType:  "bangumi",
		Lang:       "en",
	}}

	out := toProtoExtensionMeta(in)

	require.Len(t, out, 1)
	assert.Equal(t, "Nyaa", out[0].Name)
	assert.Equal(t, "nyaa.go", out[0].Package)
	assert.Equal(t, "1.2.3", out[0].Version)
	assert.Equal(t, "2", out[0].Api)
	assert.Equal(t, "bangumi", out[0].Type)
	assert.Equal(t, "en", out[0].Lang)
}
