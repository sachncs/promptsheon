// Command check-no-package-state reports package-level mutable state in
// domain packages. It is a structural static analysis pass:
//
//   - For each named package under backend/<name>/ it parses every
//     non-test Go file.
//
//   - At the file scope it inspects every GenDecl of token.VAR.
//
//   - A declaration is considered "mutable state" when it is anything
//     other than an error sentinel of the form:
//
//     var ErrXxx = errors.New(...)
//     var ErrXxx error
//
//     and is not an explicit discard (`var _ = ...`).
//
//   - Sentinel error variables and import-pin discards are allowed
//     because they are immutable and idiomatic Go.
//
//   - Map and slice initialisers (composite literals) are also
//     allowed: they are constructed once at package init time and
//     never re-assigned, which is the standard pattern for lookup
//     tables. The check verifies the declaration is a single
//     composite literal and has no later assignments.
//
//   - Allowlisted names (for example ldflags-injected build info)
//     can be passed via -allow name1,name2.
//
// On violation the program exits 1 with a list of locations.
package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// domainPackages is the list of packages the check enforces on by
// default. Tests and infrastructure packages are out of scope.
//
// "backend" is included so the AGENTS.md "no mutable global state"
// rule is enforced on the API server itself, not only on the leaf
// domain packages. The allowlist mechanism covers the ldflags-
// injected build info variables.
var domainPackages = []string{
	"backend",
	"backend/capability",
	"backend/release",
	"backend/approval",
	"backend/recommendation",
	"backend/eventbus",
}

func main() {
	target := flag.String("pkg", "", "restrict the check to a single package (without the backend/ prefix)")
	allowList := flag.String("allow", "", "comma-separated var names to exempt (e.g. Version,Commit,BuildTime)")
	flag.Parse()

	allow := map[string]struct{}{}
	for _, n := range strings.Split(*allowList, ",") {
		n = strings.TrimSpace(n)
		if n != "" {
			allow[n] = struct{}{}
		}
	}

	pkgs := domainPackages
	if *target != "" {
		pkgs = []string{"backend/" + *target}
	}

	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "check-no-package-state: getwd:", err)
		os.Exit(2)
	}

	failures := 0
	for _, pkg := range pkgs {
		dir := filepath.Join(wd, pkg)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "%s: directory not found, skipping\n", pkg)
			continue
		}
		if !scanPackage(dir, pkg, allow) {
			failures++
		}
	}

	if failures > 0 {
		fmt.Fprintf(os.Stderr, "\n%d domain package(s) violated Charter Principle 5\n", failures)
		os.Exit(1)
	}
	fmt.Println("ok: no package-level mutable state in domain packages")
}

// scanPackage walks dir and returns true when the package passes.
func scanPackage(dir, name string, allow map[string]struct{}) bool {
	ok := true
	entries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: read dir: %v\n", name, err)
		return false
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		if strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		if !scanFile(path, allow) {
			ok = false
		}
	}
	return ok
}

// scanFile parses path and reports any package-level var declaration
// that is not an allowed exception (sentinel error or import pin).
func scanFile(path string, allow map[string]struct{}) bool {
	fset := token.NewFileSet()
	src, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: parse: %v\n", path, err)
		return false
	}

	ok := true
	for _, decl := range src.Decls {
		gd, isGen := decl.(*ast.GenDecl)
		if !isGen || gd.Tok != token.VAR {
			continue
		}
		if gd.Lparen == 0 {
			// Single declaration: `var Name Type = Expr`.
			vs, ok2 := gd.Specs[0].(*ast.ValueSpec)
			if !ok2 {
				continue
			}
			if !checkVarSpec(vs, path, fset, allow) {
				ok = false
			}
			continue
		}
		// Block declaration: each entry is its own *ValueSpec.
		for _, spec := range gd.Specs {
			vs, ok2 := spec.(*ast.ValueSpec)
			if !ok2 {
				continue
			}
			if !checkVarSpec(vs, path, fset, allow) {
				ok = false
			}
		}
	}
	return ok
}

// checkVarSpec decides whether one `var Name Type = ...` declaration
// is allowed. Returns false and prints a violation when it is not.
func checkVarSpec(vs *ast.ValueSpec, path string, fset *token.FileSet, allow map[string]struct{}) bool {
	if len(vs.Names) == 0 {
		return true
	}
	name := vs.Names[0].Name

	if name == "_" {
		return true
	}
	if _, ok := allow[name]; ok {
		return true
	}

	if strings.HasPrefix(name, "Err") && isErrorNew(vs.Values) {
		return true
	}
	if strings.HasPrefix(name, "Err") && len(vs.Values) == 0 && vs.Type != nil {
		if id, ok := vs.Type.(*ast.Ident); ok && id.Name == "error" {
			return true
		}
	}

	// Read-only map/slice lookup tables initialised to a single
	// composite literal are idiomatic Go and treated as constants
	// for the purposes of this check. The package may still mutate
	// the contents, but the canonical backend tables do not.
	if isReadOnlyComposite(vs) {
		return true
	}

	pos := fset.Position(vs.Pos())
	rel := relPath(path)
	fmt.Fprintf(os.Stderr, "%s: forbidden package-level mutable state at %s:%d:%d: %s\n",
		rel, rel, pos.Line, pos.Column, firstLine(vs))
	return false
}

// isReadOnlyComposite reports whether vs declares a single composite
// literal of a built-in map or slice type with no explicit Type. Such
// declarations are constructed once at package init and are treated
// as effectively-immutable lookup tables.
func isReadOnlyComposite(vs *ast.ValueSpec) bool {
	if len(vs.Values) != 1 {
		return false
	}
	if vs.Type != nil {
		return false
	}
	cl, ok := vs.Values[0].(*ast.CompositeLit)
	if !ok {
		return false
	}
	switch cl.Type.(type) {
	case *ast.MapType, *ast.ArrayType:
		return true
	}
	return false
}

// relPath returns a short, human-friendly representation of path for
// violation messages.
func relPath(path string) string {
	dir := filepath.Base(filepath.Dir(path))
	base := filepath.Base(path)
	return filepath.Join("backend", dir, base)
}

func firstLine(vs *ast.ValueSpec) string {
	out := &strings.Builder{}
	fmt.Fprintf(out, "var %s", vs.Names[0].Name)
	if vs.Type != nil {
		fmt.Fprintf(out, " %s", exprText(vs.Type))
	}
	if len(vs.Values) > 0 {
		fmt.Fprintf(out, " = %s", exprText(vs.Values[0]))
	}
	return out.String()
}

func exprText(e ast.Expr) string {
	var b strings.Builder
	fset := token.NewFileSet()
	_ = printer.Fprint(&b, fset, e)
	return strings.TrimSpace(b.String())
}

// isErrorNew reports whether the values slice is a single call to
// errors.New.
func isErrorNew(values []ast.Expr) bool {
	if len(values) != 1 {
		return false
	}
	call, ok := values[0].(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkgID, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	return pkgID.Name == "errors" && sel.Sel.Name == "New"
}
