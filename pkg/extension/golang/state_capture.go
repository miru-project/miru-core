package golang

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"log"
	"strings"
)

// Automatic state capture for Go (Scriggo) extensions.
//
// Every endpoint call compiles and runs a FRESH VM (and Program.Call even
// re-initializes the package variables of a kept program), so package-level
// variables would reset between calls. The JavaScript runtime solves this with
// a host-side structural snapshot; Scriggo exposes no such hook (the compiled
// globals are internal), so instead the host INSTRUMENTS the extension source
// before compiling:
//
//   - every entry point (Load / Latest / Search / Detail / Watch / Mirror /
//     CreateFilter) gets two injected statements at the top of its body:
//
//	__miru_state_load_all()
//	defer __miru_state_save_all()
//
//   - a generated state bridge (appended to the same file; Scriggo programs
//     are single-file) declares, for every package-level variable, a typed
//     load/save pair that round-trips the variable as JSON through a freshly
//     declared LOCAL variable:
//
//	func __miru_state_save_item() {
//	    var __miru_tmp Item = item
//	    __miru_rt.MiruStatePut("pkg", "__miru_var.item", &__miru_tmp)
//	}
//
// The local-variable bridge is what makes the capture work for EVERY value
// kind: a pointer to an addressable local always reaches the host unwrapped,
// while values (and package cells) of script-declared types are wrapped in a
// Scriggo-internal proxy that can neither be marshaled nor re-asserted in the
// next compile.
//
// The instrumentation is best-effort by construction: if the source cannot be
// parsed, declares __miru_-prefixed identifiers, or a variable type cannot be
// determined, the source is compiled unmodified; and if the instrumented build
// fails, the VM falls back to the original source. Nothing an extension does
// today can break because of it.

// miruRuntimePkgPath is the host package the generated bridge calls into.
const miruRuntimePkgPath = "github.com/miru-project/miru-core/pkg/extension/golang/runtime"

// miruGenPrefix marks every generated identifier; a source that already
// declares such identifiers (or imports them) is left uninstrumented.
const miruGenPrefix = "__miru_"

// miruStateKeyPrefix namespaces the automatic per-variable entries inside the
// package cache. Must stay in sync with runtime.miruVarKeyPrefix.
const miruStateKeyPrefix = "__miru_var."

// miruEntryPoints are the functions the host invokes on an extension. Each one
// gets the state load/save bracket injected.
var miruEntryPoints = map[string]bool{
	"Load":         true,
	"Latest":       true,
	"Search":       true,
	"Detail":       true,
	"Watch":        true,
	"Mirror":       true,
	"CreateFilter": true,
}

// stateVar is one package-level variable eligible for automatic capture.
type stateVar struct {
	name     string
	typeExpr ast.Expr
}

// instrumentStateCapture rewrites source so entry points restore the persisted
// package variables on entry and capture them again on return. When the source
// is not eligible (parse error, identifier collision, no capturable variables)
// the ORIGINAL source is returned unchanged.
func instrumentStateCapture(pkgName, source string) string {
	if pkgName == "" {
		return source
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, pkgName+".go", source, parser.ParseComments)
	if err != nil {
		// Let the real build report the syntax error.
		return source
	}
	if declaresMiruIdentifiers(file) {
		return source
	}
	vars := collectPackageVars(file)
	if len(vars) == 0 {
		return source
	}

	// Inject the load/save bracket into every declared entry point: the load
	// at the top, the save before every return. The save cannot be deferred:
	// the Scriggo VM clobbers the entry's result slots when a deferred callee
	// performs a native call with a pointer argument (observed as
	// "reflect.Set: value of type *int is not assignable to ..."), so the
	// save runs inline on every exit path instead. A call that PANICS (rather
	// than returning) therefore does not capture -- the one divergence from
	// the JavaScript runtime's capture-on-failure guarantee.
	injected := 0
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv != nil || fn.Body == nil || !miruEntryPoints[fn.Name.Name] {
			continue
		}
		loadStmt := &ast.ExprStmt{
			X: &ast.CallExpr{Fun: &ast.Ident{Name: miruGenPrefix + "state_load_all"}},
		}
		fn.Body.List = append([]ast.Stmt{loadStmt}, fn.Body.List...)
		injectSaveCalls(fn)
		injected++
	}
	if injected == 0 {
		return source
	}

	if err := appendStateBridge(fset, file, pkgName, vars); err != nil {
		log.Println("state capture: generating bridge for", pkgName, "failed:", err)
		return source
	}

	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, file); err != nil {
		return source
	}
	return buf.String()
}

// saveCallStmt builds the state save call statement.
func saveCallStmt() ast.Stmt {
	return &ast.ExprStmt{X: &ast.CallExpr{Fun: &ast.Ident{Name: miruGenPrefix + "state_save_all"}}}
}

// injectSaveCalls inserts the state save call before every return statement of
// the entry point (at any nesting depth), and at the end of the body when the
// function can complete without a return statement (no results).
func injectSaveCalls(fn *ast.FuncDecl) {
	var walkStmts func(list []ast.Stmt) []ast.Stmt
	walkStmts = func(list []ast.Stmt) []ast.Stmt {
		out := make([]ast.Stmt, 0, len(list)+1)
		for _, st := range list {
			switch s := st.(type) {
			case *ast.ReturnStmt:
				out = append(out, saveCallStmt(), s)
				continue
			case *ast.IfStmt:
				s.Body = &ast.BlockStmt{List: walkStmts(s.Body.List)}
				if s.Else != nil {
					switch e := s.Else.(type) {
					case *ast.BlockStmt:
						e.List = walkStmts(e.List)
					case *ast.IfStmt:
						s.Else = walkStmts([]ast.Stmt{e})[0]
					}
				}
			case *ast.ForStmt:
				s.Body = &ast.BlockStmt{List: walkStmts(s.Body.List)}
			case *ast.RangeStmt:
				s.Body = &ast.BlockStmt{List: walkStmts(s.Body.List)}
			case *ast.SwitchStmt:
				for _, c := range s.Body.List {
					if cc, ok := c.(*ast.CaseClause); ok {
						cc.Body = walkStmts(cc.Body)
					}
				}
			case *ast.TypeSwitchStmt:
				for _, c := range s.Body.List {
					if cc, ok := c.(*ast.CaseClause); ok {
						cc.Body = walkStmts(cc.Body)
					}
				}
			case *ast.SelectStmt:
				for _, c := range s.Body.List {
					if cc, ok := c.(*ast.CommClause); ok {
						cc.Body = walkStmts(cc.Body)
					}
				}
			case *ast.LabeledStmt:
				if _, isRet := s.Stmt.(*ast.ReturnStmt); isRet {
					out = append(out, saveCallStmt(), s)
					continue
				}
			case *ast.BlockStmt:
				s.List = walkStmts(s.List)
			}
			out = append(out, st)
		}
		return out
	}
	fn.Body.List = walkStmts(fn.Body.List)

	// A function without results can fall off the end of its body.
	if fn.Type.Results == nil {
		last := len(fn.Body.List) - 1
		if last < 0 {
			fn.Body.List = append(fn.Body.List, saveCallStmt())
		} else if _, isRet := fn.Body.List[last].(*ast.ReturnStmt); !isRet {
			fn.Body.List = append(fn.Body.List, saveCallStmt())
		}
	}
}

// declaresMiruIdentifiers reports whether the file already declares (or
// imports) any identifier in the generated namespace, which would collide with
// the instrumentation.
func declaresMiruIdentifiers(file *ast.File) bool {
	for _, imp := range file.Imports {
		if imp.Name != nil && strings.HasPrefix(imp.Name.Name, miruGenPrefix) {
			return true
		}
	}
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Name != nil && strings.HasPrefix(d.Name.Name, miruGenPrefix) {
				return true
			}
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.ValueSpec:
					for _, n := range s.Names {
						if strings.HasPrefix(n.Name, miruGenPrefix) {
							return true
						}
					}
				case *ast.TypeSpec:
					if s.Name != nil && strings.HasPrefix(s.Name.Name, miruGenPrefix) {
						return true
					}
				}
			}
		}
	}
	return false
}

// collectPackageVars lists the package-level variables eligible for capture,
// with a type expression resolved either from the declaration or from a
// literal initializer. Variables whose type cannot be determined statically
// (e.g. `var x = someCall()`) and types JSON can never round-trip (function
// values, channels) are skipped: they keep per-call semantics.
func collectPackageVars(file *ast.File) []stateVar {
	var out []stateVar
	skip := map[string]bool{}
	add := func(name string, te ast.Expr) {
		if name == "_" || skip[name] {
			return
		}
		if te == nil {
			log.Println("state capture: variable", name, "has no statically known type; it will not persist across calls")
			skip[name] = true
			return
		}
		if hasUnpersistableType(te) {
			log.Println("state capture: variable", name, "has a function or channel type; it will not persist across calls")
			skip[name] = true
			return
		}
		out = append(out, stateVar{name: name, typeExpr: te})
	}
	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.VAR {
			continue
		}
		for _, spec := range gd.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			if len(vs.Names) == 1 && len(vs.Values) <= 1 {
				add(vs.Names[0].Name, varTypeExpr(vs))
				continue
			}
			if vs.Type != nil {
				for _, n := range vs.Names {
					add(n.Name, vs.Type)
				}
				continue
			}
			// `var a, b = f()` -- types unknowable.
			for _, n := range vs.Names {
				add(n.Name, nil)
			}
		}
	}
	return out
}

// hasUnpersistableType reports whether the type expression contains a function
// or channel type: JSON cannot represent those, so a captured value could
// never be restored.
func hasUnpersistableType(expr ast.Expr) bool {
	found := false
	ast.Inspect(expr, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.FuncType, *ast.ChanType:
			found = true
			return false
		}
		return true
	})
	return found
}

// varTypeExpr resolves the declared type of a single-name ValueSpec, falling
// back to the type of a literal initializer.
func varTypeExpr(vs *ast.ValueSpec) ast.Expr {
	if vs.Type != nil {
		return vs.Type
	}
	if len(vs.Values) != 1 {
		return nil
	}
	switch v := vs.Values[0].(type) {
	case *ast.BasicLit:
		switch v.Kind {
		case token.INT:
			return &ast.Ident{Name: "int"}
		case token.FLOAT:
			return &ast.Ident{Name: "float64"}
		case token.STRING:
			return &ast.Ident{Name: "string"}
		case token.CHAR:
			return &ast.Ident{Name: "rune"}
		case token.IMAG:
			return &ast.Ident{Name: "complex128"}
		}
	case *ast.CompositeLit:
		return v.Type
	case *ast.UnaryExpr:
		if cl, ok := v.X.(*ast.CompositeLit); ok {
			return cl.Type // &T{...}
		}
	}
	return nil
}

// appendStateBridge parses the generated bridge functions and appends them to
// the file, adding the synthetic import they call into. Type references in the
// bridge are copied verbatim from the original file's type expressions, so
// they resolve against the file's OWN imports and none of them are touched.
func appendStateBridge(fset *token.FileSet, file *ast.File, pkgName string, vars []stateVar) error {
	snippet := renderStateBridge(pkgName, fset, vars)
	bridgeFile, err := parser.ParseFile(fset, "__miru_state.go", snippet, parser.SkipObjectResolution)
	if err != nil {
		return err
	}
	ensureMiruImport(file)
	file.Decls = append(file.Decls, bridgeFile.Decls...)
	return nil
}

// ensureMiruImport adds the synthetic runtime import the generated bridge
// calls into, creating the import declaration when the file has none.
func ensureMiruImport(file *ast.File) {
	spec := &ast.ImportSpec{
		Name: &ast.Ident{Name: miruGenPrefix + "rt"},
		Path: &ast.BasicLit{Kind: token.STRING, Value: fmt.Sprintf("%q", miruRuntimePkgPath)},
	}
	var importDecl *ast.GenDecl
	for _, d := range file.Decls {
		if gd, ok := d.(*ast.GenDecl); ok && gd.Tok == token.IMPORT {
			importDecl = gd
			break
		}
	}
	if importDecl == nil {
		pos := file.Package + 1
		importDecl = &ast.GenDecl{
			Tok:    token.IMPORT,
			TokPos: pos,
			Lparen: pos,
			Specs:  []ast.Spec{spec},
		}
		file.Decls = append([]ast.Decl{importDecl}, file.Decls...)
		return
	}
	importDecl.Specs = append(importDecl.Specs, spec)
	if len(importDecl.Specs) > 1 && importDecl.Lparen == token.NoPos {
		// A paren-less single import ("import \"x\"") cannot hold several
		// specs; switch the declaration to its parenthesized form.
		importDecl.Lparen = importDecl.Specs[0].Pos()
	}
}

// renderStateBridge generates the per-variable typed load/save pairs and the
// load_all / save_all helpers. Variables whose type cannot be rendered are
// left out of BOTH the pairs and the helpers, so the generated code always
// compiles. The returned snippet has no imports: they are added to the host
// file by appendStateBridge.
func renderStateBridge(pkgName string, fset *token.FileSet, vars []stateVar) string {
	var b strings.Builder
	b.WriteString("// Code generated by miru-core automatic state capture. DO NOT EDIT.\n")
	fmt.Fprintf(&b, "package %s\n\n", pkgName)

	var rendered []string
	for _, v := range vars {
		typeText, err := typeString(fset, v.typeExpr)
		if err != nil {
			log.Println("state capture: type of variable", v.name, "could not be rendered; it will not persist:", err)
			continue
		}
		rendered = append(rendered, v.name)
		fmt.Fprintf(&b, "func %sstate_load_%s() {\n", miruGenPrefix, v.name)
		fmt.Fprintf(&b, "\tvar __miru_tmp %s\n", typeText)
		fmt.Fprintf(&b, "\tif !__miru_rt.MiruStateGet(%q, %q, &__miru_tmp) {\n", pkgName, miruStateKeyPrefix+v.name)
		b.WriteString("\t\treturn\n\t}\n")
		fmt.Fprintf(&b, "\t%s = __miru_tmp\n}\n\n", v.name)
		fmt.Fprintf(&b, "func %sstate_save_%s() {\n", miruGenPrefix, v.name)
		fmt.Fprintf(&b, "\tvar __miru_tmp %s = %s\n", typeText, v.name)
		fmt.Fprintf(&b, "\t__miru_rt.MiruStatePut(%q, %q, &__miru_tmp)\n}\n\n", pkgName, miruStateKeyPrefix+v.name)
	}

	fmt.Fprintf(&b, "func %sstate_load_all() {\n", miruGenPrefix)
	for _, name := range rendered {
		fmt.Fprintf(&b, "\t%sstate_load_%s()\n", miruGenPrefix, name)
	}
	b.WriteString("}\n\n")
	fmt.Fprintf(&b, "func %sstate_save_all() {\n", miruGenPrefix)
	for _, name := range rendered {
		fmt.Fprintf(&b, "\t%sstate_save_%s()\n", miruGenPrefix, name)
	}
	b.WriteString("}\n")
	return b.String()
}

// typeString renders a type expression as single-line source text.
func typeString(fset *token.FileSet, expr ast.Expr) (string, error) {
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, expr); err != nil {
		return "", err
	}
	return strings.ReplaceAll(buf.String(), "\n", " "), nil
}
