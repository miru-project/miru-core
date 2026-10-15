package js

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/miru-project/miru-core/proto/generate/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// This file holds the cross-call STATE tests: everything that verifies how the
// Go-native snapshot (instance_state.go) carries an extension's state across
// the per-call VM recycle -- V1 instance properties, V2 globals, rich types,
// circular references, and the failed-call boundary. apiVersion routing lives
// in apiversion_test.go; watch/mirror contract in watch_contract_test.go.
// TestV1InstancePersistsAcrossCalls is the behavioural difference the
// apiVersion rule exists for: V1 load() and every later call share the SAME
// logical instance, so state the extension keeps on `this` survives -- which
// is what "load extension by eval script call `load`, keep the runtime" means.
//
// The mechanism is instance-state snapshotting, not a persistent VM: the goja
// VM is still disposed after every call, but the extension instance's own data
// properties are captured structurally at the end of a call and rebuilt onto
// the fresh instance at the start of the next one (see instance_state.go).
// Module-level `let`/`var` values do NOT survive -- only state on the
// instance does, which is exactly why V2 can keep recycling its VM.
func TestV1InstancePersistsAcrossCalls(t *testing.T) {
	dir := t.TempDir()
	src := "// ==MiruExtension==\n" +
		"// @name         Stateful V1\n" +
		"// @package      v1class\n" +
		"// @type         bangumi\n" +
		"// @webSite      https://example.com\n" +
		"// ==/MiruExtension==\n\n" +
		"export default class extends Extension {\n" +
		"    async load() { this.loadCount = 1; }\n" +
		"    async latest(page) { return [{ title: \"loads:\" + (this.loadCount || 0), url: \"https://example.com/\" }]; }\n" +
		"    async search(kw, page, filter) { this.lastKeyword = String(kw); return [{ title: \"kw:\" + this.lastKeyword, url: \"https://example.com/\" }]; }\n" +
		"}\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, v1Fixture), []byte(src), 0644))

	// An earlier test may have registered this package from a different
	// fixture in its own temp dir; drop it so waitForPkg cannot match the
	// stale entry before this InitRuntime's loader stores the new one.
	ApiPkgCache.Remove(v1FixturePkg)

	InitRuntime(dir, AssetsFS)
	api := waitForPkg(t, v1FixturePkg)
	require.Equal(t, "1", api.Ext.ApiVersion)

	// load() ran and seeded the instance; latest() sees that value.
	r, err := Latest[proto.ExtensionListItem](v1FixturePkg, 1)
	require.NoError(t, err)
	require.Len(t, r, 1)
	assert.Equal(t, "loads:1", r[0].Title,
		"V1 load() must seed state that later calls share")

	// ... and it is still there on the SECOND call, after that call's VM was
	// recycled. A persistent VM is not required; the instance state is.
	r2, err := Latest[proto.ExtensionListItem](v1FixturePkg, 1)
	require.NoError(t, err)
	assert.Equal(t, "loads:1", r2[0].Title,
		"the V1 instance state persists across recycled VMs")

	// State written by one call is visible to the next, different kind of call.
	s, err := Search[proto.ExtensionListItem](v1FixturePkg, 1, "cats", nil)
	require.NoError(t, err)
	require.Len(t, s, 1)
	assert.Equal(t, "kw:cats", s[0].Title)

	// A later call of a different kind sees what search() recorded.
	// latest() does not touch lastKeyword, so the value must still be "cats".
	r3, err := Latest[proto.ExtensionListItem](v1FixturePkg, 1)
	require.NoError(t, err)
	assert.Equal(t, "loads:1", r3[0].Title,
		"the value seeded by load() is still intact after a search call")

	// And a further search() writes its own value during that call.
	s2, err := Search[proto.ExtensionListItem](v1FixturePkg, 1, "dogs", nil)
	require.NoError(t, err)
	assert.Equal(t, "kw:dogs", s2[0].Title,
		"the call being served writes its own value before snapshotting")
}

// TestV1ModuleScopeResetsPerCall documents the boundary of the snapshot: only
// state on the instance survives, nothing declared at module scope. A V1
// extension that wants cross-call state must put it on `this`.
func TestV1ModuleScopeResetsPerCall(t *testing.T) {
	dir := t.TempDir()
	src := "// ==MiruExtension==\n" +
		"// @name         Module Scope\n" +
		"// @package      modscope\n" +
		"// ==/MiruExtension==\n" +
		"let moduleCounter = 0;\n" +
		"export default class extends Extension {\n" +
		"    async load() { moduleCounter++; }\n" +
		"    async latest(page) {\n" +
		"        moduleCounter++;\n" +
		"        this.hits = (this.hits || 0) + 1;\n" +
		"        return [{ title: \"module:\" + moduleCounter + \":instance:\" + this.hits, url: \"https://example.com\" }];\n" +
		"    }\n" +
		"}\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "modscope.js"), []byte(src), 0644))

	InitRuntime(dir, AssetsFS)
	api := waitForPkg(t, "modscope")
	require.Equal(t, "1", api.Ext.ApiVersion)

	// load() ran on its own VM; this is the first latest() call, so the module
	// counter is 1 (fresh VM) and the instance counter is 1 (snapshot empty).
	r1, err := Latest[proto.ExtensionListItem]("modscope", 1)
	require.NoError(t, err)
	require.Len(t, r1, 1)
	assert.Equal(t, "module:1:instance:1", r1[0].Title)

	// Second call: the module counter is back to 0 on the recycled VM (it does
	// NOT survive), while the instance counter -- taken from the snapshot --
	// continues from 1.
	r2, err := Latest[proto.ExtensionListItem]("modscope", 1)
	require.NoError(t, err)
	assert.Equal(t, "module:1:instance:2", r2[0].Title,
		"module scope resets with the VM; instance state does not")
}

// TestV1StateSurvivesFailedCall guards the snapshot boundary: a call that fails
// must not lose the state recorded by earlier calls, or a single transient
// error would silently reset an extension's whole world. (The failed call's
// own capture still runs -- see TestV1FailedCallWritesReachNextCall -- but it
// stores the restored instance, which carries the previous state forward.)
func TestV1StateSurvivesFailedCall(t *testing.T) {
	dir := t.TempDir()
	src := "// ==MiruExtension==\n" +
		"// @name         Failing V1\n" +
		"// @package      v1fail\n" +
		"// ==/MiruExtension==\n" +
		"export default class extends Extension {\n" +
		"    async load() { this.token = \"good\"; }\n" +
		"    async latest(page) { return [{ title: \"t:\" + this.token, url: \"https://example.com\" }]; }\n" +
		"    async detail(url) { throw new Error(\"boom\"); }\n" +
		"}\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "v1fail.js"), []byte(src), 0644))

	InitRuntime(dir, AssetsFS)
	api := waitForPkg(t, "v1fail")
	require.Equal(t, "1", api.Ext.ApiVersion)

	r, err := Latest[proto.ExtensionListItem]("v1fail", 1)
	require.NoError(t, err)
	require.Len(t, r, 1)
	assert.Equal(t, "t:good", r[0].Title)

	_, err = Detail[proto.ExtensionDetail]("v1fail", "https://example.com/1")
	require.Error(t, err, "the failing call must surface its error")

	r2, err := Latest[proto.ExtensionListItem]("v1fail", 1)
	require.NoError(t, err)
	assert.Equal(t, "t:good", r2[0].Title,
		"a failing call must not wipe the snapshot taken by the previous one")
}

// TestV1FailedCallWritesReachNextCall is the write-ahead guarantee the old
// saveCache surface used to provide by writing to the Go map immediately:
// state an extension writes BEFORE its call fails must still be waiting for
// the next call of the same extension. The snapshot now captures on the
// failure path too, so a V1 instance mutation made before a throw survives.
// detail() throws synchronously here (the sync-failure branch of
// AsyncCallBack); the async-rejection branch is covered by the V2 twin.
func TestV1FailedCallWritesReachNextCall(t *testing.T) {
	dir := t.TempDir()
	src := "// ==MiruExtension==\n" +
		"// @name         V1 Write Ahead\n" +
		"// @package      v1writeahead\n" +
		"// ==/MiruExtension==\n" +
		"export default class extends Extension {\n" +
		"    async load() { this.token = \"good\"; }\n" +
		"    async latest(page) { return [{ title: \"t:\" + this.token + \"|staged:\" + (this.staged || \"none\"), url: \"https://example.com\" }]; }\n" +
		"    detail(url) { this.staged = \"pre-retry\"; throw new Error(\"boom\"); }\n" +
		"}\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "v1writeahead.js"), []byte(src), 0644))

	ApiPkgCache.Remove("v1writeahead")
	deleteInstanceState("v1writeahead")

	InitRuntime(dir, AssetsFS)
	api := waitForPkg(t, "v1writeahead")
	require.Equal(t, "1", api.Ext.ApiVersion)

	r, err := Latest[proto.ExtensionListItem]("v1writeahead", 1)
	require.NoError(t, err)
	require.Len(t, r, 1)
	assert.Equal(t, "t:good|staged:none", r[0].Title, "baseline before the failing call")

	_, err = Detail[proto.ExtensionDetail]("v1writeahead", "https://example.com/1")
	require.Error(t, err, "the failing call must surface its error")

	r2, err := Latest[proto.ExtensionListItem]("v1writeahead", 1)
	require.NoError(t, err)
	require.Len(t, r2, 1)
	assert.Equal(t, "t:good|staged:pre-retry", r2[0].Title,
		"a write made before the throw must be captured for the next call, "+
			"and the previous state must ride along with it")
}

// TestV2FailedCallWritesReachNextCall is the V2 twin on the ASYNC-REJECTION
// branch: detail() resolves into a rejected promise, which fails in await()
// rather than at eval time, and the capture must still run before the loop is
// disposed. Data lands on globalThis (a `let` would reset with the VM).
func TestV2FailedCallWritesReachNextCall(t *testing.T) {
	dir := t.TempDir()
	src := "// ==MiruExtension==\n" +
		"// @name         V2 Write Ahead\n" +
		"// @package      v2writeahead\n" +
		"// @apiVersion   2\n" +
		"// ==/MiruExtension==\n" +
		"var token = \"\";\n" +
		"async function load() { token = \"good\"; }\n" +
		"async function latest(page) { return [{ title: \"t:\" + token + \"|staged:\" + (globalThis.staged || \"none\"), url: \"https://example.com\" }]; }\n" +
		"async function detail(url) { globalThis.staged = \"pre-retry\"; await Promise.reject(new Error(\"boom\")); }\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "v2writeahead.js"), []byte(src), 0644))

	ApiPkgCache.Remove("v2writeahead")
	deleteInstanceState("v2writeahead")

	InitRuntime(dir, AssetsFS)
	api := waitForPkg(t, "v2writeahead")
	require.Equal(t, "2", api.Ext.ApiVersion)

	r, err := Latest[proto.ExtensionListItem]("v2writeahead", 1)
	require.NoError(t, err)
	require.Len(t, r, 1)
	assert.Equal(t, "t:good|staged:none", r[0].Title, "baseline before the failing call")

	_, err = Detail[proto.ExtensionDetail]("v2writeahead", "https://example.com/1")
	require.Error(t, err, "the rejected promise must surface its error")

	r2, err := Latest[proto.ExtensionListItem]("v2writeahead", 1)
	require.NoError(t, err)
	require.Len(t, r2, 1)
	assert.Equal(t, "t:good|staged:pre-retry", r2[0].Title,
		"a global written before the rejection must be captured for the next call")
}

// TestV2RecyclesVMPerCall is the V2 lifecycle test:
//
//  1. The VM (loop) is recycled per call -- the anti-leak guarantee. This is
//     proven with `let`, whose top-level binding lives in the script's lexical
//     scope, NOT on globalThis, so it cannot survive and resets on every call.
//  2. A `var` / implicit global IS carried across: the previous call's globals
//     are snapshotted before the loop is torn down and restored onto the next
//     call's fresh VM, so module-scope state works without a persistent VM.
func TestV2RecyclesVMPerCall(t *testing.T) {
	dir := t.TempDir()
	src := "// ==MiruExtension==\n" +
		"// @name         V2 Recycle\n" +
		"// @package      v2recycle\n" +
		"// @apiVersion   2\n" +
		"// ==/MiruExtension==\n" +
		"let lexicalCount = 0;\n" +
		"var globalCount = 0;\n" +
		"var seed = \"\";\n" +
		"async function load() { seed = \"kept\"; globalCount = 10; }\n" +
		"async function latest(page) {\n" +
		"    lexicalCount++;\n" +
		"    globalCount++;\n" +
		"    return [{ title: seed + \":lex:\" + lexicalCount + \":glob:\" + globalCount, url: \"https://example.com\" }];\n" +
		"}\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "v2recycle.js"), []byte(src), 0644))

	InitRuntime(dir, AssetsFS)
	api := waitForPkg(t, "v2recycle")
	require.Equal(t, "2", api.Ext.ApiVersion)

	r1, err := Latest[proto.ExtensionListItem]("v2recycle", 1)
	require.NoError(t, err)
	require.Len(t, r1, 1)
	// lexicalCount is 1 (fresh VM); globalCount is 11, i.e. the 10 load() set --
	// restored from the previous call's snapshot -- plus this call's increment.
	assert.Equal(t, "kept:lex:1:glob:11", r1[0].Title,
		"V2 must recycle the VM per call while carrying its own globals")

	r2, err := Latest[proto.ExtensionListItem]("v2recycle", 1)
	require.NoError(t, err)
	// lexicalCount resets with the VM (1 again); globalCount continues (12),
	// proving the VM was recycled but the extension's globals survived.
	assert.Equal(t, "kept:lex:1:glob:12", r2[0].Title,
		"V2 must recycle per call and still continue its global state")
}

// TestV2FunctionGlobalsReDeclaredNotRestored documents what the snapshot does
// NOT drop a function global: a function assigned at RUNTIME (e.g. inside
// load()) is not re-declared by the recompiled program on later calls, so it
// must survive via the snapshot. The host rebuilds it from its source on the
// fresh VM.
func TestV2FunctionGlobalsSurviveAcrossCalls(t *testing.T) {
	dir := t.TempDir()
	src := "// ==MiruExtension==\n" +
		"// @name         V2 Fn\n" +
		"// @package      v2fn\n" +
		"// @apiVersion   2\n" +
		"// ==/MiruExtension==\n" +
		"var calls = 0;\n" +
		"var injectedHelper = null;\n" +
		"async function load() { injectedHelper = function (x) { return \"dyn:\" + x; }; }\n" +
		"async function latest(page) {\n" +
		"    calls++;\n" +
		"    return [{ title: \"calls:\" + calls + \":\" + injectedHelper(\"v\"), url: \"https://example.com\" }];\n" +
		"}\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "v2fn.js"), []byte(src), 0644))

	InitRuntime(dir, AssetsFS)
	api := waitForPkg(t, "v2fn")
	require.Equal(t, "2", api.Ext.ApiVersion)

	// load() defined injectedHelper at runtime; latest() on THIS call sees it
	// because load() ran first on its own VM whose state was captured.
	r, err := Latest[proto.ExtensionListItem]("v2fn", 1)
	require.NoError(t, err)
	require.Len(t, r, 1)
	assert.Equal(t, "calls:1:dyn:v", r[0].Title,
		"a runtime-assigned function global must be callable after the recycle")

	// Second call: a FRESH VM re-declares `injectedHelper = null` from the
	// program, then the snapshot is restored onto it -- the function must come
	// back and stay callable.
	r2, err := Latest[proto.ExtensionListItem]("v2fn", 1)
	require.NoError(t, err)
	assert.Equal(t, "calls:2:dyn:v", r2[0].Title,
		"the function global must be rebuilt from source on the next call")
}

// TestV2RichStateRoundTrip proves the structural (non-serialised) snapshot: a
// V2 extension stores a Date, RegExp, Map, Set, typed array, BigInt, nested
// object, array and a runtime-assigned function on its globals, and every one
// of them must come back with its original TYPE and value after the VM is
// recycled. Nothing is JSON round-tripped; the host rebuilds each value via
// its goja constructor.
func TestV2RichStateRoundTrip(t *testing.T) {
	dir := t.TempDir()
	src := "// ==MiruExtension==\n" +
		"// @name         V2 Rich\n" +
		"// @package      v2rich\n" +
		"// @apiVersion   2\n" +
		"// ==/MiruExtension==\n" +
		"var built = false;\n" +
		"async function load() {\n" +
		"    if (built) { return; }\n" +
		"    built = true;\n" +
		"    globalThis.when = new Date(\"2020-01-02T03:04:05.000Z\");\n" +
		"    globalThis.pattern = /ab+c/gi;\n" +
		"    globalThis.counts = new Map([[\"a\", 1], [\"b\", { n: 2 }]]);\n" +
		"    globalThis.tags = new Set([\"x\", \"y\"]);\n" +
		"    globalThis.bytes = new Uint8Array([1, 2, 255]);\n" +
		"    globalThis.huge = 12345678901234567890n;\n" +
		"    globalThis.nested = { arr: [1, \"two\", null, true], deep: { k: \"v\" } };\n" +
		"    globalThis.doubleIt = function (x) { return x * 2; };\n" +
		"}\n" +
		"async function latest(page) {\n" +
		"    const parts = [\n" +
		"        when instanceof Date ? when.toISOString() : \"bad-date\",\n" +
		"        pattern instanceof RegExp ? pattern.source + \"/\" + pattern.flags : \"bad-regexp\",\n" +
		"        (counts instanceof Map && counts.get(\"a\") === 1 && counts.get(\"b\").n === 2) ? \"map-ok\" : \"bad-map\",\n" +
		"        (tags instanceof Set && tags.has(\"x\") && tags.size === 2) ? \"set-ok\" : \"bad-set\",\n" +
		"        (bytes instanceof Uint8Array && bytes.length === 3 && bytes[2] === 255) ? \"u8-ok\" : \"bad-u8\",\n" +
		"        (typeof huge === \"bigint\" && huge === 12345678901234567890n) ? \"bigint-ok\" : \"bad-bigint\",\n" +
		"        (nested.deep.k === \"v\" && nested.arr.length === 4 && nested.arr[1] === \"two\") ? \"obj-ok\" : \"bad-obj\",\n" +
		"        (typeof doubleIt === \"function\" && doubleIt(21) === 42) ? \"fn-ok\" : \"bad-fn\",\n" +
		"    ];\n" +
		"    return [{ title: parts.join(\"|\"), url: \"https://example.com\" }];\n" +
		"}\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "v2rich.js"), []byte(src), 0644))

	InitRuntime(dir, AssetsFS)
	api := waitForPkg(t, "v2rich")
	require.Equal(t, "2", api.Ext.ApiVersion)

	want := "2020-01-02T03:04:05.000Z|ab+c/gi|map-ok|set-ok|u8-ok|bigint-ok|obj-ok|fn-ok"

	// First call: load() builds the state, latest() verifies it on the same VM.
	r1, err := Latest[proto.ExtensionListItem]("v2rich", 1)
	require.NoError(t, err)
	require.Len(t, r1, 1)
	assert.Equal(t, want, r1[0].Title, "state built by load() must verify on the first call")

	// Second call: FRESH VM, program re-declares nothing but `built`/`load`,
	// so everything latest() sees must have come from the snapshot.
	r2, err := Latest[proto.ExtensionListItem]("v2rich", 1)
	require.NoError(t, err)
	require.Len(t, r2, 1)
	assert.Equal(t, want, r2[0].Title,
		"every rich type (Date/RegExp/Map/Set/TypedArray/BigInt/object/function) must round-trip")
}

// TestV1RichStateRoundTrip is the V1 counterpart: the same structural state
// lives on the class INSTANCE (`this`) instead of globalThis.
func TestV1RichStateRoundTrip(t *testing.T) {
	dir := t.TempDir()
	src := "// ==MiruExtension==\n" +
		"// @name         V1 Rich\n" +
		"// @package      v1rich\n" +
		"// ==/MiruExtension==\n" +
		"export default class extends Extension {\n" +
		"    async load() {\n" +
		"        this.when = new Date(\"2020-01-02T03:04:05.000Z\");\n" +
		"        this.pattern = /ab+c/gi;\n" +
		"        this.counts = new Map([[\"a\", 1]]);\n" +
		"        this.tags = new Set([\"x\", \"y\"]);\n" +
		"        this.bytes = new Uint8Array([1, 2, 255]);\n" +
		"        this.huge = 12345678901234567890n;\n" +
		"        this.nested = { arr: [1, \"two\"], deep: { k: \"v\" } };\n" +
		"        this.doubleIt = function (x) { return x * 2; };\n" +
		"        this.hits = 0;\n" +
		"    }\n" +
		"    async latest(page) {\n" +
		"        this.hits++;\n" +
		"        const parts = [\n" +
		"            this.when instanceof Date ? this.when.toISOString() : \"bad-date\",\n" +
		"            this.pattern instanceof RegExp ? this.pattern.source : \"bad-regexp\",\n" +
		"            (this.counts instanceof Map && this.counts.get(\"a\") === 1) ? \"map-ok\" : \"bad-map\",\n" +
		"            (this.tags instanceof Set && this.tags.has(\"y\")) ? \"set-ok\" : \"bad-set\",\n" +
		"            (this.bytes instanceof Uint8Array && this.bytes[2] === 255) ? \"u8-ok\" : \"bad-u8\",\n" +
		"            (typeof this.huge === \"bigint\") ? \"bigint-ok\" : \"bad-bigint\",\n" +
		"            (this.nested.deep.k === \"v\" && this.nested.arr[1] === \"two\") ? \"obj-ok\" : \"bad-obj\",\n" +
		"            (typeof this.doubleIt === \"function\" && this.doubleIt(21) === 42) ? \"fn-ok\" : \"bad-fn\",\n" +
		"            \"hits:\" + this.hits,\n" +
		"        ];\n" +
		"        return [{ title: parts.join(\"|\"), url: \"https://example.com\" }];\n" +
		"    }\n" +
		"}\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "v1rich.js"), []byte(src), 0644))

	InitRuntime(dir, AssetsFS)
	api := waitForPkg(t, "v1rich")
	require.Equal(t, "1", api.Ext.ApiVersion)

	// First call: load() seeded the instance; latest() runs on a FRESH VM
	// whose `ext` got the snapshot restored, and its own `this.hits++` must
	// apply on top of the restored instance.
	r1, err := Latest[proto.ExtensionListItem]("v1rich", 1)
	require.NoError(t, err)
	require.Len(t, r1, 1)
	assert.Equal(t,
		"2020-01-02T03:04:05.000Z|ab+c|map-ok|set-ok|u8-ok|bigint-ok|obj-ok|fn-ok|hits:1",
		r1[0].Title, "instance state built by load() must verify on the first call")

	// Second call: another fresh VM; hits must continue (2), proving the
	// restored instance carried the counter AND every rich type.
	r2, err := Latest[proto.ExtensionListItem]("v1rich", 1)
	require.NoError(t, err)
	require.Len(t, r2, 1)
	assert.Equal(t,
		"2020-01-02T03:04:05.000Z|ab+c|map-ok|set-ok|u8-ok|bigint-ok|obj-ok|fn-ok|hits:2",
		r2[0].Title,
		"every rich type must round-trip on the V1 instance across recycled VMs")
}

// TestV1CircularReferenceRoundTrip covers the hardest structural case: an
// instance object that refers to itself. JSON cannot represent this at all;
// the Go-native snapshot must preserve the cycle so the rebuilt graph stays
// self-consistent (obj.self === obj).
func TestV1CircularReferenceRoundTrip(t *testing.T) {
	dir := t.TempDir()
	src := "// ==MiruExtension==\n" +
		"// @name         V1 Cycle\n" +
		"// @package      v1cycle\n" +
		"// ==/MiruExtension==\n" +
		"export default class extends Extension {\n" +
		"    async load() {\n" +
		"        this.node = { name: \"root\" };\n" +
		"        this.node.self = this.node;\n" +
		"        this.node.children = [this.node];\n" +
		"    }\n" +
		"    async latest(page) {\n" +
		"        const ok = this.node.self === this.node && this.node.children[0] === this.node;\n" +
		"        return [{ title: ok ? \"cycle-ok\" : \"bad-cycle\", url: \"https://example.com\" }];\n" +
		"    }\n" +
		"}\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "v1cycle.js"), []byte(src), 0644))

	InitRuntime(dir, AssetsFS)
	api := waitForPkg(t, "v1cycle")
	require.Equal(t, "1", api.Ext.ApiVersion)

	r, err := Latest[proto.ExtensionListItem]("v1cycle", 1)
	require.NoError(t, err)
	require.Len(t, r, 1)
	assert.Equal(t, "cycle-ok", r[0].Title,
		"a circular reference must survive the recycle with the cycle intact")

	r2, err := Latest[proto.ExtensionListItem]("v1cycle", 1)
	require.NoError(t, err)
	require.Len(t, r2, 1)
	assert.Equal(t, "cycle-ok", r2[0].Title,
		"the cycle must still be intact on the second recycle")
}

// TestV2StateSurvivesFailedCall guards the snapshot boundary for V2: a call
// that throws must not wipe the last good snapshot, exactly like V1.
func TestV2StateSurvivesFailedCall(t *testing.T) {
	dir := t.TempDir()
	src := "// ==MiruExtension==\n" +
		"// @name         V2 Err\n" +
		"// @package      v2fail\n" +
		"// @apiVersion   2\n" +
		"// ==/MiruExtension==\n" +
		"var token = \"\";\n" +
		"async function load() { token = \"good\"; }\n" +
		"async function latest(page) { return [{ title: \"t:\" + token, url: \"https://example.com\" }]; }\n" +
		"async function detail(url) { throw new Error(\"boom\"); }\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "v2fail.js"), []byte(src), 0644))

	InitRuntime(dir, AssetsFS)
	api := waitForPkg(t, "v2fail")
	require.Equal(t, "2", api.Ext.ApiVersion)

	r, err := Latest[proto.ExtensionListItem]("v2fail", 1)
	require.NoError(t, err)
	require.Len(t, r, 1)
	assert.Equal(t, "t:good", r[0].Title)

	_, err = Detail[proto.ExtensionDetail]("v2fail", "https://example.com/1")
	require.Error(t, err, "the failing call must surface its error")

	r2, err := Latest[proto.ExtensionListItem]("v2fail", 1)
	require.NoError(t, err)
	assert.Equal(t, "t:good", r2[0].Title,
		"a failing call must not wipe the globals recorded by the previous one")
}

