// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package parser

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/doc"
	"go/token"
	"os"
	"slices"
	"strings"
	"sync"
	"unicode"

	"github.com/ctx42/ring/pkg/ring"
)

// genMakefileUser parses targets in `makefile*.go` files in given directory.
// Returns source code for the mkf.MakefileUser file and the matching targets
// collection.
func genMakefileUser(rng *ring.Ring, dir string) ([]byte, *Targets, error) {
	// Parse makefile*.go files with user defined targets.
	pmf, err := NewMakefile(rng, dir)
	if err != nil {
		return nil, nil, err
	}
	// Generate source for user targets.
	gen := NewGenerator(pmf.Targets)
	code, err := gen.Generate(WithGenNames(MainName, "User"), WithGenReg)
	if err != nil {
		return nil, nil, err
	}
	return code, pmf.Targets, nil
}

// GenMakefileUserAndSave generates the mkf.MakefileUser code for the targets
// in the src directory and writes it to the file dst. Returns the user
// targets.
func GenMakefileUserAndSave(rng *ring.Ring, src, dst string) (*Targets, error) {
	code, tgs, err := genMakefileUser(rng, src)
	if err != nil {
		return nil, err
	}
	if err = CreateFile(dst, code); err != nil {
		return nil, err
	}
	return tgs, nil
}

// GetBuildTag returns metaBuildTag from the [ring.Ring] or empty string if
// the key does not exist.
func GetBuildTag(rng *ring.Ring) string {
	if v, ok := rng.MetaLookup(metaBuildTag); ok {
		if bt, ok := v.(string); ok {
			return bt
		}
	}
	return ""
}

// SetBuildTag sets metaBuildTag on [ring.Ring] to [BuildTag].
func SetBuildTag(rng *ring.Ring) *ring.Ring {
	rng.MetaSet(metaBuildTag, BuildTag)
	return rng
}

// RemBuildTag removes metaBuildTag from the ring metadata.
func RemBuildTag(rng *ring.Ring) *ring.Ring {
	rng.MetaDelete(metaBuildTag)
	return rng
}

// toOneLine replaces all new line characters in s with space.
func toOneLine(s string) string {
	return strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
}

// removeNoLintComments removes lines starting with "nolint" from s, the
// resulting string is truncated. Returns original string when "nolint" is
// not found. Must run before the doc is collapsed to a single line.
func removeNoLintComments(s string) string {
	s, _ = removeLines(s, "nolint")
	return s
}

// isHidden checks string contains line with hiddenTag. Returns string with
// line containing hiddenTag removed and if the line was removed.
func isHidden(s string) (string, bool) {
	s, n := removeLines(s, hiddenTag)
	if n > 0 {
		return s, true
	}
	return s, false
}

// removeLines removes lines starting with start string from s, the result
// string is truncated. Returns string with lines removed and number of removed
// lines. If nothing was removed it returns original string and zero.
func removeLines(s, start string) (string, int) {
	buf := strings.Builder{}
	var removed int
	for lin := range strings.Lines(s) {
		if strings.HasPrefix(lin, start) {
			removed++
			continue
		}
		// Drop the line ending, CRLF included.
		lin = strings.TrimSuffix(strings.TrimSuffix(lin, "\n"), "\r")
		buf.WriteString(lin)
		buf.WriteString("\n")
	}
	return strings.TrimSpace(buf.String()), removed
}

// targetSynopsis returns the first sentence of the target documentation
// docStr without its closing period. The caller strips the function name the
// documentation starts with, so a word repeating the name is kept.
//
// Example: for the documentation "does stuff. Some more stuff." the synopsis
// is "does stuff".
func targetSynopsis(docStr string) string {
	var p doc.Package
	return strings.TrimRight(p.Synopsis(docStr), ".")
}

// breadcrumbs returns the type's breadcrumb trail or nil. See the isNS
// documentation for more info.
//
// The namespace path for C:
//
//	type A struct{} //gomake:ns_root
//	type B A
//	type C B
//
// is:
//
//	[]string{"__root__", "A", "B", "C"}
//
// Another example:
//
//	[]string{"__!!!__", "NS3", "NS4"}
//
// When the first element in the slice is "__!!!__":
//
//   - the slice is guaranteed to have at least three elements,
//   - the breadcrumbs trail is incomplete and namespace root is probably
//     in a different file or package than the type,
//   - the type "NS4" is not a namespace.
func breadcrumbs(typ *doc.Type) []string {
	if len(typ.Decl.Specs) != 1 {
		return nil
	}
	prev := []string{typ.Name}
	typSpec, ok := typ.Decl.Specs[0].(*ast.TypeSpec)
	if !ok {
		return nil
	}
	pth := isNS(typSpec, prev)
	for i, j := 0, len(pth)-1; i < j; i, j = i+1, j-1 {
		pth[i], pth[j] = pth[j], pth[i]
	}
	return pth
}

// Breadcrumb markers delimit a target name trail.
const (
	// partCrumb represents breadcrumb element indicating incomplete trail. It
	// happens when the type is based on a type defined in a different file or
	// that the type is not a namespace.
	//
	// Example:
	//
	// first.go:
	//
	//	type First struct{} //gomake:ns_root
	//
	// second.go:
	//
	//	type Second First
	partCrumb = "__!!!__"

	// rootCrumb is the first (root) breadcrumb element in the target's
	// breadcrumb trail. All valid trails must start with it.
	rootCrumb = "__root__"
)

// isNS returns breadcrumb trail for the given type or nil if it isn't
// namespace. When non-nil slice is returned the first element in it may be:
//
//   - rootCrumb: means the breadcrumb trail is complete (valid namespace).
//   - partCrumb: means the breadcrumb trail describes a potential namespace,
//     which means additional checks are needed. It happens for example when
//     the base type is in different file or package.
func isNS(spc *ast.TypeSpec, prev []string) []string {
	_, ok := spc.Type.(*ast.SelectorExpr)
	if ok {
		return nil
	}
	// Generated code declares a namespace variable without type arguments,
	// so a generic type cannot be a namespace.
	if spc.TypeParams != nil && len(spc.TypeParams.List) > 0 {
		return nil
	}
	switch typ := spc.Type.(type) {
	case *ast.Ident:
		if typ.Obj != nil {
			if spc2, ok2 := typ.Obj.Decl.(*ast.TypeSpec); ok2 {
				name := spc2.Name.Name
				if slices.Contains(prev, name) {
					// Cyclic type chain (e.g. `type A B; type B A`); not a
					// namespace, and recursing would never terminate.
					return nil
				}
				prev = append(prev, name)
				return isNS(spc2, prev)
			}
		}
		if isBuiltinType(typ.Name) {
			return nil
		}
		return append(prev, typ.Name, partCrumb)

	case *ast.StructType:
		if isNSRoot(spc) {
			return append(prev, rootCrumb)
		}
	}
	return nil
}

// isNSRoot returns true when a type spec has a `gomake:ns_root` comment
// marking the type as a namespace root. The comment may be an end-of-line
// comment or a doc comment above the type. Accepts optional space after //
// (//gomake:ns_root or // gomake:ns_root).
func isNSRoot(spc *ast.TypeSpec) bool {
	if spc == nil {
		return false
	}
	lines := append(commentBodies(spc.Comment), commentBodies(spc.Doc)...)
	for _, text := range lines {
		if text == nsTag || strings.HasPrefix(text, nsTag+" ") {
			return true
		}
	}
	return false
}

// attachDeclDoc copies a one-spec declaration's doc comment onto that spec
// when the spec has none. go/parser leaves the comment on the GenDecl for
// `//gomake:ns_root` above `type Foo struct{}` and the same shape of import.
func attachDeclDoc(f *ast.File) {
	if f == nil {
		return
	}
	for _, dcl := range f.Decls {
		gen, ok := dcl.(*ast.GenDecl)
		if !ok || gen.Doc == nil || len(gen.Specs) != 1 {
			continue
		}
		switch spc := gen.Specs[0].(type) {
		case *ast.TypeSpec:
			if spc.Doc == nil {
				spc.Doc = gen.Doc
			}

		case *ast.ImportSpec:
			if spc.Doc == nil {
				spc.Doc = gen.Doc
			}
		}
	}
}

// commentBodies returns each // comment with the prefix and surrounding
// space removed. A nil group returns nil.
func commentBodies(grp *ast.CommentGroup) []string {
	if grp == nil {
		return nil
	}
	out := make([]string, 0, len(grp.List))
	for _, cmt := range grp.List {
		text := strings.TrimSpace(cmt.Text)
		text = strings.TrimPrefix(text, "//")
		out = append(out, strings.TrimSpace(text))
	}
	return out
}

// isBuiltinType returns true if a type is a predeclared basic type.
func isBuiltinType(typeName string) bool {
	switch typeName {
	case "bool", "string", "byte", "rune", "uintptr",
		"int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64",
		"float32", "float64", "complex64", "complex128":
		return true
	}
	return false
}

// toKebabCase converts a CamelCase name to kebab-case.
//
//nolint:cyclop,gocognit
func toKebabCase(camel string) string {
	// Character types.
	const (
		UNK = iota // Unknown.
		LC         // Lowercase.
		UC         // Uppercase.
		NUM        // Number.
	)

	var b bytes.Buffer
	var ppt, pt, t int
	var pv, v rune

	for _, v = range camel {
		t = UNK
		if v >= 48 && v <= 57 { // Number.
			t = NUM
		}
		if v >= 97 && v <= 122 { // Lowercase.
			t = LC
		}
		if v >= 65 && v <= 90 { // Uppercase.
			t = UC
			v = unicode.ToLower(v)
		}

		switch {
		// Number followed by letter.
		case pt == NUM && (t == LC || t == UC):
			b.WriteRune('-')

		// Lowercase followed by uppercase.
		case pt == LC && t == UC:
			b.WriteRune('-')

		// Few uppercase followed by lowercase.
		case ppt == UC && pt == UC && t == LC:
			b.Truncate(b.Len() - 1)
			b.WriteRune('-')
			b.WriteRune(pv)
		}

		b.WriteRune(v)
		ppt = pt
		pt = t
		pv = v
	}
	return b.String()
}

// importLocalNames maps the import local names of fil to their import paths.
// Named imports use the name; unnamed imports use the last path segment. Dot
// and blank imports are skipped.
func importLocalNames(fil *ast.File) map[string]string {
	out := make(map[string]string)
	for _, imp := range fil.Imports {
		path := unquote(imp.Path)
		if path == "" {
			continue
		}
		if imp.Name != nil {
			switch imp.Name.Name {
			case "_", ".":
				continue

			default:
				out[imp.Name.Name] = path
				continue
			}
		}
		// Default local name is the last path element.
		base := path
		if i := strings.LastIndex(path, "/"); i >= 0 {
			base = path[i+1:]
		}
		out[base] = path
	}
	return out
}

// topLevelNames returns the package-block identifiers declared in files:
// functions (not methods), types, variables, and constants.
func topLevelNames(files map[string]*ast.File) []string {
	var names []string
	for _, fil := range files {
		for _, dcl := range fil.Decls {
			switch dcl := dcl.(type) {
			case *ast.FuncDecl:
				if dcl.Recv == nil {
					names = append(names, dcl.Name.Name)
				}

			case *ast.GenDecl:
				for _, spec := range dcl.Specs {
					switch spec := spec.(type) {
					case *ast.TypeSpec:
						names = append(names, spec.Name.Name)

					case *ast.ValueSpec:
						for _, name := range spec.Names {
							names = append(names, name.Name)
						}
					}
				}
			}
		}
	}
	return names
}

// fileImports maps the import local names of the file in files containing pos
// to their import paths. Import names are file-scoped, so a declaration sees
// only the imports of its own file. Returns nil when no file contains pos.
func fileImports(files map[string]*ast.File, pos token.Pos) map[string]string {
	for _, fil := range files {
		if fil != nil && fil.FileStart <= pos && pos < fil.FileEnd {
			return importLocalNames(fil)
		}
	}
	return nil
}

// gmImpSpec returns import namespace and import spec only for specs tagged
// with `gomake:import`. Otherwise it returns two empty strings. The tag may
// be an end-of-line comment or a doc comment above the spec; an end-of-line
// tag wins when both are present. The namespace is always lowercase
// regardless of how it was written in the source. A tag with more than one
// namespace word returns [ErrImportTag].
func gmImpSpec(is *ast.ImportSpec) (string, string, error) {
	if is == nil {
		return "", "", nil
	}
	lines := append(commentBodies(is.Comment), commentBodies(is.Doc)...)
	for _, text := range lines {
		fields := strings.Fields(text)
		if len(fields) == 0 || fields[0] != importTag {
			continue
		}
		path := unquote(is.Path)
		switch len(fields) {
		case 1:
			return "", path, nil // Import without namespace.
		case 2:
			return strings.ToLower(fields[1]), path, nil // With namespace.
		default:
			return "", "", fmt.Errorf("%w: %q: %s", ErrImportTag, path, text)
		}
	}
	return "", "", nil
}

// unquote removes quotes from the beginning and the end of a string.
//
// Examples:
//
//	"str" -> str
//	`str` -> str
func unquote(bl *ast.BasicLit) string { return strings.Trim(bl.Value, "\"`") }

// gmImpPackages returns packages imported with a `gomake:import` comment,
// each namespace and import path pair once. Packages are resolved
// concurrently against the go.mod of the module rooted at dir (empty dir falls
// back to the process working directory).
func gmImpPackages(
	rng *ring.Ring,
	dir string,
	dcs ...ast.Decl,
) ([]*Package, error) {

	type impSpec struct {
		ns  string
		imp string
	}
	var specs []impSpec
	seen := make(map[impSpec]bool)
	for _, del := range dcs {
		gen, ok := del.(*ast.GenDecl)
		if !ok || gen.Tok != token.IMPORT {
			continue
		}
		for _, spec := range gen.Specs {
			ispec := spec.(*ast.ImportSpec) //nolint:forcetypeassert
			pkgNS, imp, err := gmImpSpec(ispec)
			if err != nil {
				return nil, err
			}
			if imp == "" {
				continue
			}
			key := impSpec{ns: pkgNS, imp: imp}
			if seen[key] {
				continue // Tagged in more than one file.
			}
			seen[key] = true
			specs = append(specs, key)
		}
	}
	if len(specs) == 0 {
		return nil, nil
	}

	pks := make([]*Package, len(specs))
	errc := make(chan error, len(specs))
	var wg sync.WaitGroup
	for i, s := range specs {
		wg.Add(1)
		go func(i int, ns, imp string) {
			defer wg.Done()
			pkg, err := NewPackage(
				rng,
				imp,
				withPkgSpec,
				withPkgNS(ns),
				withPkgDir(dir),
			)
			if err != nil {
				errc <- err
				return
			}
			pks[i] = pkg
		}(i, s.ns, s.imp)
	}
	wg.Wait()
	close(errc)
	if err := <-errc; err != nil {
		return nil, err
	}
	return pks, nil
}

// findDefault returns the code reference assigned to the variable named
// Default and the position of its declaration. Returns nil when the variable
// is not declared or not set.
func findDefault(vars ...*doc.Value) ([]string, token.Pos) {
	for _, v := range vars {
		for _, spec := range v.Decl.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for j, name := range vs.Names {
				if name.Name != "Default" {
					continue
				}
				if len(vs.Values) == 0 {
					return nil, token.NoPos // Declared without an initializer.
				}
				// Shared initializer for multi-name specs uses Values[0];
				// per-name values use the matching index when present.
				idx := 0
				if j < len(vs.Values) {
					idx = j
				}
				return codeRef(vs.Values[idx], nil), vs.Pos()
			}
		}
	}
	return nil, token.NoPos
}

// codeRef takes Go expression and returns its code reference. Function calls
// itself recursively so the first call should set prev to nil.
//
//nolint:gocritic
func codeRef(expr ast.Expr, prev []string) []string {
	switch v := expr.(type) {
	case *ast.Ident:
		// var Default = Target
		prev = append(prev, v.Name)

	case *ast.SelectorExpr:
		// var Default = NS.Target
		// var Default = pkg.Target
		// var Default = pkg.NS.Target
		prev = codeRef(v.X, prev)
		prev = codeRef(v.Sel, prev)
	}
	return prev
}

// qIdent returns qualified identifier for given expression. Returns empty
// string if it does not know how to build it.
func qIdent(expr ast.Expr) string {
	if st, ok := expr.(*ast.StarExpr); ok {
		return qIdent(st.X)
	}
	switch v := expr.(type) {
	case *ast.Ident:
		return v.Name

	case *ast.SelectorExpr:
		if sel, ok := v.X.(*ast.Ident); ok {
			return sel.Name + "." + v.Sel.Name
		}

	case *ast.ArrayType:
		return "[]" + qIdent(v.Elt)
	}
	return ""
}

// CreateFile writes code to the file at dst.
func CreateFile(dst string, code []byte) error {
	if err := os.WriteFile(dst, code, 0600); err != nil {
		return fmt.Errorf("write %s: %w", dst, err)
	}
	return nil
}
