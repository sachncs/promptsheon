// Command codemod-sdktopkgshe-on is a developer tool that rewrites
// Go source files to migrate from the v0.3.x import path
// github.com/sachncs/promptsheon/sdk to the v1.0.0 path
// github.com/sachncs/promptsheon/pkg/promptsheon.
//
// Usage:
//
//	go run ./tools/codemod-sdktopkgshe-on/ ./...
//
// The tool:
//   - Replaces 'import "github.com/sachncs/promptsheon/sdk"' with
//     'import "github.com/sachncs/promptsheon/pkg/promptsheon"'.
//   - When the import uses a local name (e.g. 'import sdk "..."'),
//     the local references 'sdk.X' are renamed to 'promptsheon.X'.
//   - Leaves non-import 'sdk' references (e.g. 'log/slog', local
//     variables, type names) alone.
//
// The tool is conservative: it only changes files where the
// exact import string is found. Run gofmt and goimports after
// running.
package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

const oldImport = `"github.com/sachncs/promptsheon/sdk"`
const newImport = `"github.com/sachncs/promptsheon/pkg/promptsheon"`
const newDefault = "promptsheon"

func main() {
	flag.Parse()
	for _, arg := range flag.Args() {
		if err := walk(arg); err != nil {
			fmt.Fprintf(os.Stderr, "codemod: %v\n", err)
			os.Exit(1)
		}
	}
}

func walk(root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if strings.HasPrefix(info.Name(), ".") || info.Name() == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		return process(path, info)
	})
}

func process(path string, info os.FileInfo) error {
	fset := token.NewFileSet()
	src, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if !strings.Contains(string(src), oldImport) {
		return nil
	}
	file, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}

	renamed := false
	localName := ""
	for _, imp := range file.Imports {
		if imp.Path.Value == oldImport {
			if imp.Name != nil {
				localName = imp.Name.Name
			} else {
				localName = "sdk"
			}
			imp.Path.Value = newImport
			if imp.Name != nil {
				imp.Name = nil
			}
			renamed = true
		}
	}
	if !renamed {
		return nil
	}
	if localName != "" && localName != newDefault {
		ast.Inspect(file, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			ident, ok := sel.X.(*ast.Ident)
			if !ok || ident.Name != localName {
				return true
			}
			ident.Name = newDefault
			return true
		})
	}
	var buf strings.Builder
	if err := format.Node(&buf, fset, file); err != nil {
		return fmt.Errorf("format %s: %w", path, err)
	}
	return os.WriteFile(path, []byte(buf.String()), info.Mode())
}
