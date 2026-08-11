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
func callExtension(pkg, fn string, args ...any) (any, error) {
	vm := NewScriggoVM(nil)
	src, err := readExtensionPkgSource(pkg)
	if err != nil {
		return nil, err
	}

	prog, err := vm.Compile(pkg, pkg+"_"+fn, string(src))
	if err != nil {
		return nil, fmt.Errorf("compile extension %s: %w", pkg, err)
	}

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
