// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package parser

import (
	"bufio"
	"bytes"
	"go/ast"
	"go/doc"
	"go/token"
	"slices"
	"strings"
	"sync"
	"unicode"

	"github.com/ctx42/ring/pkg/ring"
)

// genMakefileUser parses targets in `makefile*.go` files in given directory.
// Returns source code for [mkf.MakefileUser] file and matching targets
// collection.
func genMakefileUser(
	rng *ring.Ring,
	dir string,
) ([]byte, *Targets, error) {

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

// GenMakefileUserAndSave based on targets in src directory generates code for
// [mkf.MakefileUser] in the same directory. Returns user targets.
func GenMakefileUserAndSave(
	rng *ring.Ring,
	src, dst string,
) (*Targets, error) {

	code, tgs, err := genMakefileUser(rng, src)
	if err != nil {
		return nil, err
	}
	if err = CreateFile(dst, code); err != nil {
		return nil, err
	}
	return tgs, nil
}

// GetBuildTag returns [metaBuildTag] from the [ring.Ring] or empty string if
// the key does not exist.
func GetBuildTag(rng *ring.Ring) string {
	if v, ok := rng.MetaLookup(metaBuildTag); ok {
		if bt, ok := v.(string); ok {
			return bt
		}
	}
	return ""
}

// SetBuildTag sets [metaBuildTag] on [ring.Ring] to [BuildTag].
func SetBuildTag(rng *ring.Ring) *ring.Ring {
	rng.MetaSet(metaBuildTag, BuildTag)
	return rng
}

// RemBuildTag removes [metaBuildTag] from `rng` metadata.
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
	scn := bufio.NewScanner(strings.NewReader(s))
	var removed int
	for scn.Scan() {
		lin := scn.Text()
		if strings.HasPrefix(lin, start) {
			removed++
			continue
		}
		buf.WriteString(lin)
		buf.WriteString("\n")
	}
	if err := scn.Err(); err != nil {
		return s, 0
	}
	return strings.TrimSpace(buf.String()), removed
}

// targetSynopsis sanitizes target function documentation and creates a summary.
// Summary is defined as a first sentence in the target function documentation
// without the name of the function itself.
//
// Example:
//
//	// TargetName does stuff. Some more stuff.
//	// A lot of more stuff.
//	func TargetName(ctx context.Context, rng *ring.Ring) error {}
//
// Summary for the above example would be "does stuff.".
func targetSynopsis(name, docStr string) string {
	var p doc.Package
	syn := p.Synopsis(docStr)
	tokens := strings.Split(syn, " ")
	if strings.EqualFold(name, tokens[0]) {
		syn = strings.Join(tokens[1:], " ")
	}
	return strings.TrimRight(syn, ".")
}

// breadcrumbs returns type's breadcrumb trail or nil. See [isNS] documentation
// for more info.
//
// The namespace path for C:
//
//	type A struct{} //gomake:ns_root
//	type B A
//	type C B
//
// is
//
//	[]string{"__root__", "A", "B", "C"}
//
// other examples:
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

// partCrumb represents breadcrumb element indicating incomplete trail. It
// happens when the type is based on a type defined in a different file or that
// the type is not a namespace.
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
const partCrumb = "__!!!__"

// rootCrumb is the first (root) breadcrumb element in the target's breadcrumb
// trail. All valid trails must start with it.
const rootCrumb = "__root__"

// isNS returns breadcrumb trail for the given type or nil if it isn't
// namespace. When non-nil slice is returned the first element in it may be:
//
//   - [rootCrumb]: means the breadcrumb trail is complete (valid namespace).
//   - [partCrumb]: means the breadcrumb trail describes potential namespace -
//     which means additional checks are need. It happens for example when
//     the base type is in different file or package.
func isNS(spc *ast.TypeSpec, prev []string) []string {
	_, ok := spc.Type.(*ast.SelectorExpr)
	if ok {
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
// marking the type as a namespace root. Accepts optional space after //
// (//gomake:ns_root or // gomake:ns_root).
func isNSRoot(spc *ast.TypeSpec) bool {
	if spc == nil || spc.Comment == nil || len(spc.Comment.List) == 0 {
		return false
	}
	text := strings.TrimSpace(spc.Comment.List[0].Text)
	text = strings.TrimPrefix(text, "//")
	text = strings.TrimSpace(text)
	return text == nsTag || strings.HasPrefix(text, nsTag+" ")
}

// builtinTypes represents a list of build in types.
var builtinTypes = map[string]bool{
	"bool":       true,
	"string":     true,
	"int":        true,
	"int8":       true,
	"int16":      true,
	"int32":      true,
	"int64":      true,
	"uint":       true,
	"uint8":      true,
	"uint16":     true,
	"uint32":     true,
	"uint64":     true,
	"uintptr":    true,
	"float32":    true,
	"float64":    true,
	"complex64":  true,
	"complex128": true,
	"byte":       true,
	"rune":       true,
}

// isBuiltinType returns true if a type is built-in.
func isBuiltinType(typeName string) bool {
	_, ok := builtinTypes[typeName]
	return ok
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

// importLocalNames maps import local names to import paths for all imports in
// the given files. Named imports use the name; unnamed imports use the last
// path segment. Dot and blank imports are skipped.
func importLocalNames(files map[string]*ast.File) map[string]string {
	out := make(map[string]string)
	for _, fil := range files {
		if fil == nil {
			continue
		}
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
	}
	return out
}

// gmImpSpec returns import namespace and import spec only for specs tagged
// with `gomake:import`. Otherwise it returns two empty strings. The namespace
// is always lowercase regardless of how it was written in the source.
func gmImpSpec(is *ast.ImportSpec) (string, string) {
	// No comment, no gomake tag.
	if is.Comment == nil || len(is.Comment.List) == 0 {
		return "", ""
	}

	// Get tag and remove leading '//'.
	tag := is.Comment.List[0].Text[2:]
	fields := strings.Fields(tag)
	if len(fields) == 0 || fields[0] != importTag {
		return "", ""
	}

	path := unquote(is.Path)
	switch len(fields) {
	case 1:
		return "", path // Import without namespace.
	case 2:
		return strings.ToLower(fields[1]), path // Import with namespace.
	default:
		return "", ""
	}
}

// unquote removes quotes from the beginning and the end of a string.
//
// Examples:
//
//	"str" -> str
//	`str` -> str
func unquote(bl *ast.BasicLit) string { return strings.Trim(bl.Value, "\"`") }

// gmImpPackages returns packages imported with a `gomake:import` comment.
// Packages are resolved concurrently against the go.mod of the module rooted
// at dir (empty dir falls back to the process working directory).
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
	for _, del := range dcs {
		gen, ok := del.(*ast.GenDecl)
		if !ok || gen.Tok != token.IMPORT {
			continue
		}
		for _, spec := range gen.Specs {
			//nolint:forcetypeassert
			pkgNS, imp := gmImpSpec(spec.(*ast.ImportSpec))
			if imp == "" {
				continue
			}
			specs = append(specs, impSpec{ns: pkgNS, imp: imp})
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

// findDefault returns expression assigned to variable named Default. Returns
// nil when variable is not declared or not set.
func findDefault(vars ...*doc.Value) []string {
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
					return nil // Declared without an initializer.
				}
				// Shared initializer for multi-name specs uses Values[0];
				// per-name values use the matching index when present.
				idx := 0
				if j < len(vs.Values) {
					idx = j
				}
				return codeRef(vs.Values[idx], nil)
			}
		}
	}
	return nil
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
