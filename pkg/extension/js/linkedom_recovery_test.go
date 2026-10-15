package js

import (
	"testing"

	"encoding/json"
	"os"
	"path/filepath"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/eventloop"
	gojarequire "github.com/dop251/goja_nodejs/require"
	"github.com/miru-project/miru-core/proto/generate/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// This file proves the hardest recovery case the snapshot faces: a CLOSURE from
// a registered module (here linkedom's parseHTML, which closes over the
// module-scope DOMParser and the whole 13k-line bundle scope chain).
//
// A closure's scope chain cannot be rebuilt from its source -- a recompiled
// parseHTML throws ReferenceError: DOMParser is not defined on its first call
// (verified empirically). The only faithful recovery is RE-LINKING: the host
// re-evaluates the module's cached program on the fresh VM and hands back the
// live exported function. This test drives that in the SIMPLEST possible
// environment -- two bare event-loop VMs, no InitRuntime, no extension files,
// no runtime boot -- and the crucial ordering the contract demands:
//
//	the PARSING happens at VM2, on the recovered function, not at VM1.
//
// The test itself never calls require('linkedom') at VM2; the only require in
// the whole file runs at VM1 to obtain the function the way an extension would.
// Recovery at VM2 is entirely the host snapshot machinery's job.

// linkedomRegistry boots a minimal environment with linkedom registered -- the
// same registration production performs in initModule(), minus everything else.
func linkedomRegistry(t *testing.T) *gojarequire.Registry {
	t.Helper()
	sharedRegistry = gojarequire.NewRegistry()
	lnk, err := AssetsFS.ReadFile("assets/linkedom/worker.js")
	require.NoError(t, err, "bundled linkedom asset must exist")
	RegisterJSModule("linkedom", string(lnk), func(vm *goja.Runtime, module *goja.Object) {})
	return sharedRegistry
}

// TestLinkedomClosureRecoveredAtVM2 is the simple-environment recovery test.
func TestLinkedomClosureRecoveredAtVM2(t *testing.T) {
	reg := linkedomRegistry(t)
	_ = reg
	extAPI := &ExtApi{Ext: &Ext{
		Name:       "Linkedom Recovery",
		Pkg:        "linkedom.test",
		ApiVersion: "1",
		Website:    "https://example.com",
	}}

	// ---- VM1: the way an extension obtains the library, then state capture.
	var snap *stateNode
	loop1 := eventloop.NewEventLoop(eventloop.WithRegistry(sharedRegistry))
	loop1.Run(func(vm *goja.Runtime) {
		sharedRegistry.Enable(vm)
		if _, err := vm.RunString(`
			const {parseHTML} = require('linkedom');
			ext = { parseIt: parseHTML };
		`); err != nil {
			t.Fatalf("vm1 setup: %v", err)
		}
		snap = captureStateOnVM(extAPI, vm)
	})
	loop1.Stop() // VM1 is gone; its scope chain (and DOMParser) died with it.

	require.NotNil(t, snap, "the extension state must be captured")
	parseNode := snap.props["parseIt"]
	require.NotNil(t, parseNode, "parseIt must be in the snapshot")
	require.Equal(t, kindFunction, parseNode.kind)
	assert.Equal(t, "linkedom", parseNode.fnModule,
		"the snapshot must attribute the closure to its defining module; "+
			"without this the function can only be recompiled and would die on "+
			"its first call with ReferenceError")

	// ---- VM2: fresh VM. The test's own JS never requires anything here.
	// parseHTML must PARSE at VM2 and the result must answer querySelector.
	loop2 := eventloop.NewEventLoop(eventloop.WithRegistry(sharedRegistry))
	loop2.Run(func(vm *goja.Runtime) {
		// Mirror the host's per-call VM setup: the registry is enabled so the
		// snapshot machinery can re-link, but no module is required by test code.
		sharedRegistry.Enable(vm)
		if _, err := vm.RunString(`ext = {}`); err != nil {
			t.Fatalf("vm2 ext init: %v", err)
		}
		restoreStateOnVM(extAPI, vm, snap)

		v, err := vm.RunString(`(function () {
			if (typeof ext.parseIt !== 'function') return 'not-a-function';
			// The PARSING happens here, at VM2, on the recovered function.
			const doc = ext.parseIt('<html><body>' +
				'<p id="a">hello</p>' +
				'<ul><li class="item">one</li><li class="item">two</li></ul>' +
				'</body></html>').document;
			const a = doc.querySelector('#a');
			return JSON.stringify({
				text: a.textContent,
				items: doc.querySelectorAll('.item').length,
				second: doc.querySelectorAll('.item')[1].textContent,
				tag: a.tagName,
			});
		})()`)
		if err != nil {
			t.Fatalf("recovered parseHTML failed at VM2: %v", err)
		}
		var got struct {
			Text  string `json:"text"`
			Items int    `json:"items"`
			Second string `json:"second"`
			Tag   string `json:"tag"`
		}
		mustJSON(t, v.String(), &got)
		assert.Equal(t, "hello", got.Text,
			"the recovered closure must parse at VM2 and querySelector must work")
		assert.Equal(t, 2, got.Items, "querySelectorAll must work on the VM2-parsed document")
		assert.Equal(t, "two", got.Second)
		assert.Equal(t, "P", got.Tag)
	})
	loop2.Stop()
}

// TestLinkedomClosureRecoveredV2Globals is the same recovery driven through the
// V2 capture shape (globalThis instead of the ext instance).
func TestLinkedomClosureRecoveredV2Globals(t *testing.T) {
	linkedomRegistry(t)
	extAPI := &ExtApi{Ext: &Ext{
		Name:       "Linkedom V2",
		Pkg:        "linkedom.v2.test",
		ApiVersion: "2",
		Website:    "https://example.com",
	}}

	var snap *stateNode
	loop1 := eventloop.NewEventLoop(eventloop.WithRegistry(sharedRegistry))
	loop1.Run(func(vm *goja.Runtime) {
		sharedRegistry.Enable(vm)
		if _, err := vm.RunString(`
			const {parseHTML} = require('linkedom');
			globalThis.lib = { parseIt: parseHTML };
		`); err != nil {
			t.Fatalf("vm1 setup: %v", err)
		}
		snap = captureStateOnVM(extAPI, vm)
	})
	loop1.Stop()

	require.NotNil(t, snap)
	libNode := snap.props["lib"]
	require.NotNil(t, libNode, "the lib object must be captured")
	require.Equal(t, kindObject, libNode.kind)
	parseNode := libNode.props["parseIt"]
	require.NotNil(t, parseNode)
	assert.Equal(t, "linkedom", parseNode.fnModule,
		"module attribution must work through nested V2 global objects")

	loop2 := eventloop.NewEventLoop(eventloop.WithRegistry(sharedRegistry))
	loop2.Run(func(vm *goja.Runtime) {
		sharedRegistry.Enable(vm)
		restoreStateOnVM(extAPI, vm, snap)
		v, err := vm.RunString(`(function () {
			const doc = lib.parseIt('<html><body><h1 id="t">title</h1></body></html>').document;
			return doc.querySelector('#t').textContent;
		})()`)
		if err != nil {
			t.Fatalf("recovered V2-global parseHTML failed at VM2: %v", err)
		}
		assert.Equal(t, "title", v.String())
	})
	loop2.Stop()
}

// TestLinkedomRecoverySurvivesFullRuntime is the END-TO-END version: a real V1
// extension stores parseHTML on its instance in load(), and a LATER call (fresh
// VM, recycled) parses with it and runs querySelector. This is the production
// AsyncCallBack path, not the bare-VM harness.
func TestLinkedomRecoverySurvivesFullRuntime(t *testing.T) {
	dir := t.TempDir()
	src := "// ==MiruExtension==\n" +
		"// @name         Linkedom Ext\n" +
		"// @package      v1linkedom\n" +
		"// ==/MiruExtension==\n" +
		"export default class extends Extension {\n" +
		"    async load() {\n" +
		"        const { parseHTML } = require(\"linkedom\");\n" +
		"        this.parseIt = parseHTML;\n" +
		"    }\n" +
		"    async latest(page) {\n" +
		"        const doc = this.parseIt('<html><body><p id=\"x\">recovered</p></body></html>').document;\n" +
		"        const el = doc.querySelector('#x');\n" +
		"        return [{ title: el.textContent, url: \"https://example.com\" }];\n" +
		"    }\n" +
		"}\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "v1linkedom.js"), []byte(src), 0644))

	ApiPkgCache.Remove("v1linkedom")
	deleteInstanceState("v1linkedom")

	InitRuntime(dir, AssetsFS)
	api := waitForPkg(t, "v1linkedom")
	require.Equal(t, "1", api.Ext.ApiVersion)
	require.Empty(t, api.Ext.Error, "extension must load cleanly")

	// latest() runs on a FRESH VM whose only link to linkedom is the snapshot
	// taken when load() ran on its own VM. The parse happens on this fresh VM.
	items, err := Latest[proto.ExtensionListItem]("v1linkedom", 1)
	require.NoError(t, err, "a stored module closure must be callable from a later call")
	require.Len(t, items, 1)
	assert.Equal(t, "recovered", items[0].Title,
		"the parse+querySelector must run at the recycled VM via the recovered closure")

	// And again: every call is a new VM, so the recovery must be repeatable.
	items2, err := Latest[proto.ExtensionListItem]("v1linkedom", 1)
	require.NoError(t, err)
	require.Len(t, items2, 1)
	assert.Equal(t, "recovered", items2[0].Title)
}

func mustJSON(t *testing.T, s string, out any) {
	t.Helper()
	require.NoError(t, json.Unmarshal([]byte(s), out), "probe result must be JSON: %s", s)
}
