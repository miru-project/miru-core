package js

import (
	"fmt"
	"math"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/eventloop"
	"github.com/dop251/goja_nodejs/require"
	log "github.com/miru-project/miru-core/pkg/logger"
)

// This file implements the cross-call state snapshot for BOTH JS API versions,
// entirely on the Go side of the goja boundary.
//
// The goja VM is disposed after every call (the anti-leak guarantee), so an
// extension's own state has to live outside the VM to survive. The snapshot is
// a STRUCTURAL COPY, not a serialisation: capture walks the live value tree
// from Go and produces a *stateNode tree -- a plain Go structure holding no
// goja references at all -- and rebuild instantiates that tree on a FRESH VM
// using only goja's Go APIs (constructor calls, goja.Compile for functions).
// No JSON, no JS helper functions, nothing round-tripped through a string.
//
// Because a *stateNode tree contains no VM references, the snapshot is fully
// decoupled from the runtime that produced it: it does not keep the old VM
// alive and it can be applied to any new VM.
//
// What survives a recycle, per type:
//
//   - functions        (source captured, recompiled on the new VM)
//   - objects/arrays   (own enumerable string-keyed props, recursively)
//   - getters/setters  (captured as accessors, reinstalled via defineProperty)
//   - Date / RegExp    (epoch ms / source+flags)
//   - Map / Set        (entries iterated with a Go callback)
//   - TypedArrays      (constructor name + element values, incl. BigInt ones)
//   - BigInt           (*big.Int)
//   - primitives       (string, bool, int64/float64 incl NaN, +-Inf, -0)
//   - undefined        (own property whose value is undefined)
//   - circular refs    (re-linked on rebuild; the rebuilt graph is
//     self-consistent)
//
// What cannot survive, by the laws of the VM boundary rather than by choice:
//
//   - closures over per-VM lexical state: a rebuilt function re-binds against
//     the NEW VM's scope. State a function needs must live on the instance /
//     the globals (or in a persistent host channel such as
//     registerSetting/getSetting), all of which outlive the VM.
//   - host/native functions ([native code]): not recordable, skipped at
//     capture.
//   - symbol-keyed properties, WeakMap/WeakSet contents, DataViews: not
//     enumerable / not reconstructible; skipped.
//   - Number/String/Boolean wrapper objects: captured as their primitive
//     value (the wrapper identity is not preserved).
//
// V1 snapshots the EXTENSION INSTANCE (own props of the `ext` object). V2
// snapshots the GLOBALS (own props of globalThis minus a denylist of
// host-owned and base-runtime keys), so console/crypto/Math/JSON and the
// entry-point functions are never recorded and therefore never clobbered on
// restore.

// stateKind enumerates the value shapes the snapshot understands.
type stateKind uint8

const (
	kindUndefined stateKind = iota
	kindNull
	kindBool
	kindInt   // JS integer that exported as int64
	kindFloat // float64, incl NaN / +-Inf / -0
	kindString
	kindBigInt
	kindFunction
	kindAccessor // object getter and/or setter
	kindDate
	kindRegExp
	kindTypedArray
	kindMap
	kindSet
	kindArray
	kindObject
	kindRef // cycle: target points at the node that represents the object
)

// stateNode is one captured value. Exactly the fields matching `kind` are
// populated. The tree holds no goja references, which is what makes the
// snapshot portable across VMs.
type stateNode struct {
	kind stateKind

	b   bool
	i   int64
	f   float64
	s   string
	big *big.Int

	fn    string // function / getter source
	fnSet string // setter source (kindAccessor only; "" = no setter)
	// fnModule names the registered JS module this function was DEFINED in
	// (matched by source containment at capture). A closure cannot be revived
	// from source -- its scope chain dies with the VM -- but the module can be
	// re-evaluated on the new VM and the function re-linked live. Empty means
	// the function is self-contained (or too short/ambiguous to attribute).
	fnModule string

	date    time.Time
	reSrc   string
	reFlags string

	taCtor  string
	taElems []*stateNode

	mapPairs [][2]*stateNode
	setVals  []*stateNode
	items    []*stateNode
	props    map[string]*stateNode

	target *stateNode // kindRef: the node this reference points at
}

// extInstanceState is the per-package snapshot store. Values are *stateNode
// trees; no goja value is retained, so a recorded snapshot never pins a
// disposed VM.
var extInstanceState sync.Map

// saveInstanceState records the snapshot of a package.
func saveInstanceState(pkg string, node *stateNode) {
	extInstanceState.Store(pkg, node)
}

// loadInstanceState returns the recorded snapshot of a package. The second
// return value reports whether any state was recorded.
func loadInstanceState(pkg string) (*stateNode, bool) {
	v, ok := extInstanceState.Load(pkg)
	if !ok {
		return nil, false
	}
	node, ok := v.(*stateNode)
	return node, ok && node != nil
}

// deleteInstanceState drops the recorded snapshot of a package. It is called
// when an extension is reloaded or removed so state from a previous version of
// the source cannot leak into the new one.
func deleteInstanceState(pkg string) {
	extInstanceState.Delete(pkg)
}

// hostGlobalKeys are globalThis properties owned by the HOST, never by the
// extension: the native functions registered by the host, the ECMAScript
// intrinsics whose members are native functions (rebuilding them would produce
// crippled shells), and scalar globals. Recording them would be pointless at
// best and destroying the live object at worst.
var hostGlobalKeys = map[string]bool{
	"console": true, "crypto": true, "zlib": true,
	"require": true, "fetch": true,
	"Request": true, "Response": true,
	"AbortSignal": true, "AbortController": true,
	"URL": true, "URLSearchParams": true,
	"setTimeout": true, "setInterval": true, "setImmediate": true,
	"clearTimeout": true, "clearInterval": true, "clearImmediate": true,
	"registerSetting": true, "getSetting": true, "setSetting": true,
	"getCookies": true, "setCookies": true,
	"btoa": true, "atob": true,
	"jsRequest": true, "__fetch": true,
	"Math": true, "JSON": true, "Reflect": true,
	"Atomics": true, "Intl": true,
	"globalThis": true, "NaN": true, "Infinity": true, "undefined": true,
}

// baseRuntimeGlobals are the V2 entry-point bindings declared by
// assets/runtime_v2.js with `var` / function declarations. They are
// globalThis properties, but the base runtime re-declares every one of them on
// every call, so recording and restoring them is pure noise.
var baseRuntimeGlobals = map[string]bool{
	"latest": true, "search": true, "createFilter": true,
	"detail": true, "watch": true, "mirror": true,
	"checkUpdate": true, "load": true, "throwError": true,
}

// skipGlobal reports whether a globalThis key must never be recorded (V2).
func skipGlobal(k string) bool {
	return hostGlobalKeys[k] || baseRuntimeGlobals[k]
}

// bigTypedArrayCtors need BigInt elements; every other typed array takes
// plain numbers.
var bigTypedArrayCtor = map[string]bool{
	"BigInt64Array": true, "BigUint64Array": true,
}

// shorthandMethodRe matches the toString form of a shorthand or async method
// ("name(args) {...}" / "async name(args) {...}"), which is not a valid
// expression until the `function` keyword is inserted.
var shorthandMethodRe = regexp.MustCompile(`^(async\s+)?([A-Za-z_$][\w$]*)\s*\(`)

// ---------------------------------------------------------------------------
// capture
// ---------------------------------------------------------------------------

// captureCtx carries the per-capture state: the VM being read, the accessor
// helper fetched once, and the object->node table that makes circular
// references re-linkable instead of recursing forever.
type captureCtx struct {
	vm          *goja.Runtime
	seen        map[*goja.Object]*stateNode
	getOwnDesc  goja.Callable // Object.getOwnPropertyDescriptor
}

func newCaptureCtx(vm *goja.Runtime) *captureCtx {
	c := &captureCtx{vm: vm, seen: map[*goja.Object]*stateNode{}}
	if objVal, err := vm.RunString("Object"); err == nil {
		if fn, ok := goja.AssertFunction(objVal.ToObject(vm).Get("getOwnPropertyDescriptor")); ok {
			c.getOwnDesc = fn
		}
	}
	return c
}

// captureValue copies one live JS value into a stateNode tree. It must run on
// the VM's own goroutine (goja is not thread-safe). A nil return means "not
// recordable" (host function); array/map/set callers substitute undefined to
// keep index/entry alignment.
func captureValue(c *captureCtx, v goja.Value) *stateNode {
	switch {
	case v == nil || goja.IsUndefined(v):
		return &stateNode{kind: kindUndefined}
	case goja.IsNull(v):
		return &stateNode{kind: kindNull}
	}

	if o, ok := v.(*goja.Object); ok {
		// A previously visited object: emit a cycle reference instead of
		// recursing. (IsNull must be checked BEFORE this assertion -- goja's
		// null is an *Object whose ClassName() would dereference nil.)
		if existing, seen := c.seen[o]; seen {
			return &stateNode{kind: kindRef, target: existing}
		}
		node := &stateNode{}
		c.seen[o] = node
		c.fillObject(node, o)
		return node
	}

	// Primitives, straight from Export (probe-verified: string, bool, int64,
	// float64 incl NaN/+-Inf/-0, *big.Int).
	switch e := v.Export().(type) {
	case string:
		return &stateNode{kind: kindString, s: e}
	case bool:
		return &stateNode{kind: kindBool, b: e}
	case int64:
		return &stateNode{kind: kindInt, i: e}
	case float64:
		return &stateNode{kind: kindFloat, f: e}
	case *big.Int:
		return &stateNode{kind: kindBigInt, big: new(big.Int).Set(e)}
	}
	// A non-Object JS function should not occur; capture it defensively.
	if _, isFn := goja.AssertFunction(v); isFn {
		src := v.String()
		if strings.Contains(src, "[native code]") {
			return nil
		}
		return &stateNode{kind: kindFunction, fn: src, fnModule: moduleForFunctionSource(src)}
	}
	log.Println("captureValue: unsupported value type", v.ExportType())
	return &stateNode{kind: kindUndefined}
}

// fillObject dispatches on the object's class/constructor and mutates node in
// place to describe it.
func (c *captureCtx) fillObject(node *stateNode, o *goja.Object) {
	if _, isFn := goja.AssertFunction(o); isFn {
		src := o.String()
		if strings.Contains(src, "[native code]") {
			// Host function: cannot be rebuilt on another VM.
			*node = stateNode{kind: kindUndefined}
			return
		}
		*node = stateNode{kind: kindFunction, fn: src, fnModule: moduleForFunctionSource(src)}
		return
	}

	switch o.ClassName() {
	case "Date":
		*node = stateNode{kind: kindDate, date: c.dateValue(o)}
		return
	case "RegExp":
		*node = stateNode{
			kind:    kindRegExp,
			reSrc:   o.Get("source").String(),
			reFlags: o.Get("flags").String(),
		}
		return
	case "Array":
		*node = *c.captureArray(o)
		return
	case "Number", "String", "Boolean":
		// Wrapper objects: keep the primitive value (the wrapper identity is
		// not preserved). Export() on a wrapper yields the primitive.
		switch e := o.Export().(type) {
		case int64:
			*node = stateNode{kind: kindInt, i: e}
		case float64:
			*node = stateNode{kind: kindFloat, f: e}
		case string:
			*node = stateNode{kind: kindString, s: e}
		case bool:
			*node = stateNode{kind: kindBool, b: e}
		default:
			*node = stateNode{kind: kindUndefined}
		}
		return
	}

	// Constructor-name-based shapes: Map, Set, typed arrays; everything else
	// falls through to a plain object.
	ctorName := ""
	if cn := o.Get("constructor"); cn != nil && !goja.IsUndefined(cn) && !goja.IsNull(cn) {
		if co, ok := cn.(*goja.Object); ok {
			ctorName = co.Get("name").String()
		}
	}
	switch ctorName {
	case "Map":
		*node = *c.captureMap(o)
		return
	case "Set":
		*node = *c.captureSet(o)
		return
	}
	if isTypedArrayCtor(ctorName) {
		*node = *c.captureTypedArray(o, ctorName)
		return
	}
	*node = *c.capturePlainObject(o)
}

// dateValue reads a Date as epoch milliseconds via getTime (absolute, so the
// rebuild is exact regardless of timezone). A NaN date (Invalid Date) yields
// NaN, which the caller drops.
func (c *captureCtx) dateValue(o *goja.Object) time.Time {
	fn, ok := goja.AssertFunction(o.Get("getTime"))
	if !ok {
		return time.Time{}
	}
	tv, err := fn(o)
	if err != nil {
		return time.Time{}
	}
	ms := tv.ToFloat()
	if math.IsNaN(ms) {
		return time.Time{}
	}
	return time.UnixMilli(int64(ms))
}

func (c *captureCtx) captureArray(o *goja.Object) *stateNode {
	length := int(o.Get("length").ToInteger())
	items := make([]*stateNode, 0, length)
	for i := 0; i < length; i++ {
		child := captureValue(c, o.Get(strconv.Itoa(i)))
		if child == nil {
			// Keep index alignment for non-recordable elements.
			child = &stateNode{kind: kindUndefined}
		}
		items = append(items, child)
	}
	return &stateNode{kind: kindArray, items: items}
}

// capturePlainObject walks own enumerable string-keyed props. Accessors are
// detected with Object.getOwnPropertyDescriptor so a getter is captured as the
// ACCESSOR (not as a snapshot of whatever it returned at capture time).
func (c *captureCtx) capturePlainObject(o *goja.Object) *stateNode {
	props := make(map[string]*stateNode)
	for _, k := range o.Keys() {
		if c.getOwnDesc != nil {
			desc, err := c.getOwnDesc(goja.Undefined(), o, c.vm.ToValue(k))
			if err == nil && desc != nil && !goja.IsUndefined(desc) {
				if d, ok := desc.(*goja.Object); ok {
					if acc := c.captureAccessor(d); acc != nil {
						props[k] = acc
						continue
					}
				}
			}
		}
		if child := captureValue(c, o.Get(k)); child != nil {
			props[k] = child
		}
	}
	return &stateNode{kind: kindObject, props: props}
}

// captureAccessor turns a property descriptor holding a getter and/or setter
// into a kindAccessor node. A descriptor with neither is a data property.
func (c *captureCtx) captureAccessor(desc *goja.Object) *stateNode {
	getVal := desc.Get("get")
	setVal := desc.Get("set")
	hasGet := getVal != nil && !goja.IsUndefined(getVal) && !goja.IsNull(getVal)
	hasSet := setVal != nil && !goja.IsUndefined(setVal) && !goja.IsNull(setVal)
	if !hasGet && !hasSet {
		return nil
	}
	acc := &stateNode{kind: kindAccessor}
	if hasGet {
		if src := getVal.String(); !strings.Contains(src, "[native code]") {
			acc.fn = src
		}
	}
	if hasSet {
		if src := setVal.String(); !strings.Contains(src, "[native code]") {
			acc.fnSet = src
		}
	}
	if acc.fn == "" && acc.fnSet == "" {
		return nil
	}
	return acc
}

// captureMap iterates entries with a Go callback passed into JS forEach, so
// keys and values stay goja.Values all the way down (Export() would flatten
// nested functions into unusable host callables).
func (c *captureCtx) captureMap(o *goja.Object) *stateNode {
	pairs := [][2]*stateNode{}
	cb := func(call goja.FunctionCall) goja.Value {
		// Map.forEach yields (value, key, map); store as (key, value).
		key := captureValue(c, call.Argument(1))
		val := captureValue(c, call.Argument(0))
		if key == nil {
			key = &stateNode{kind: kindUndefined}
		}
		if val == nil {
			val = &stateNode{kind: kindUndefined}
		}
		pairs = append(pairs, [2]*stateNode{key, val})
		return goja.Undefined()
	}
	if forEach, ok := goja.AssertFunction(o.Get("forEach")); ok {
		_, _ = forEach(o, c.vm.ToValue(cb))
	}
	return &stateNode{kind: kindMap, mapPairs: pairs}
}

func (c *captureCtx) captureSet(o *goja.Object) *stateNode {
	vals := []*stateNode{}
	cb := func(call goja.FunctionCall) goja.Value {
		// Set.forEach yields (value, value, set).
		v := captureValue(c, call.Argument(0))
		if v == nil {
			v = &stateNode{kind: kindUndefined}
		}
		vals = append(vals, v)
		return goja.Undefined()
	}
	if forEach, ok := goja.AssertFunction(o.Get("forEach")); ok {
		_, _ = forEach(o, c.vm.ToValue(cb))
	}
	return &stateNode{kind: kindSet, setVals: vals}
}

func (c *captureCtx) captureTypedArray(o *goja.Object, ctorName string) *stateNode {
	length := int(o.Get("length").ToInteger())
	elems := make([]*stateNode, 0, length)
	for i := 0; i < length; i++ {
		e := captureValue(c, o.Get(strconv.Itoa(i)))
		if e == nil {
			e = &stateNode{kind: kindUndefined}
		}
		elems = append(elems, e)
	}
	return &stateNode{kind: kindTypedArray, taCtor: ctorName, taElems: elems}
}

func isTypedArrayCtor(name string) bool {
	switch name {
	case "Int8Array", "Uint8Array", "Uint8ClampedArray",
		"Int16Array", "Uint16Array", "Int32Array", "Uint32Array",
		"Float32Array", "Float64Array", "BigInt64Array", "BigUint64Array":
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// rebuild
// ---------------------------------------------------------------------------

// rebuildValue instantiates a stateNode tree on the given VM using only goja
// Go APIs. objIDs re-links cycle references: every container is registered
// BEFORE its children are filled, so a reference to an ancestor resolves to
// the live container being built.
func rebuildValue(vm *goja.Runtime, objIDs map[*stateNode]goja.Value, n *stateNode) (goja.Value, error) {
	if n == nil {
		return goja.Undefined(), nil
	}
	switch n.kind {
	case kindUndefined:
		return goja.Undefined(), nil
	case kindNull:
		return goja.Null(), nil
	case kindBool:
		return vm.ToValue(n.b), nil
	case kindInt:
		return vm.ToValue(n.i), nil
	case kindFloat:
		return vm.ToValue(n.f), nil
	case kindString:
		return vm.ToValue(n.s), nil
	case kindBigInt:
		return rebuildBigInt(vm, n.big)
	case kindRef:
		if v, ok := objIDs[n.target]; ok {
			return v, nil
		}
		return goja.Undefined(), nil
	case kindFunction:
		// A function defined by a registered module (e.g. linkedom's parseHTML)
		// closes over module scope that source-recompile cannot recover. Re-link
		// it to a LIVE instance of the module evaluated on this VM instead.
		if n.fnModule != "" {
			if live, ok := relinkFromModule(vm, n.fnModule, n.fn); ok {
				return live, nil
			}
			// Module unavailable on this VM: fall through to source rebuild,
			// which keeps self-contained bodies working and fails at call time
			// only for genuine closures.
		}
		return rebuildFunction(vm, n.fn)
	case kindAccessor:
		// An accessor alone cannot be a value; rebuild the getter (or a
		// no-op function for a setter-only accessor) so a property holding
		// one still carries something callable.
		src := n.fn
		if src == "" {
			src = n.fnSet
		}
		return rebuildFunction(vm, src)
	case kindDate:
		// Date must be constructed with `new`; a plain call returns a string.
		if n.date.IsZero() {
			// Invalid Date was captured as the zero time; rebuild it as
			// `new Date(NaN)`.
			return jsNew(vm, "Date", vm.ToValue(math.NaN()))
		}
		return jsNew(vm, "Date", vm.ToValue(n.date.UnixMilli()))
	case kindRegExp:
		return jsNew(vm, "RegExp", vm.ToValue(n.reSrc), vm.ToValue(n.reFlags))
	case kindTypedArray:
		return rebuildTypedArray(vm, n)
	case kindMap:
		return rebuildMap(vm, objIDs, n)
	case kindSet:
		return rebuildSet(vm, objIDs, n)
	case kindArray:
		return rebuildArray(vm, objIDs, n)
	case kindObject:
		return rebuildObject(vm, objIDs, n)
	}
	return goja.Undefined(), nil
}

// rebuildFunction recompiles captured function source with goja.Compile (no
// eval, no JS helper). Shorthand/async method sources need the `function`
// keyword inserted to become valid expressions.
func rebuildFunction(vm *goja.Runtime, src string) (goja.Value, error) {
	candidates := []string{"(" + src + ")"}
	if m := shorthandMethodRe.FindStringSubmatch(src); m != nil {
		kw := ""
		rest := src
		if m[1] != "" {
			kw = "async "
			rest = src[len(m[1]):]
		}
		candidates = append(candidates, "("+kw+"function "+rest+")")
	}
	for _, cand := range candidates {
		prog, err := goja.Compile("<state-fn>", cand, false)
		if err != nil {
			continue
		}
		if v, err := vm.RunProgram(prog); err == nil {
			return v, nil
		}
	}
	return nil, fmt.Errorf("cannot rebuild function from source %q", src)
}

func rebuildBigInt(vm *goja.Runtime, bi *big.Int) (goja.Value, error) {
	if bi == nil {
		bi = big.NewInt(0)
	}
	ctor, err := jsConstructor(vm, "BigInt")
	if err != nil {
		return nil, err
	}
	// BigInt forbids `new`; it must be plain-called.
	return ctor(goja.Undefined(), vm.ToValue(bi.String()))
}

func rebuildTypedArray(vm *goja.Runtime, n *stateNode) (goja.Value, error) {
	elems := vm.NewArray()
	bigElems := bigTypedArrayCtor[n.taCtor]
	for i, e := range n.taElems {
		var v goja.Value
		if bigElems {
			bv, err := rebuildBigInt(vm, e.big)
			if err != nil {
				return nil, err
			}
			v = bv
		} else {
			switch e.kind {
			case kindInt:
				v = vm.ToValue(e.i)
			case kindFloat:
				v = vm.ToValue(e.f)
			default:
				v = goja.Undefined()
			}
		}
		if err := elems.Set(strconv.Itoa(i), v); err != nil {
			return nil, err
		}
	}
	return jsNew(vm, n.taCtor, elems)
}

func rebuildMap(vm *goja.Runtime, objIDs map[*stateNode]goja.Value, n *stateNode) (goja.Value, error) {
	// Create the empty Map first and register it, so an entry whose value
	// refers back to the map resolves to the live object. Map must be
	// constructed with `new`.
	m, err := jsNew(vm, "Map")
	if err != nil {
		return nil, err
	}
	objIDs[n] = m
	setFn, ok := goja.AssertFunction(m.ToObject(vm).Get("set"))
	if !ok {
		return nil, fmt.Errorf("Map.set is not available")
	}
	for _, pair := range n.mapPairs {
		k, err := rebuildValue(vm, objIDs, pair[0])
		if err != nil {
			return nil, err
		}
		v, err := rebuildValue(vm, objIDs, pair[1])
		if err != nil {
			return nil, err
		}
		if _, err := setFn(m, k, v); err != nil {
			return nil, err
		}
	}
	return m, nil
}

func rebuildSet(vm *goja.Runtime, objIDs map[*stateNode]goja.Value, n *stateNode) (goja.Value, error) {
	// Set must be constructed with `new`.
	s, err := jsNew(vm, "Set")
	if err != nil {
		return nil, err
	}
	objIDs[n] = s
	addFn, ok := goja.AssertFunction(s.ToObject(vm).Get("add"))
	if !ok {
		return nil, fmt.Errorf("Set.add is not available")
	}
	for _, v := range n.setVals {
		rv, err := rebuildValue(vm, objIDs, v)
		if err != nil {
			return nil, err
		}
		if _, err := addFn(s, rv); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func rebuildArray(vm *goja.Runtime, objIDs map[*stateNode]goja.Value, n *stateNode) (goja.Value, error) {
	// Register before filling so cycle refs to this array resolve.
	out := vm.NewArray()
	objIDs[n] = out
	for i, item := range n.items {
		v, err := rebuildValue(vm, objIDs, item)
		if err != nil {
			return nil, err
		}
		if err := out.Set(strconv.Itoa(i), v); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func rebuildObject(vm *goja.Runtime, objIDs map[*stateNode]goja.Value, n *stateNode) (goja.Value, error) {
	out := vm.NewObject()
	objIDs[n] = out
	for k, child := range n.props {
		if child == nil {
			continue
		}
		v, err := rebuildValue(vm, objIDs, child)
		if err != nil {
			return nil, err
		}
		if child.kind == kindAccessor {
			if err := installAccessor(vm, out, k, child, objIDs); err != nil {
				return nil, err
			}
			continue
		}
		if err := out.Set(k, v); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// jsNew constructs a JS value with `new` (the only correct way to build Map,
// Set, TypedArray, Date and RegExp on a real VM -- a plain call throws
// "Constructor X requires 'new'").
func jsNew(vm *goja.Runtime, name string, args ...goja.Value) (goja.Value, error) {
	ctor, err := vm.RunString(name)
	if err != nil {
		return nil, fmt.Errorf("js constructor %s: %w", name, err)
	}
	if _, ok := goja.AssertFunction(ctor); !ok {
		return nil, fmt.Errorf("%s is not a constructor", name)
	}
	o, err := vm.New(ctor, args...)
	if err != nil {
		return nil, fmt.Errorf("new %s: %w", name, err)
	}
	return o, nil
}

func jsConstructor(vm *goja.Runtime, name string) (goja.Callable, error) {
	v, err := vm.RunString(name)
	if err != nil {
		return nil, fmt.Errorf("js constructor %s: %w", name, err)
	}
	ctor, ok := goja.AssertFunction(v)
	if !ok {
		return nil, fmt.Errorf("%s is not a constructor", name)
	}
	return ctor, nil
}

// ---------------------------------------------------------------------------
// per-version entry points (called from AsyncCallBack)
// ---------------------------------------------------------------------------

// captureStateOnVM captures the extension's own state on the VM that just
// served a call. It must run ON the loop (goja is not thread-safe); the
// returned tree holds no VM references and outlives the loop.
//
// V1 reads the own properties of the `ext` instance; V2 reads globalThis
// minus host-owned and base-runtime keys.
func captureStateOnVM(api *ExtApi, vm *goja.Runtime) *stateNode {
	c := newCaptureCtx(vm)
	props := map[string]*stateNode{}

	if api.Ext.ApiVersion == "2" {
		globals := vm.GlobalObject()
		for _, k := range globals.Keys() {
			if skipGlobal(k) {
				continue
			}
			if child := captureValue(c, globals.Get(k)); child != nil {
				props[k] = child
			}
		}
	} else {
		ext := vm.Get("ext")
		if ext == nil || goja.IsUndefined(ext) || goja.IsNull(ext) {
			return nil
		}
		o, ok := ext.(*goja.Object)
		if !ok {
			return nil
		}
		for _, k := range o.Keys() {
			if child := captureValue(c, o.Get(k)); child != nil {
				props[k] = child
			}
		}
	}
	if len(props) == 0 {
		return nil
	}
	return &stateNode{kind: kindObject, props: props}
}

// restoreStateOnVM applies a previously recorded snapshot to the VM that is
// about to serve the next call. It is a no-op when nothing was recorded.
// Failures are logged and skipped: a stale snapshot must never break a call.
func restoreStateOnVM(api *ExtApi, vm *goja.Runtime, root *stateNode) {
	if root == nil || root.kind != kindObject {
		return
	}
	var target *goja.Object
	if api.Ext.ApiVersion == "2" {
		target = vm.GlobalObject()
	} else {
		ext := vm.Get("ext")
		o, ok := ext.(*goja.Object)
		if !ok || goja.IsUndefined(ext) || goja.IsNull(ext) {
			return
		}
		target = o
	}

	objIDs := map[*stateNode]goja.Value{}
	for k, child := range root.props {
		if child == nil {
			continue
		}
		v, err := rebuildValue(vm, objIDs, child)
		if err != nil {
			log.Println("restore extension state:", api.Ext.Pkg, "key", k, err)
			continue
		}
		if child.kind == kindAccessor {
			if err := installAccessor(vm, target, k, child, objIDs); err != nil {
				log.Println("restore extension accessor:", api.Ext.Pkg, k, err)
			}
			continue
		}
		if err := target.Set(k, v); err != nil {
			log.Println("restore extension state:", api.Ext.Pkg, "key", k, err)
		}
	}
}

// installAccessor re-creates a captured getter/setter with
// Object.defineProperty so the property keeps behaving like an accessor
// instead of degrading into a data property.
func installAccessor(vm *goja.Runtime, target *goja.Object, key string, n *stateNode, objIDs map[*stateNode]goja.Value) error {
	objVal, err := vm.RunString("Object")
	if err != nil {
		return err
	}
	defineProp, ok := goja.AssertFunction(objVal.ToObject(vm).Get("defineProperty"))
	if !ok {
		return fmt.Errorf("Object.defineProperty is not available")
	}
	desc := vm.NewObject()
	if n.fn != "" {
		fn, err := rebuildFunction(vm, n.fn)
		if err != nil {
			return err
		}
		if err := desc.Set("get", fn); err != nil {
			return err
		}
	}
	if n.fnSet != "" {
		fn, err := rebuildFunction(vm, n.fnSet)
		if err != nil {
			return err
		}
		if err := desc.Set("set", fn); err != nil {
			return err
		}
	}
	if err := desc.Set("enumerable", true); err != nil {
		return err
	}
	_, err = defineProp(goja.Undefined(), target, vm.ToValue(key), desc)
	return err
}

// relinkFromModule evaluates the named registered module on vm (the shared
// registry caches its compiled program, so this only re-runs the wrapper) and
// returns the EXPORTED function whose source matches exactly -- a live closure
// with its module scope intact, rather than a recompiled husk that would throw
// ReferenceError on its first call.
//
// The exports graph is walked breadth-first including prototype chains (so
// class methods like linkedom's querySelector are reachable), matching by
// Function.prototype.toString equality. require.Require panics on failure, so
// the whole relink is guarded.
func relinkFromModule(vm *goja.Runtime, moduleName, src string) (v goja.Value, ok bool) {
	defer func() {
		if r := recover(); r != nil {
			log.Println("relink module function:", moduleName, r)
			v, ok = nil, false
		}
	}()
	exports := require.Require(vm, moduleName)
	if exports == nil || goja.IsUndefined(exports) || goja.IsNull(exports) {
		return nil, false
	}
	return findFunctionBySource(vm, exports, src, 6, map[*goja.Object]bool{})
}

// findFunctionBySource walks an exports graph (objects, functions and their
// prototypes) looking for a function whose toString() equals src. depth bounds
// the walk; seen guards cycles.
func findFunctionBySource(vm *goja.Runtime, val goja.Value, src string, depth int, seen map[*goja.Object]bool) (goja.Value, bool) {
	if depth <= 0 || val == nil || goja.IsUndefined(val) || goja.IsNull(val) {
		return nil, false
	}
	o, isObj := val.(*goja.Object)
	if !isObj || seen[o] {
		return nil, false
	}
	seen[o] = true

	if _, isFn := goja.AssertFunction(o); isFn {
		if o.String() == src {
			return o, true
		}
		// Class methods live on the prototype; walk it too.
		if proto := o.Get("prototype"); proto != nil {
			if f, ok := findFunctionBySource(vm, proto, src, depth-1, seen); ok {
				return f, true
			}
		}
		return nil, false
	}

	getNames, ok := goja.AssertFunction(vm.Get("Object").ToObject(vm).Get("getOwnPropertyNames"))
	if !ok {
		return nil, false
	}
	namesVal, err := getNames(goja.Undefined(), o)
	if err != nil {
		return nil, false
	}
	list, ok := namesVal.Export().([]interface{})
	if !ok {
		return nil, false
	}
	for _, n := range list {
		name, ok := n.(string)
		if !ok {
			continue
		}
		if f, ok := findFunctionBySource(vm, o.Get(name), src, depth-1, seen); ok {
			return f, true
		}
	}
	return nil, false
}

// runCapture dispatches captureStateOnVM onto the running loop and returns the
// resulting tree. The loop must still be alive (the caller runs this before
// loop.Stop()). Any failure -- including a loop that was already terminated --
// yields nil, which the caller treats as "keep the previous snapshot".
func runCapture(api *ExtApi, loop *eventloop.EventLoop) *stateNode {
	done := make(chan *stateNode, 1)
	if !loop.RunOnLoop(func(vm *goja.Runtime) {
		defer func() {
			if r := recover(); r != nil {
				log.Println("Failed to capture extension state:", api.Ext.Pkg, r)
				done <- nil
			}
		}()
		done <- captureStateOnVM(api, vm)
	}) {
		return nil
	}
	return <-done
}

// finalizeInstanceState captures the extension state at the end of a
// successful call and records it. A call that failed never reaches here, which
// keeps the last good snapshot instead of wiping the extension's state.
func finalizeInstanceState(api *ExtApi, loop *eventloop.EventLoop) {
	if node := runCapture(api, loop); node != nil {
		saveInstanceState(api.Ext.Pkg, node)
	}
}
