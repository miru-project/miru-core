package golang

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"

	"github.com/miru-project/miru-core/pkg/extension"
	"github.com/open2b/scriggo"
	"github.com/open2b/scriggo/native"
)

var packages native.Packages

// withStackTrace annotates a Scriggo VM / extension Go error with a goroutine
// stack trace so failures surfaced to callers (and ultimately the gRPC layer)
// are debuggable. Without this, extension errors arrive as bare strings (for
// example "expected a map or struct, got slice") with no context about where
// in the Go runtime they originated.
func withStackTrace(where string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w\n%s", where, err, debug.Stack())
}

// VM is the Scriggo/Go-subset extension runtime.
type VM interface {
	// Compile compiles extension source code.
	Compile(name string, source any) (*Program, error)
	// CompileEntry compiles extension source code with a custom entry point.
	// The entryPoint argument names the function run by the Run method (for
	// example "Load"); the other functions of the program can still be looked
	// up and called later with Program.Call.
	CompileEntry(name, source, entryPoint string) (*Program, error)
	// Run executes a compiled program.
	Run(*Program, *scriggo.RunOptions) (Value, error)
}

// Program represents compiled Scriggo code.
type Program struct {
	program *scriggo.Program
}

// Value is a generic Scriggo value.
type Value any

// toAny converts a slice of Value to a slice of any so it can be passed to the
// underlying Scriggo Program.Call method (which takes []any).
func toAny(values []Value) []any {
	args := make([]any, len(values))
	for i, v := range values {
		args[i] = v
	}
	return args
}

// Runtime is the Scriggo runtime environment.
type Runtime struct {
	vm VM

	// program is the compiled program of the extension loaded with
	// LoadExtension. It is kept so that specific functions exported by the
	// extension can be invoked later with Call.
	program *Program
}

// NewRuntime creates a new Scriggo runtime.
func NewRuntime(vm VM) *Runtime {
	return &Runtime{vm: vm}
}

// LoadExtension loads a Scriggo extension into the runtime.
//
// The extension is built with "Load" as its entry point: the Load function
// declared in the extension source is run once (via Run) to set up and extend
// the runtime. The compiled program is kept in the runtime so that the
// specific functions exported by the extension can later be invoked with Call,
// using the local Scriggo build that supports calling a function by name.
func (r *Runtime) LoadExtension(ext *extension.Extension) error {
	if ext == nil {
		return errors.New("extension is nil")
	}

	extPath := filepath.Join(ExtensionDir, ext.Pkg+".go")
	source, err := os.ReadFile(extPath)
	if err != nil {
		return fmt.Errorf("read extension source %s: %w", extPath, err)
	}

	prog, err := r.vm.CompileEntry(ext.Name, string(source), "Load")
	if err != nil {
		return fmt.Errorf("compile extension %s: %w", ext.Name, err)
	}

	// Run the Load entry point once to initialize/extend the runtime.
	if _, err := r.vm.Run(prog, nil); err != nil {
		return fmt.Errorf("run extension %s: %w", ext.Name, err)
	}

	apiVersion := ext.ApiVersion
	if apiVersion == "" {
		apiVersion = "1"
	}
	log.Printf("Extension loaded (V%s) [GO]: %s %s", apiVersion, ext.Name, ext.Pkg)

	r.program = prog
	return nil
}

// Call invokes a function exported by the loaded extension on the Scriggo
// runtime. It relies on the local Scriggo build's Program.Call method, which
// looks up the function by name and evaluates it with the given arguments.
//
// Call must be preceded by LoadExtension; otherwise it returns an error.
func (r *Runtime) Call(name string, args ...Value) (Value, error) {
	if r.program == nil || r.program.program == nil {
		return nil, errors.New("no extension loaded; call LoadExtension first")
	}
	res, err := r.program.program.Call(name, toAny(args)...)
	if err != nil {
		return nil, withStackTrace("scriggo call "+name, err)
	}
	if len(res) == 0 {
		return nil, nil
	}
	return res[0], nil
}

// ScriggoVM implements the VM interface using Scriggo.
type ScriggoVM struct {
	scriggoFile *Scriggofile
}

// NewScriggoVM creates a new Scriggo VM.
func NewScriggoVM(scriggoFile *Scriggofile) *ScriggoVM {
	return &ScriggoVM{
		scriggoFile: scriggoFile,
	}
}

// Compile compiles extension source code using Scriggo.
func (v *ScriggoVM) Compile(name string, source any) (*Program, error) {
	var src string
	switch s := source.(type) {
	case string:
		src = s
	case scriggo.Files:
		dir, err := os.MkdirTemp("", "scriggo-"+name)
		if err != nil {
			return nil, fmt.Errorf("create temp dir: %w", err)
		}
		for fname, content := range s {
			fpath := filepath.Join(dir, fname)
			if err := os.WriteFile(fpath, content, 0644); err != nil {
				os.RemoveAll(dir)
				return nil, fmt.Errorf("write file %s: %w", fname, err)
			}
		}
		fsys := os.DirFS(dir)
		p, err := scriggo.Build(fsys, &scriggo.BuildOptions{Packages: packages})
		os.RemoveAll(dir)
		if err != nil {
			return nil, withStackTrace("scriggo build", err)
		}
		return &Program{program: p}, nil
	default:
		return nil, fmt.Errorf("unsupported source type %T", source)
	}

	dir, err := os.MkdirTemp("", "scriggo-"+name)
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(dir)

	fpath := filepath.Join(dir, "main.go")
	if err := os.WriteFile(fpath, []byte(src), 0644); err != nil {
		return nil, fmt.Errorf("write main.go: %w", err)
	}

	fsys := os.DirFS(dir)
	p, err := scriggo.Build(fsys, &scriggo.BuildOptions{Packages: packages})
	if err != nil {
		return nil, withStackTrace("scriggo build", err)
	}
	return &Program{program: p}, nil
}

// CompileEntry compiles extension source code with a custom entry point using
// Scriggo. The entryPoint argument names the function run by the Run method
// (for example "Load"); the other functions of the program can still be looked
// up and called later with Program.Call.
func (v *ScriggoVM) CompileEntry(name string, source string, entryPoint string) (*Program, error) {
	dir, err := os.MkdirTemp("", "scriggo-"+name)
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(dir)

	fpath := filepath.Join(dir, "main.go")
	if err := os.WriteFile(fpath, []byte(source), 0644); err != nil {
		return nil, fmt.Errorf("write main.go: %w", err)
	}

	fsys := os.DirFS(dir)
	p, err := scriggo.Build(fsys, &scriggo.BuildOptions{Packages: packages, EntryPoint: entryPoint})
	if err != nil {
		return nil, withStackTrace("scriggo build", err)
	}
	return &Program{program: p}, nil
}

// Run executes a compiled Scriggo program.
func (v *ScriggoVM) Run(p *Program, opts *scriggo.RunOptions) (Value, error) {
	if p == nil || p.program == nil {
		return nil, errors.New("program is nil")
	}

	options := &scriggo.RunOptions{}
	if opts != nil {
		options = opts
	}

	if err := p.program.Run(options); err != nil {
		return nil, withStackTrace("scriggo run", err)
	}

	return nil, nil
}

// Scriggofile represents the Scriggo configuration file.
type Scriggofile struct {
	// Packages lists allowed native packages.
	Packages []string `json:"packages"`
}
