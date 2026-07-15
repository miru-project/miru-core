package js

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestJSCrossFunctionCacheRoundTrip verifies the cross-function / cross-call
// variable store that backs Miru.saveCache / Miru.getCache. Because the goja VM
// is disposed after every execution, an extension's long-lived state must live
// OUTSIDE the VM in this pkg -> key -> value string map and survive between
// calls. This mirrors the golang sdk.SaveCache / sdk.GetCache contract and the
// test in the golang package (TestLoadSeedsCacheAndSearchReadsIt).
func TestJSCrossFunctionCacheRoundTrip(t *testing.T) {
	defer deleteExtVarCache("jscache")
	defer deleteExtVarCache("otherpkg")

	// Write once (as a load()-style hook would do) ...
	saveExtVar("jscache", "token", "secret")
	// ... then read it back from a later function call on a fresh VM, exactly
	// the scenario that requires the value to live outside the disposed VM.
	v, ok := getExtVar("jscache", "token")
	assert.True(t, ok)
	assert.Equal(t, "secret", v)

	// Missing keys report false.
	_, ok = getExtVar("jscache", "missing")
	assert.False(t, ok)

	// Different packages are isolated.
	saveExtVar("otherpkg", "token", "other")
	v2, ok := getExtVar("jscache", "token")
	assert.True(t, ok)
	assert.Equal(t, "secret", v2)
	ov, ok := getExtVar("otherpkg", "token")
	assert.True(t, ok)
	assert.Equal(t, "other", ov)

	// Reloading / removing the package drops its cached variables, while other
	// packages remain untouched.
	deleteExtVarCache("jscache")
	_, ok = getExtVar("jscache", "token")
	assert.False(t, ok)
	_, ok = getExtVar("otherpkg", "token")
	assert.True(t, ok)
}
