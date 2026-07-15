package js

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/miru-project/miru-core/proto/generate/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// v1AsyncExtension is a V1 (class-based) extension whose load() and latest()
// both do real asynchronous work (await a setTimeout) and prove it actually ran
// by writing the awaited result through Miru.saveCache. The event loop must
// await these promises -- if it did not, the values would never be written and
// the calls would return instantly instead of after the delay.
const v1AsyncExtension = `// ==MiruExtension==
// @name         V1 Async Test
// @package      v1async
// @apiVersion   1
// ==/MiruExtension==

class V1Async extends Extension {
    async load() {
        await new Promise(r => setTimeout(r, 60));
        Miru.saveCache("loadRan", "yes");
    }
    async latest(page) {
        const start = Date.now();
        await new Promise(r => setTimeout(r, 60));
        const loadRan = Miru.getCache("loadRan");
        return [{ title: loadRan, url: "https://example.com/" + (Date.now() - start) }];
    }
}
`

// v2AsyncExtension is the V2 (top-level async function) equivalent. It must go
// through the very same event loop / await machinery as V1, so a regression in
// the event loop that breaks async would be caught on either version.
const v2AsyncExtension = `// ==MiruExtension==
// @name         V2 Async Test
// @package      v2async
// @apiVersion   2
// ==/MiruExtension==

async function load() {
    await new Promise(r => setTimeout(r, 60));
    Miru.saveCache("loadRan", "yes");
}

async function latest(page) {
    const start = Date.now();
    await new Promise(r => setTimeout(r, 60));
    const loadRan = Miru.getCache("loadRan");
    return [{ title: loadRan, url: "https://example.com/" + (Date.now() - start) }];
}
`

// TestJSAsyncAwaitRunsOnEventLoopV1AndV2 proves, for BOTH the V1 and V2
// JavaScript runtimes, that asynchronous code (await + setTimeout) actually
// executes inside the shared event loop and is properly awaited by the host:
//
//  1. The first latest() call (which compiles and runs on a FRESH goja VM,
//     while load() ran on another FRESH VM earlier) takes at least the awaited
//     delay to complete. If the event loop did NOT await the promise, latest()
//     would return instantly in ~0ms instead -- the elapsed time is the
//     time-based calling validation.
//  2. load() also ran asynchronously (it awaited its own setTimeout) and,
//     because the goja VM is disposed after every call, communicated the result
//     through the cross-function cache (Miru.saveCache / Miru.getCache). latest()
//     reads that value back, proving load() was awaited too.
//
// Both runtimes share one AsyncCallBack / handlePromise / await implementation,
// so this single test guards async correctness for V1 and V2 alike.
func TestJSAsyncAwaitRunsOnEventLoopV1AndV2(t *testing.T) {
	cases := []struct {
		name string
		pkg  string
		src  string
	}{
		{"V1", "v1async", v1AsyncExtension},
		{"V2", "v2async", v2AsyncExtension},
	}

	dir := t.TempDir()
	for _, c := range cases {
		require.NoError(t, os.WriteFile(filepath.Join(dir, c.pkg+".js"), []byte(c.src), 0644))
	}

	// One shared InitRuntime boots both extensions through the real event loop.
	InitRuntime(dir, AssetsFS)
	// Give the init event-loop bootstrap (which runs each load() hook
	// asynchronously) a moment to finish before we send traffic.
	time.Sleep(500 * time.Millisecond)

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			api := ApiPkgCache.Load(c.pkg)
			require.NotNil(t, api, "the %s extension should be registered after InitRuntime", c.name)
			assert.Empty(t, api.Ext.Error, "the %s extension should load without an error", c.name)

			// First latest(): runs on its own fresh VM, awaits a setTimeout.
			// The elapsed time must include that await -- this is the
			// time-based proof that async actually runs on the event loop.
			start := time.Now()
			results, err := Latest[proto.ExtensionListItem](c.pkg, 1)
			elapsed := time.Since(start)
			require.NoError(t, err, "latest() must not error for %s", c.name)
			require.Len(t, results, 1, "latest() should return one item for %s", c.name)

			assert.GreaterOrEqual(t, elapsed.Milliseconds(), int64(50),
				"%s latest() should take at least the awaited delay (async ran on the event loop), got %s",
				c.name, elapsed)

			// load() was awaited on its own fresh VM and wrote its result to
			// the cross-function cache; latest() read it back, confirming both
			// hooks execute their async work under the same event loop.
			assert.Equal(t, "yes", results[0].Title,
				"%s latest() should observe the async result seeded by load()", c.name)

			// Second latest(): still a fresh VM, still must await its own
			// setTimeout (the VM is disposed & recreated each call).
			start2 := time.Now()
			r2, err2 := Latest[proto.ExtensionListItem](c.pkg, 1)
			elapsed2 := time.Since(start2)
			require.NoError(t, err2, "second latest() must not error for %s", c.name)
			assert.GreaterOrEqual(t, elapsed2.Milliseconds(), int64(50),
				"%s second latest() should still await on the recreated VM, got %s", c.name, elapsed2)
			assert.Equal(t, "yes", r2[0].Title)
		})
	}
}
