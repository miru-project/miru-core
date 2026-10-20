package runtime

import (
	"encoding/json"
	"reflect"
	"strings"
	"sync"
)

// extVarCache is the cross-call state store of the Go (Scriggo) extension
// runtime. Every public endpoint call (Search / Latest / Detail / ...) runs a
// FRESH VM, so any state an extension keeps in its package-level variables
// must live OUTSIDE the VM between calls -- here.
//
// The store is keyed by package name then key:
//
//	map[pkg]map[key]value
//
// matching the layout used by the JavaScript runtime.
//
// There is NO author-facing cache API: the host captures every package-level
// variable after each call and restores it before the next one (see
// pkg/extension/golang/state_capture.go, which instruments the entry points to
// bracket calls with MiruStateGet / MiruStatePut). DeleteCache drops a
// package's captured state when its source changes or the extension is
// removed, so stale values never leak into a new version.
//
// Type identity note: a type DECLARED in the extension source gets a fresh
// identity on every compile (a fresh compile is a fresh package, exactly like
// real Go). The automatic capture sidesteps this by round-tripping every
// variable as JSON into a freshly declared local of the (re)compiled type.
var extVarCache sync.Map

// isScriggoProxy reports whether v is the internal wrapper Scriggo puts around
// values of script-declared types when they cross into native code. The state
// capture never stores or decodes such wrappers: the generated bridge code
// marshals through a freshly declared LOCAL variable, whose address always
// reaches the host unwrapped.
func isScriggoProxy(v any) bool {
	if v == nil {
		return false
	}
	t := reflect.TypeOf(v)
	return t.Kind() == reflect.Struct &&
		t.Name() == "emptyInterfaceProxy" &&
		strings.HasSuffix(t.PkgPath(), "/scriggo/internal/compiler/types")
}

// MiruStateGet decodes the automatically captured value of the package
// variable named key into dst (a pointer). It is called by the generated state
// bridge at the START of every extension entry point; false means the variable
// has no persisted value yet (first call) and keeps its initializer.
func MiruStateGet(pkg, key string, dst any) bool {
	m, ok := extVarCache.Load(pkg)
	if !ok {
		return false
	}
	raw, ok := m.(*sync.Map).Load(key)
	if !ok {
		return false
	}
	data, isStr := raw.(string)
	if !isStr {
		return false
	}
	if err := json.Unmarshal([]byte(data), dst); err != nil {
		// A variable persisted by an older extension version that no longer
		// fits the current type: drop it and keep the initializer.
		m.(*sync.Map).Delete(key)
		return false
	}
	return true
}

// MiruStatePut captures the package variable addressed by ptr as JSON. It is
// called by the generated state bridge at the END of every extension entry
// point (before each return, so a failed call captures what it wrote before
// failing -- the same guarantee the JavaScript runtime gives). Kinds JSON
// cannot represent (function values, channels) are skipped silently: capture
// is best-effort by design and must never break a call.
func MiruStatePut(pkg, key string, ptr any) {
	if ptr == nil || isScriggoProxy(ptr) {
		return
	}
	data, err := json.Marshal(ptr)
	if err != nil {
		return
	}
	m, _ := extVarCache.LoadOrStore(pkg, &sync.Map{})
	m.(*sync.Map).Store(key, string(data))
}

// DeleteCache drops every captured variable for a package. It is called when
// an extension source changes or the extension is removed, so stale values
// from a previous version do not leak into the next compile.
func DeleteCache(pkg string) {
	extVarCache.Delete(pkg)
}
