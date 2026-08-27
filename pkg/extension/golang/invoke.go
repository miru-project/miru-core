package golang

import (
	"fmt"
	"os"
	"path/filepath"
)

// readExtensionPkgSource reads the source of an extension package from disk.
func readExtensionPkgSource(pkg string) ([]byte, error) {
	extPath := filepath.Join(ExtensionDir, pkg+".go")
	return os.ReadFile(extPath)
}

// callExtension compiles an extension package and invokes the named function by
// name using the local Scriggo build that supports calling a function by name.
//
// The extension source is compiled as-is: the package declaration, any imports
// and the // ==MiruExtension== metadata header are all valid Go that the
// compiler handles, so there is no need to strip anything. The first return
// value of the called function (index 0) is returned; if the function also
// returns an error (index 1) it is propagated to the caller.
//
// The whole call is guarded with recover: the Scriggo Call path is reflective,
// and an extension entry point whose declared signature does not accept the
// host's arguments (for example `filter string` instead of `sdk.Filter`)
// panics inside reflect. Without this guard that panic would unwind through
// the c-shared boundary and kill the whole core process; with it, the failure
// becomes a normal error that travels back over gRPC to the UI.
//
// Every call compiles its own VM and program (see below), so recovering here
// cannot leave shared interpreter state corrupted.
func callExtension(pkg, fn string, args ...any) (result any, err error) {
	vm := NewScriggoVM(nil)
	src, err := readExtensionPkgSource(pkg)
	if err != nil {
		return nil, err
	}

	prog, err := vm.Compile(pkg, pkg+"_"+fn, string(src))
	if err != nil {
		return nil, fmt.Errorf("compile extension %s: %w", pkg, err)
	}

	defer func() {
		if r := recover(); r != nil {
			result, err = nil, withStackTrace(
				"extension "+pkg+"."+fn,
				fmt.Errorf("calling %s panicked: %v", fn, r),
			)
		}
	}()

	res, err := prog.program.Call(fn, args...)
	if err != nil {
		return nil, withStackTrace("extension "+pkg+"."+fn, err)
	}
	if len(res) >= 2 {
		if e, ok := res[1].(error); ok && e != nil {
			return nil, e
		}
	}
	if len(res) == 0 {
		return nil, nil
	}
	return res[0], nil
}

