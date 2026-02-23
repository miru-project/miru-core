package jsExtension

import (
	"fmt"
	"path/filepath"

	"github.com/dop251/goja"
)

// RegisterJSModule registers a JavaScript module from a source string.
// This allows the module to be loaded via standard require(name) in JavaScript.
func RegisterJSModule(name string, source string, callback func(*goja.Runtime, *goja.Object)) {
	// Wrap the source in a Node.js-style module wrapper
	wrappedSource := "(function(exports, require, module, __filename, __dirname) {" + source + "\n})"

	program, err := goja.Compile(name, wrappedSource, true)
	if err != nil {
		panic(fmt.Errorf("failed to compile module %s: %w", name, err))
	}

	sharedRegistry.RegisterNativeModule(name, func(vm *goja.Runtime, module *goja.Object) {
		f, err := vm.RunProgram(program)
		if err != nil {
			panic(vm.NewGoError(fmt.Errorf("failed to execute module %s: %w", name, err)))
		}

		if call, ok := goja.AssertFunction(f); ok {
			jsExports := module.Get("exports")
			jsRequire := vm.Get("require")

			// func(exports, require, module, __filename, __dirname)
			_, err = call(jsExports, jsExports, jsRequire, module, vm.ToValue(name), vm.ToValue("."))
			if err != nil {
				panic(vm.NewGoError(fmt.Errorf("failed to run module %s: %w", name, err)))
			}
			callback(vm, module)
		}
	})
}

func LoadModule(vm *goja.Runtime, module *goja.Object, program *goja.Program, name string) {
	f, err := vm.RunProgram(program)
	if err != nil {
		panic(vm.NewGoError(fmt.Errorf("failed to execute module %s: %w", name, err)))
	}

	if call, ok := goja.AssertFunction(f); ok {
		jsExports := module.Get("exports")
		jsRequire := vm.Get("require")

		// func(exports, require, module, __filename, __dirname)
		_, err = call(jsExports, jsExports, jsRequire, module, vm.ToValue(name), vm.ToValue("."))
		if err != nil {
			panic(vm.NewGoError(fmt.Errorf("failed to run module %s: %w", name, err)))
		}
	}
}

// LoadSourceModule loads a JavaScript module from source string
// It follows the Node.js module loading pattern by wrapping the source in a function
func LoadSourceModule(vm *goja.Runtime, path string, source string) (goja.Value, error) {
	// Wrap the source in a Node.js-style module wrapper
	// (function(exports, require, module, __filename, __dirname) { ... })
	wrappedSource := "(function(exports, require, module, __filename, __dirname) {" + source + "\n})"

	program, err := goja.Compile(path, wrappedSource, true)
	if err != nil {
		return nil, fmt.Errorf("failed to compile module %s: %w", path, err)
	}

	// Execute the program to get the wrapper function
	wrapperValue, err := vm.RunProgram(program)
	if err != nil {
		return nil, fmt.Errorf("failed to execute module wrapper %s: %w", path, err)
	}

	wrapper, ok := goja.AssertFunction(wrapperValue)
	if !ok {
		return nil, fmt.Errorf("module %s did not result in a function", path)
	}

	// Create module and context objects
	module := vm.NewObject()
	exports := vm.NewObject()
	module.Set("exports", exports)

	require := vm.Get("require")
	filename := vm.ToValue(path)
	dirname := vm.ToValue(filepath.Dir(path))

	// Call the wrapper function: func(exports, require, module, __filename, __dirname)
	_, err = wrapper(exports, exports, require, module, filename, dirname)
	if err != nil {
		return nil, fmt.Errorf("failed to run module %s: %w", path, err)
	}

	return module.Get("exports"), nil
}
