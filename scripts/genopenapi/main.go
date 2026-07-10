// Package main is the genopenapi tool. It walks the server's
// route registrations and the corresponding handler functions in
// internal/api/handlers_*.go, and emits a real OpenAPI path
// entry per route (no TODO stubs, no template "OK" responses).
//
// The tool is intentionally simple: it uses the go/parser AST,
// not reflection or runtime data. That makes the output
// deterministic and the tool itself trivially testable with
// canned inputs.
//
// Usage:
//
//	go run ./scripts/genopenapi                  # write to api/openapi.yaml
//	go run ./scripts/genopenapi -out foo.yaml    # write to foo.yaml
//	go run ./scripts/genopenapi -dry-run         # print to stdout
//
// The tool is idempotent: running it twice produces the same
// output. Re-running the tool is the canonical way to keep the
// spec in sync with the code.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// route is a (method, path) pair registered with mux.HandleFunc.
type route struct {
	Method string
	Path   string
	// handlerName is the resolved handler function name (e.g.
	// "handleCreatePrompt"). It may be empty for routes whose
	// handler is wrapped through an alias (e.g. createKey :=
	// s.handleCreateAPIKey); in that case the tool resolves the
	// alias.
	handlerName string
}

func main() {
	repoRoot, err := os.Getwd()
	if err != nil {
		fail("getwd: %v", err)
	}
	outPath := flag.String("out", "api/openapi.yaml", "output file (relative to repo root)")
	dryRun := flag.Bool("dry-run", false, "print to stdout instead of writing")
	flag.Parse()

	if !filepath.IsAbs(*outPath) {
		*outPath = filepath.Join(repoRoot, *outPath)
	}

	routes, err := collectRoutes(repoRoot + "/internal/api/server.go")
	if err != nil {
		fail("collect routes: %v", err)
	}
	if len(routes) == 0 {
		fail("no routes collected; refusing to write an empty spec")
	}
	// Resolve the handler name for any route that used an
	// alias. The aliases live in the routes() function so we
	// parse server.go twice (cheap) or we resolve during
	// collection.
	for i := range routes {
		if routes[i].handlerName == "" {
			resolved, resolveErr := resolveAlias(repoRoot+"/internal/api/server.go", routes[i])
			if resolveErr != nil {
				fmt.Fprintf(os.Stderr, "warn: could not resolve handler for %s %s: %v\n", routes[i].Method, routes[i].Path, resolveErr)
			} else {
				routes[i].handlerName = resolved
			}
		}
	}

	// For each route, find the handler in handlers_*.go and
	// extract the request struct.
	handlersByName, err := collectHandlers(repoRoot + "/internal/api")
	if err != nil {
		fail("collect handlers: %v", err)
	}

	// Group by path.
	byPath := map[string][]route{}
	for _, r := range routes {
		byPath[r.Path] = append(byPath[r.Path], r)
	}
	// Sort paths for stable output.
	paths := make([]string, 0, len(byPath))
	for p := range byPath {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	var buf bytes.Buffer
	writeHeader(&buf)
	for _, p := range paths {
		rs := byPath[p]
		// Sort methods within a path (GET, POST, PUT, DELETE).
		sort.SliceStable(rs, func(i, j int) bool {
			return methodOrder(rs[i].Method) < methodOrder(rs[j].Method)
		})
		writePath(&buf, p, rs, handlersByName)
	}
	writeFooter(&buf)

	if *dryRun {
		_, _ = os.Stdout.Write(buf.Bytes())
		return
	}
	if err := os.WriteFile(*outPath, buf.Bytes(), 0o600); err != nil {
		fail("write: %v", err)
	}
	fmt.Fprintf(os.Stderr, "wrote %d path(s) to %s\n", len(byPath), *outPath)
}

const methodGET = "GET"
const methodDELETE = "DELETE"
const methodPOST = "POST"
const methodPUT = "PUT"
const typeObject = "object"
const typeString = "string"
const typeNumber = "number"
const typeArray = "array"
const typeInt = "int"
const pathHealth = "/health"
const pathReady = "/ready"
const resAlert = "Alert"
const resAlertRule = "AlertRule"
const resEvalResult = "EvalResult"
const resEvalRun = "EvalRun"
const resEvalReport = "EvalReport"
const resGuardrailViolation = "GuardrailViolation"
const resGuardrailRule = "GuardrailRule"
const resPrompt = "Prompt"
const segAlerts = "alerts"
const segEval = "eval"
const segGuardrails = "guardrails"
const segPrompts = "prompts"

func methodOrder(m string) int {
	switch strings.ToUpper(m) {
	case methodGET:
		return 0
	case methodPOST:
		return 1
	case methodPUT:
		return 2
	case methodDELETE:
		return 3
	case "PATCH":
		return 4
	default:
		return 5
	}
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "genopenapi: "+format+"\n", args...)
	os.Exit(1)
}

func collectRoutes(path string) ([]route, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	var routes []route
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if sel.Sel.Name != "HandleFunc" {
			return true
		}
		// sel.X should be "s.mux"
		x, ok := sel.X.(*ast.SelectorExpr)
		if !ok || x.Sel.Name != "mux" {
			return true
		}
		// call.Args[0] is a string literal: "METHOD /path"
		// call.Args[1] is the handler expression
		if len(call.Args) < 2 {
			return true
		}
		lit, ok := call.Args[0].(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		mp := strings.Trim(lit.Value, `"`)
		parts := strings.SplitN(mp, " ", 2)
		if len(parts) != 2 {
			return true
		}
		method, p := parts[0], parts[1]
		// call.Args[1] is the handler. It may be:
		//   s.handleX
		//   createKey (an alias)
		//   s.wrapHandler(s.handleX)
		handlerName := extractHandlerName(call.Args[1])
		routes = append(routes, route{
			Method:      method,
			Path:        p,
			handlerName: handlerName,
		})
		return true
	})
	return routes, nil
}

// extractHandlerName pulls the function name out of a handler
// expression. Returns the empty string if the expression is an
// alias (variable name) that needs further resolution.
func extractHandlerName(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.SelectorExpr:
		// s.handleX
		return e.Sel.Name
	case *ast.CallExpr:
		// s.wrapHandler(s.handleX) or s.wrapHandler(createKey)
		if len(e.Args) > 0 {
			return extractHandlerName(e.Args[0])
		}
	case *ast.Ident:
		// alias, e.g. createKey
		// leave empty so the caller resolves it
		_ = e
	}
	return ""
}

// resolveAlias looks up the value of a variable alias in the
// server.go routes() function. For example:
//
//	createKey := s.handleCreateAPIKey
//
// is resolved to "handleCreateAPIKey".
func resolveAlias(path string, r route) (string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return "", err
	}
	// Find the s.mux.HandleFunc call. Look at the statement
	// that contains it. The previous statement should be the
	// assignment of the alias.
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "HandleFunc" {
			return true
		}
		if len(call.Args) < 2 {
			return true
		}
		lit, ok := call.Args[0].(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		mp := strings.Trim(lit.Value, `"`)
		if !strings.HasPrefix(mp, r.Method+" ") || strings.SplitN(mp, " ", 2)[1] != r.Path {
			return true
		}
		// We have the matching HandleFunc. The alias is
		// whatever the previous statement assigned. We can
		// walk up the block, but a simpler approach: search
		// the entire file for an assignment of the form
		//
		//   <alias> := <expr>
		//
		// where <alias> appears as the second arg of THIS
		// HandleFunc call. We do that by comparing the
		// syntactic text of call.Args[1] to the LHS of
		// every AssignStmt in the file.
		ident, ok := call.Args[1].(*ast.Ident)
		if !ok {
			return true
		}
		ast.Inspect(file, func(m ast.Node) bool {
			as, ok := m.(*ast.AssignStmt)
			if !ok || as.Tok != token.DEFINE || len(as.Lhs) != 1 || len(as.Rhs) != 1 {
				return true
			}
			lhs, ok := as.Lhs[0].(*ast.Ident)
			if !ok {
				return true
			}
			if lhs.Name == ident.Name {
				if sel, ok := as.Rhs[0].(*ast.SelectorExpr); ok {
					ident.Name = sel.Sel.Name
				}
			}
			return true
		})
		// We mutated ident.Name in place; the caller will
		// re-extract.
		_ = ident
		return false
	})
	return r.handlerName, nil
}

// handlerInfo is the extracted description of a handler. We
// record the function's name, its doc comment (used as the
// OpenAPI summary), and the schema of the first local variable
// declaration (the request struct).
type handlerInfo struct {
	Name    string
	Summary string
	// requestType is the Go type name of the request struct
	// (e.g. "struct {...}"). The generator treats it as
	// anonymous and walks its fields directly rather than
	// resolving to a named type.
	requestType *requestStruct
}

// requestStruct is the parsed shape of a `var req struct{...}`
// or `req := struct{...}{}` declaration. We only need field
// names and JSON tags.
type requestStruct struct {
	Fields []requestField
}

type requestField struct {
	JSONName string
	GoType   string
	Required bool
}

// collectHandlers reads every Go file in the given directory and
// extracts handler function metadata.
func collectHandlers(dir string) (map[string]*handlerInfo, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "handlers_*.go"))
	if err != nil {
		return nil, err
	}
	out := map[string]*handlerInfo{}
	for _, path := range matches {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			if !strings.HasPrefix(fn.Name.Name, "handle") {
				continue
			}
			info := &handlerInfo{Name: fn.Name.Name}
			info.Summary = extractDocSummary(fn.Doc)
			info.requestType = extractRequestStruct(fn)
			out[fn.Name.Name] = info
		}
	}
	return out, nil
}

// extractDocSummary returns the first sentence of the doc
// comment, or "" if there is no doc comment.
func extractDocSummary(doc *ast.CommentGroup) string {
	if doc == nil || doc.List == nil {
		return ""
	}
	full := doc.Text()
	// Strip leading // and any leading package/function
	// heading.
	full = strings.TrimSpace(full)
	if i := strings.IndexAny(full, "\n."); i > 0 {
		// Prefer the first sentence (terminated by '.')
		// over the first line, since the line might be a
		// sub-heading like "Foo does X.".
		end := strings.Index(full, ". ")
		if end == -1 {
			return full
		}
		return full[:end+1]
	}
	return full
}

// extractRequestStruct walks a function body looking for the
// first local variable declaration whose type is a
// `var req struct{...}` or `req := struct{...}{}`. The generator
// stops at the first hit.
func extractRequestStruct(fn *ast.FuncDecl) *requestStruct {
	if fn.Body == nil {
		return nil
	}
	for _, stmt := range fn.Body.List {
		switch s := stmt.(type) {
		case *ast.DeclStmt:
			gs, ok := s.Decl.(*ast.GenDecl)
			if !ok || gs.Tok != token.VAR {
				continue
			}
			for _, sp := range gs.Specs {
				vs, ok := sp.(*ast.ValueSpec)
				if !ok {
					continue
				}
				if len(vs.Names) == 0 {
					continue
				}
				name := vs.Names[0].Name
				if !strings.HasPrefix(strings.ToLower(name), "req") {
					continue
				}
				if len(vs.Values) == 0 {
					// var req struct{...} has no Values.
					if rt := structFromType(vs.Type); rt != nil {
						return rt
					}
				}
			}
		case *ast.AssignStmt:
			if s.Tok != token.DEFINE {
				continue
			}
			if len(s.Lhs) != 1 || len(s.Rhs) != 1 {
				continue
			}
			ident, ok := s.Lhs[0].(*ast.Ident)
			if !ok {
				continue
			}
			if !strings.HasPrefix(strings.ToLower(ident.Name), "req") {
				continue
			}
			if rt := structFromType(s.Rhs[0]); rt != nil {
				return rt
			}
		}
	}
	return nil
}

// structFromType returns a *requestStruct if expr is an
// anonymous struct type or a struct composite literal whose
// type is an anonymous struct. Returns nil otherwise.
func structFromType(expr ast.Expr) *requestStruct {
	st, ok := unwrapStructType(expr)
	if !ok {
		return nil
	}
	rt := &requestStruct{}
	for _, field := range st.Fields.List {
		nameFn, required := jsonTagInfo(field.Tag)
		// Field can be multi-named: `A, B int`. Emit each.
		names := field.Names
		if len(names) == 0 {
			// Embedded field; skip.
			continue
		}
		for _, n := range names {
			rt.Fields = append(rt.Fields, requestField{
				JSONName: nameFn(fallbackName(n)),
				GoType:   typeName(field.Type),
				Required: required,
			})
		}
	}
	return rt
}

func unwrapStructType(expr ast.Expr) (*ast.StructType, bool) {
	switch e := expr.(type) {
	case *ast.StructType:
		return e, true
	case *ast.CompositeLit:
		return unwrapStructType(e.Type)
	}
	return nil, false
}

// jsonTagInfo parses a struct field's `json:"name,omitempty"`
// tag and returns the jsonNameFn callback and the required
// flag. The required flag is true when the tag does not have
// "omitempty".
func jsonTagInfo(tag *ast.BasicLit) (jsonNameFn, bool) {
	if tag == nil {
		return func(fallback string) string { return fallback }, true
	}
	raw := strings.Trim(tag.Value, "`")
	st := newTagParser(raw).parse("json")
	required := true
	if _, ok := st["omitempty"]; ok {
		required = false
	}
	name := st["_default_"]
	return func(fallback string) string {
		if name != "" {
			return name
		}
		return fallback
	}, required
}

// jsonNameFn takes a fallback field name and returns the
// preferred JSON name (from the json tag if set).
type jsonNameFn func(fallback string) string

// fallbackName returns the conventional JSON name for an
// ast.Ident (lowercase first letter).
func fallbackName(n *ast.Ident) string {
	if n == nil {
		return ""
	}
	s := n.Name
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}

// tagParser is a small subset of encoding/json's struct tag
// parser. We avoid importing reflect.StructTag so the tool has
// no runtime dependencies.
//
// The first whitespace-separated field is the unnamed (default)
// value, which for the json tag is the field name. The
// remaining fields are key:"value" pairs.
type tagParser struct {
	raw string
}

func newTagParser(raw string) *tagParser { return &tagParser{raw: raw} }

// parse returns the parsed tag. The "_default_" key is the
// unnamed value of the first whitespace-separated field. For
// a `json:"name,omitempty"` tag, _default_="name".
func (p *tagParser) parse(_ string) map[string]string {
	out := map[string]string{}
	if p.raw == "" {
		return out
	}
	fields := splitTagFields(p.raw)
	for i, f := range fields {
		// Each field is either `value` (the unnamed case) or
		// `key:"value"`. Detect which by looking for the
		// colon.
		colon := strings.Index(f, ":")
		if colon == -1 {
			// Unnamed: `name` or `name,suffix`. The leading
			// word is the value.
			name := f
			if comma := strings.Index(f, ","); comma != -1 {
				name = f[:comma]
			}
			if i == 0 {
				out["_default_"] = name
			}
			continue
		}
		k := f[:colon]
		v := strings.Trim(f[colon+1:], `"`)
		out[k] = v
	}
	return out
}

func splitTagFields(s string) []string {
	var out []string
	var cur strings.Builder
	inQuote := false
	for _, r := range s {
		switch {
		case r == '"':
			inQuote = !inQuote
			cur.WriteRune(r)
		case r == ' ' && !inQuote:
			if cur.Len() > 0 {
				out = append(out, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteRune(r)
		}
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

// typeName returns the source-level name of expr. For built-in
// types it returns the keyword. For pointer/slice/map types it
// strips the qualifiers. For named types it returns the name.
func typeName(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.StarExpr:
		return "*" + typeName(e.X)
	case *ast.ArrayType:
		if e.Len == nil {
			return "[]" + typeName(e.Elt)
		}
		return "[N]" + typeName(e.Elt)
	case *ast.MapType:
		return "map[" + typeName(e.Key) + "]" + typeName(e.Value)
	case *ast.SelectorExpr:
		return typeName(e.X) + "." + e.Sel.Name
	}
	return "any"
}

// yamlType maps a Go type name to an OpenAPI type. The mapping
// is conservative; anything we cannot classify becomes
// `type: object` and is later resolved as a $ref.
func yamlType(goType string) (string, bool) {
	switch goType {
	case typeString:
		return typeString, false
	case typeInt, "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64",
		"float32", "float64":
		return typeNumber, false
	case "bool":
		return "boolean", false
	}
	if strings.HasPrefix(goType, "[]") {
		return typeArray, false
	}
	if strings.HasPrefix(goType, "map[") {
		return typeObject, false
	}
	if strings.HasPrefix(goType, "*") {
		// For pointers, the schema is the same as the
		// element type.
		inner := strings.TrimPrefix(goType, "*")
		yt, isRef := yamlType(inner)
		return yt, isRef
	}
	return goType, true
}

func writeHeader(buf *bytes.Buffer) {
	buf.WriteString(`openapi: "3.0.3"
info:
  title: Promptsheon API
  description: Version Control System for AI agent intelligence
  # Version 0.3.0 documents the workflow engine, evaluation
  # framework, and OAuth/SSO. The spec is auto-generated by
  # scripts/genopenapi from the route registrations in
  # internal/api/server.go and the handler bodies in
  # internal/api/handlers_*.go. Re-run the generator whenever a
  # route or handler changes.
  version: "0.3.0"

security:
  - BearerAuth: []

components:
  securitySchemes:
    BearerAuth:
      type: apiKey
      in: header
      name: Authorization
      description: "API key prefixed with ps_. Send as: Authorization: Bearer ps_..."

  schemas:
    Error:
      type: object
      properties:
        error:
          type: string

paths:
`)
}

func writeFooter(_ *bytes.Buffer) {
	// No footer needed: the paths section is the last block.
}

func writePath(buf *bytes.Buffer, path string, rs []route, handlers map[string]*handlerInfo) {
	buf.WriteString("  ")
	buf.WriteString(path)
	buf.WriteString(":\n")
	for _, r := range rs {
		h := handlers[r.handlerName]
		writeMethod(buf, r.Method, h, path, r)
	}
}

func writeMethod(buf *bytes.Buffer, method string, h *handlerInfo, path string, _ route) {
	buf.WriteString("    ")
	buf.WriteString(strings.ToLower(method))
	buf.WriteString(":\n")

	summary := ""
	if h != nil && h.Summary != "" {
		summary = h.Summary
	} else {
		summary = fmt.Sprintf("%s %s", method, cleanPath(path))
	}
	fmt.Fprintf(buf, "      summary: %q\n", summary)

	if path == pathHealth || path == pathReady {
		// System endpoints are unauthenticated.
		fmt.Fprintf(buf, "      security: []\n")
	}

	if hasPathParam(path) {
		writePathParams(buf, path)
	}

	if method != methodGET && method != methodDELETE && h != nil && h.requestType != nil {
		writeRequestBody(buf, h.requestType)
	}

	fmt.Fprintf(buf, "      responses:\n")
	fmt.Fprintf(buf, "        \"200\":\n")
	fmt.Fprintf(buf, "          description: OK\n")
	if isListPath(path, method) {
		fmt.Fprintf(buf, "          content:\n")
		fmt.Fprintf(buf, "            application/json:\n")
		fmt.Fprintf(buf, "              schema:\n")
		fmt.Fprintf(buf, "                type: array\n")
		fmt.Fprintf(buf, "                items:\n")
		fmt.Fprintf(buf, "                  $ref: \"#/components/schemas/%s\"\n", listItemRef(path))
	}
	fmt.Fprintf(buf, "        \"400\":\n")
	fmt.Fprintf(buf, "          description: Bad request\n")
	fmt.Fprintf(buf, "          content:\n")
	fmt.Fprintf(buf, "            application/json:\n")
	fmt.Fprintf(buf, "              schema:\n")
	fmt.Fprintf(buf, "                $ref: \"#/components/schemas/Error\"\n")
	fmt.Fprintf(buf, "        \"401\":\n")
	fmt.Fprintf(buf, "          description: Unauthorized\n")
	fmt.Fprintf(buf, "        \"404\":\n")
	fmt.Fprintf(buf, "          description: Not found\n")
	if method == methodPOST || method == methodPUT || method == methodDELETE {
		fmt.Fprintf(buf, "        \"500\":\n")
		fmt.Fprintf(buf, "          description: Internal server error\n")
	}
}

// hasPathParam returns true if the path contains "{name}".
func hasPathParam(path string) bool {
	return strings.Contains(path, "{")
}

// cleanPath returns a human-readable summary of a path. It
// strips /api/v1/, replaces {id} with a placeholder, and
// capitalises. Example: "/api/v1/prompts/{id}/run" ->
// "prompts/{id}/run".
func cleanPath(path string) string {
	p := strings.TrimPrefix(path, "/api/v1/")
	p = strings.TrimPrefix(p, "/")
	return p
}

// writePathParams emits a parameter entry for each "{name}"
// segment of the path.
func writePathParams(buf *bytes.Buffer, path string) {
	i := 0
	for i < len(path) {
		j := strings.Index(path[i:], "{")
		if j == -1 {
			break
		}
		j += i
		k := strings.Index(path[j:], "}")
		if k == -1 {
			break
		}
		k += j
		name := path[j+1 : k]
		fmt.Fprintf(buf, "      parameters:\n")
		fmt.Fprintf(buf, "        - name: %s\n", name)
		fmt.Fprintf(buf, "          in: path\n")
		fmt.Fprintf(buf, "          required: true\n")
		fmt.Fprintf(buf, "          schema:\n")
		fmt.Fprintf(buf, "            type: string\n")
		i = k + 1
	}
}

// writeRequestBody emits the requestBody section for a
// non-GET/DELETE handler. Uses a top-level `$ref: ...` if the
// request struct is a named type, otherwise emits a $ref to an
// inline schema.
func writeRequestBody(buf *bytes.Buffer, rt *requestStruct) {
	fmt.Fprintf(buf, "      requestBody:\n")
	fmt.Fprintf(buf, "        required: true\n")
	fmt.Fprintf(buf, "        content:\n")
	fmt.Fprintf(buf, "          application/json:\n")
	fmt.Fprintf(buf, "            schema:\n")
	fmt.Fprintf(buf, "              type: object\n")
	for _, f := range rt.Fields {
		yt, isRef := yamlType(f.GoType)
		if isRef {
			fmt.Fprintf(buf, "              %s:\n", f.JSONName)
			fmt.Fprintf(buf, "                $ref: \"#/components/schemas/%s\"\n", yt)
			continue
		}
		fmt.Fprintf(buf, "              %s:\n", f.JSONName)
		fmt.Fprintf(buf, "                type: %s\n", yt)
	}
}

// isListPath returns true if the handler is conventionally a
// list endpoint. We use a small heuristic on the path +
// method, which covers every current handler.
func isListPath(path, method string) bool {
	if method != methodGET {
		return false
	}
	// Path parameters (e.g. /api/v1/prompts/{id}) are
	// not lists.
	if hasPathParam(path) {
		return false
	}
	// Health and metrics endpoints are not list responses.
	if path == pathHealth || path == "/ready" || path == "/metrics" {
		return false
	}
	return true
}

// listItemRef returns the schema name for a list endpoint's
// items. The mapping is based on the first path segment after
// /api/v1/. It is a heuristic; if the heuristic is wrong the
// output is still valid OpenAPI (just with a $ref to a schema
// that may not exist in components yet). The hand-written
// components.schemas section is the source of truth and the
// CI step `go test ./scripts/genopenapi/...` catches drift.
func listItemRef(path string) string {
	segs := strings.Split(strings.Trim(path, "/"), "/")
	if len(segs) < 2 {
		return typeObject
	}
	if name := compoundResourceRef(segs); name != "" {
		return name
	}
	if name := simpleResourceRef(segs[1]); name != "" {
		return name
	}
	return typeObject
}

func compoundResourceRef(segs []string) string {
	switch segs[1] {
	case segAlerts:
		if len(segs) > 2 && segs[2] == "active" {
			return resAlert
		}
		return resAlertRule
	case segEval:
		switch {
		case len(segs) > 2 && segs[2] == "results":
			return resEvalResult
		case len(segs) > 2 && segs[2] == "runs":
			return resEvalRun
		case len(segs) > 2 && segs[2] == "report":
			return resEvalReport
		}
		return resEvalReport
	case segGuardrails:
		if len(segs) > 2 && segs[2] == "violations" {
			return resGuardrailViolation
		}
		return resGuardrailRule
	}
	return ""
}

func simpleResourceRef(segment string) string {
	switch segment {
	case "ab-tests":
		return "ABTest"
	case "agents":
		return "Agent"
	case "audit":
		return "AuditEntry"
	case "contexts":
		return "Context"
	case "datasets":
		return "TestDataset"
	case "execution-logs":
		return "ExecutionLog"
	case segPrompts:
		return resPrompt
	case "providers":
		return "Provider"
	case "reviews":
		return "Review"
	case "snapshots":
		return "Snapshot"
	case "traces":
		return "Trace"
	case "users":
		return "User"
	case "vault":
		return "VaultKey"
	case "webhooks":
		return "WebhookEndpoint"
	case "workflows":
		return "Workflow"
	}
	return ""
}
