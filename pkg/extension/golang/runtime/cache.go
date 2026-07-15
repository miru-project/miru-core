package runtime

import "sync"

// extVarCache is the cross-function / cross-call variable store shared by all
// functions of a Go (Scriggo) extension. Because every public endpoint call
// (Search / Latest / Detail / ...) compiles and runs a FRESH Scriggo VM and the
// startup LoadExtension does NOT retain its Runtime, any state an extension
// wants to share between its functions must live OUTSIDE the VM -- here.
//
// The store is keyed by package name then variable key:
//
//	map[pkg]map[key]value
//
// matching the layout used by the JavaScript runtime. Values are stored as
// `any`; because they are plain Go values (no VM involved) they can be any
// serializable Go type. Avoid storing values that would "break" the Scriggo VM
// when passed back into a function call -- e.g. function values, channels tied
// to a goroutine that has exited, or anything holding a reference to a disposed
// runtime. Stick to strings, numbers, slices, maps and structs.
var extVarCache sync.Map

// SaveCache stores value under (pkg, key) in the shared cross-function store.
func SaveCache(pkg, key string, value any) {
	m, _ := extVarCache.LoadOrStore(pkg, &sync.Map{})
	m.(*sync.Map).Store(key, value)
}

// GetCache reads the value stored under (pkg, key). The second return value
// reports whether the key was present.
func GetCache(pkg, key string) (any, bool) {
	m, ok := extVarCache.Load(pkg)
	if !ok {
		return nil, false
	}
	v, ok := m.(*sync.Map).Load(key)
	if !ok {
		return nil, false
	}
	return v, true
}

// DeleteCache drops every cached variable for a package. It is called when an
// extension is removed so stale values from a previous version do not leak.
func DeleteCache(pkg string) {
	extVarCache.Delete(pkg)
}
