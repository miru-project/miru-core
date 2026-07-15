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

// withStackTrace annotates a Scriggo VM / extension Go error with a stack trace
// so failures surfaced to callers (and ultimately the gRPC layer) are
// debuggable. Without this, extension errors arrive as bare strings (for
// example "expected a map or struct, got slice") with no context about where
// they originated.
//
// When the error is a *scriggo.PanicError (an unrecovered panic inside the
// extension), the Scriggo interpreter call stack is used instead of the host Go
// stack: it lists the extension's own inner function call chain (for example
// Search -> fetchList -> parseItem), which is what an extension author needs to
// locate the failing call. The host goroutine stack (debug.Stack) is only a
// wrapper around the interpreter and is therefore not helpful here. For any
// other error the host goroutine stack is attached as before.
func withStackTrace(where string, err error) error {
	if err == nil {
		return nil
	}
	var panicErr *scriggo.PanicError
	if errors.As(err, &panicErr) {
		if stack := panicErr.Stack(); stack != "" {
			return fmt.Errorf("%s: %w\n%s", where, err, stack)
		}
	}
	return fmt.Errorf("%s: %w\n%s", where, err, debug.Stack())
}

// VM is the Scriggo/Go-subset extension runtime.
type VM interface {
	// Compile compiles extension source code.
	Compile(name string, source any) (*Program, error)
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

	prog, err := r.vm.Compile(ext.Name, string(source))
	if err != nil {
		return fmt.Errorf("compile extension %s: %w", ext.Name, err)
	}

	apiVersion := ext.ApiVersion
	if apiVersion == "" {
		apiVersion = "1"
	}
	log.Printf("Extension loaded (V%s) [GO]: %s %s", apiVersion, ext.Name, ext.Pkg)

	// Run the Load entry point once. Load takes no arguments, exactly mirroring
	// the JavaScript runtime's load() hook. Any cross-function state an
	// extension wants to seed here is written with sdk.SaveCache, keyed by the
	// extension's own package name (which the author already knows). Extensions
	// that do not declare Load are skipped and loaded lazily on the first
	// request instead.
	if _, err := prog.program.Call("Load"); err != nil {
		return err
	}

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
//
// It builds directly from the source in memory. scriggo.Files is an fs.FS
// backed by a map, and scriggo.Build compiles entirely in memory, so there is
// no need to materialize a temporary directory or file on disk.
func (v *ScriggoVM) Compile(name string, source any) (*Program, error) {
	var fsys scriggo.Files
	switch s := source.(type) {
	case string:
		fsys = scriggo.Files{"main.go": []byte(s)}
	case scriggo.Files:
		fsys = s
	default:
		return nil, fmt.Errorf("unsupported source type %T", source)
	}

	p, err := scriggo.Build(fsys, &scriggo.BuildOptions{Packages: packages})
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
