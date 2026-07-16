// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package parser

import (
	"bytes"
	"fmt"
	"go/doc"
	"sort"
	"strings"

	"github.com/ctx42/ring/pkg/ring"

	"github.com/ctx42/gomake/internal/mkf"
)

// TgsMapCB is callback signature for [Targets.Map] method.
type TgsMapCB func(*Targets, *mkf.Target)

// Targets represent a collection of unique gomake targets.
type Targets struct {
	unique map[string]struct{} // Map of target names (for uniqueness).
	list   []*mkf.Target       // List of unique targets (sorted).
	sorted bool                // True when list is sorted.
}

// NewTargets returns new instance of Targets.
func NewTargets() *Targets {
	return &Targets{
		unique: make(map[string]struct{}, 10),
		list:   make([]*mkf.Target, 0, 10),
	}
}

// TargetsFromList uses [NewTargets] to create a new instance and adds the
// targets to it. If there are duplicates, it returns an [ErrDupTarget] error.
func TargetsFromList(ts ...*mkf.Target) (*Targets, error) {
	tgs := NewTargets()
	if err := tgs.Add(ts...); err != nil {
		return nil, err
	}
	return tgs, nil
}

// Import identifies an external target package to compile in. Namespace, when
// non-empty, prefixes every target name the package contributes.
type Import struct {
	// Path is the Go import path (spec) of the target package.
	Path string

	// Namespace prefixes the names of all targets from the package. Empty
	// registers the targets in the root namespace.
	Namespace string
}

// TargetsFromSpecs returns a list of targets in given import specs.
// For targets from each spec it applies provided call back function(s) and
// then merges them into one list of targets. All targets are registered in the
// root namespace; use [TargetsFromImports] to apply a namespace.
func TargetsFromSpecs(
	rng *ring.Ring,
	specs []string,
	fns ...TgsMapCB,
) (*Targets, error) {

	imports := make([]Import, len(specs))
	for i, spec := range specs {
		imports[i] = Import{Path: spec}
	}
	return TargetsFromImports(rng, "", imports, fns...)
}

// TargetsFromImports returns the targets found in the given import packages.
// Each import's namespace is applied to its targets, the callbacks fns are
// applied to every target, and the results are merged into one list. The specs
// are resolved against the go.mod of the module rooted at dir; an empty dir
// falls back to the process working directory.
func TargetsFromImports(
	rng *ring.Ring,
	dir string,
	imports []Import,
	fns ...TgsMapCB,
) (*Targets, error) {

	all := NewTargets()
	for _, imp := range imports {
		pkg, err := NewPackage(
			rng,
			imp.Path,
			withPkgSpec,
			withPkgNS(imp.Namespace),
			withPkgDir(dir),
		)
		if err != nil {
			return nil, err
		}
		pmf, err := MakefileFromPackage(rng, pkg)
		if err != nil {
			return nil, err
		}
		pmf.Targets.Map(fns...)
		if err = all.Add(pmf.Targets.List()...); err != nil {
			return nil, err
		}
	}
	return all, nil
}

// Len returns number of targets in the collection.
func (tgs *Targets) Len() int {
	return len(tgs.list)
}

// Add adds new target(s) to the collection. Returns [ErrDupTarget] when target
// is already in the collection.
func (tgs *Targets) Add(ts ...*mkf.Target) error {
	for _, tgt := range ts {
		if got := tgs.Get(tgt.Name); got != nil {
			format := "%w: %s, %s"
			return fmt.Errorf(format, ErrDupTarget, got.DefRef, tgt.DefRef)
		}
		tgs.unique[tgt.Name] = struct{}{}
		tgs.list = append(tgs.list, tgt)
		tgs.sorted = false
	}
	return nil
}

// Has returns true if target name exists in the collection.
func (tgs *Targets) Has(name string) bool {
	_, ok := tgs.unique[name]
	return ok
}

// pkgNameForImp returns the package name of the first target from ImpSpec, or
// empty when no such target is registered.
func (tgs *Targets) pkgNameForImp(impSpec string) string {
	for _, tgt := range tgs.list {
		if tgt.ImpSpec == impSpec && tgt.PkgName != "" {
			return tgt.PkgName
		}
	}
	return ""
}

// addFunc adds function(s) (targets) belonging to given package to the
// collection. If a function doesn't have compatible signature it will be
// skipped without error. Method returns [ErrDupTarget] when duplicate target
// is detected. Targets are considered a duplicate if their Name fields are
// the same.
func (tgs *Targets) addFunc(pkg *Package, fns ...*doc.Func) error {
	for _, fn := range fns {
		if tgt, err := newTarget(pkg, fn); err == nil {
			if err = tgs.Add(tgt); err != nil {
				return err
			}
		}
	}
	return nil
}

// addType adds targets (methods) from the given type (namespace) in given
// package. If the type is not namespace it's skipped, the same applies to
// the methods of the type. Method returns [ErrDupTarget] when duplicate target
// is detected. Targets are considered a duplicate if their Name fields are
// the same.
func (tgs *Targets) addType(pkg *Package, tps ...*doc.Type) error {
	// Map of incomplete namespace paths (types) where keys are missing
	// receiver names (types). Incomplete type is a type which namespace path
	// does not go all the way from rootNS to the type (namespace) itself.
	inc := make(map[string][]*doc.Type)
	// Matching namespace paths for above types.
	pth := make(map[string][][]string)
	// Complete namespace trails by type name, including methodless ns roots,
	// so cross-file children can resolve without an existing method target.
	complete := make(map[string][]string)

	for _, typ := range tps {
		nsp := breadcrumbs(typ)
		if len(nsp) == 0 {
			continue
		}
		if nsp[0] == partCrumb {
			rel := nsp[1] // The namespace name this namespace extends.
			inc[rel] = append(inc[rel], typ)
			pth[rel] = append(pth[rel], nsp[2:])
			continue
		}
		complete[typ.Name] = nsp
		if err := tgs.addMethods(pkg, typ.Methods, nsp...); err != nil {
			return err
		}
	}

	prev := len(inc) // Number of incomplete types.
	for len(inc) > 0 {
		for rcv, types := range inc {
			nspParent, ok := complete[rcv]
			if !ok {
				if tgt := tgs.withReceiver(rcv); tgt != nil {
					nspParent = tgt.Breadcrumbs
					ok = true
				}
			}
			if !ok {
				continue
			}
			for i, typ := range types {
				nsp := append([]string{}, nspParent...)
				nsp = append(nsp, pth[rcv][i]...)
				complete[typ.Name] = nsp
				if err := tgs.addMethods(pkg, typ.Methods, nsp...); err != nil {
					return err
				}
			}
			delete(inc, rcv)
		}
		cur := len(inc)
		if prev == cur {
			break // If we did not resolve any types exit.
		}
		prev = cur
	}
	return nil
}

// addMethods adds methods (targets) with given namespace path. Returns
// [ErrDupTarget] when there are duplicated targets on the list.
func (tgs *Targets) addMethods(
	pkg *Package,
	fns []*doc.Func,
	nsp ...string,
) error {

	for _, met := range fns {
		if tgt, err := newTarget(pkg, met, nsp...); err == nil {
			if err = tgs.Add(tgt); err != nil {
				return err
			}
		}
	}
	return nil
}

// withReceiver returns the first target information instance with given
// [mkf.Target.Receiver].
func (tgs *Targets) withReceiver(name string) *mkf.Target {
	if name == "" {
		return nil
	}
	for _, tgt := range tgs.list {
		if name == tgt.Receiver {
			return tgt
		}
	}
	return nil
}

// Sort sorts the internal list of targets.
func (tgs *Targets) Sort() {
	list := tgs.list
	sort.Slice(list, func(i, j int) bool {
		return list[i].Name < list[j].Name
	})
	tgs.sorted = true
}

// Get returns target by its name or nil if it's not in the collection.
func (tgs *Targets) Get(tgtName string) *mkf.Target {
	if _, ok := tgs.unique[tgtName]; ok {
		for _, tgt := range tgs.list {
			if tgt.Name == tgtName {
				return tgt
			}
		}
	}
	return nil
}

// List returns alphabetically sorted list of targets by their names.
func (tgs *Targets) List() []*mkf.Target {
	if !tgs.sorted {
		tgs.Sort()
	}
	list := make([]*mkf.Target, len(tgs.list))
	copy(list, tgs.list)
	return list
}

// Names returns the list of target names in the collection.
func (tgs *Targets) Names() []string {
	list := tgs.List()
	names := make([]string, len(list))
	for i, tgt := range list {
		names[i] = tgt.Name
	}
	return names
}

// MarkDefault marks the first target whose DefRef matches as default and
// returns its name. Returns empty string if DefRef does not match any target.
// Other targets' Default flags are cleared so at most one default remains.
func (tgs *Targets) MarkDefault(defRef string) string {
	def := ""
	for _, tgt := range tgs.list {
		tgt.Default = false
		if def == "" && tgt.DefRef == defRef {
			def = tgt.Name
			tgt.Default = true
		}
	}
	return def
}

// BuiltInCB is callback function for [Targets.Map] method which marks given
// target as built-in. It clears Default so an external package's var Default
// cannot become the no-arg default of the installed binary.
func BuiltInCB(tgs *Targets, tgt *mkf.Target) {
	delete(tgs.unique, tgt.Name)
	tgt.Name = ":" + tgt.Name
	tgt.Default = false
	tgs.unique[tgt.Name] = struct{}{}
}

// Map runs function(s) on all the targets.
func (tgs *Targets) Map(fns ...TgsMapCB) {
	for _, fn := range fns {
		for _, tgt := range tgs.list {
			fn(tgs, tgt)
		}
	}
}

// importAliasMap returns a map from ImpSpec to the Go import alias that
// generated code must use. When several imports share a package name, later
// ones get a numeric suffix (pkg, pkg2, …). Every alias is unique across the
// whole set so a natural name like "foo2" cannot collide with a suffix alias.
// Entries that keep the default package name map to that name (no explicit
// alias line).
func (tgs *Targets) importAliasMap() map[string]string {
	// ImpSpec -> declared package name (first target wins).
	pkgBySpec := make(map[string]string, 8)
	for _, tgt := range tgs.List() {
		if tgt.PkgName == MainName || tgt.ImpSpec == "" {
			continue
		}
		if _, ok := pkgBySpec[tgt.ImpSpec]; !ok {
			pkgBySpec[tgt.ImpSpec] = tgt.PkgName
		}
	}
	if len(pkgBySpec) == 0 {
		return nil
	}

	// Stable order for deterministic suffixes.
	specs := make([]string, 0, len(pkgBySpec))
	for spec := range pkgBySpec {
		specs = append(specs, spec)
	}
	sort.Strings(specs)

	// alias -> ImpSpec already assigned; seed reserved names used by gen
	// templates and GoCode locals so user packages cannot shadow them.
	taken := map[string]string{
		"context": "reserved",
		"ring":    "reserved",
		"mkf":     "reserved",
		"targets": "reserved",
		"tgt":     "reserved",
	}
	out := make(map[string]string, len(pkgBySpec))
	for _, spec := range specs {
		alias := uniqueImportAlias(pkgBySpec[spec], taken, spec)
		taken[alias] = spec
		out[spec] = alias
	}
	return out
}

// uniqueImportAlias returns base when free, otherwise stem2, stem3, … where
// stem is base with trailing digits stripped so "foo2" does not become "foo22".
func uniqueImportAlias(base string, taken map[string]string, spec string) string {
	if other, ok := taken[base]; !ok || other == spec {
		return base
	}
	stem := strings.TrimRight(base, "0123456789")
	if stem == "" {
		stem = base
	}
	for n := 2; ; n++ {
		alias := fmt.Sprintf("%s%d", stem, n)
		if other, ok := taken[alias]; !ok || other == spec {
			return alias
		}
	}
}

// GoImports returns unique imports tagged with a `gomake:import` comment.
// When two import paths declare the same package name, later imports get an
// explicit alias (see importAliasMap).
func (tgs *Targets) GoImports() string {
	aliases := tgs.importAliasMap()
	var lines []string
	used := make(map[string]struct{}, 10)
	for _, tgt := range tgs.List() {
		// There is no way to import main package so we skip it.
		if tgt.PkgName == MainName {
			continue
		}
		if _, ok := used[tgt.ImpSpec]; ok {
			continue
		}
		if len(lines) == 0 {
			lines = append(lines, "")
		}
		alias := aliases[tgt.ImpSpec]
		var code string
		if alias != "" && alias != tgt.PkgName {
			code = fmt.Sprintf("%s %q", alias, tgt.ImpSpec)
		} else {
			code = fmt.Sprintf("%q", tgt.ImpSpec)
		}
		lines = append(lines, code)
		used[tgt.ImpSpec] = struct{}{}
	}

	if len(lines) > 0 {
		sort.Strings(lines)
		return strings.Join(lines, "\n") + "\n"
	}
	return ""
}

// codePkgName returns the package identifier to use for tgt in generated
// code, applying import aliases when package names collide.
func (tgs *Targets) codePkgName(
	tgt *mkf.Target,
	aliases map[string]string,
) string {

	if tgt.PkgName == MainName || tgt.ImpSpec == "" {
		return tgt.PkgName
	}
	if alias, ok := aliases[tgt.ImpSpec]; ok && alias != "" {
		return alias
	}
	return tgt.PkgName
}

// GoCode returns Go source code defining the targets. If the qt is true the
// package qualifier is added to the Target references ("mkf.Target" vs
// "Target"). Import package-name collisions use aliases from importAliasMap.
func (tgs *Targets) GoCode(qt bool) string {
	corePkg := ""
	if qt {
		corePkg = "mkf."
	}
	aliases := tgs.importAliasMap()
	buf := &bytes.Buffer{}
	var code string
	if n := len(tgs.list); n > 0 {
		code = fmt.Sprintf("targets := make([]*%sTarget, 0, %d)", corePkg, n)
	} else {
		code = fmt.Sprintf("targets := make([]*%sTarget, 0)", corePkg)
	}
	buf.WriteString(code)

	vars := make(map[string]struct{}, 10)
	for i, tgt := range tgs.List() {
		if i == 0 {
			buf.WriteString("\n")
			buf.WriteString("var tgt *")
			buf.WriteString(corePkg)
			buf.WriteString("Target\n")
			buf.WriteString("\n")
		}
		pkgID := tgs.codePkgName(tgt, aliases)
		gen := *tgt
		if pkgID != tgt.PkgName && tgt.PkgName != MainName {
			if gen.VarName != "" {
				// Method receiver var must be unique per import alias.
				gen.VarName = "_v" + pkgID + tgt.Receiver
				gen.CodeRef = gen.VarName + "." + tgt.FuncName
			} else if strings.HasPrefix(gen.CodeRef, tgt.PkgName+".") {
				gen.CodeRef = pkgID + gen.CodeRef[len(tgt.PkgName):]
			}
			if strings.HasPrefix(gen.DefRef, tgt.PkgName+".") {
				gen.DefRef = pkgID + gen.DefRef[len(tgt.PkgName):]
			}
		}
		if gen.VarName != "" {
			// Define variable if it's not defined.
			if _, ok := vars[gen.VarName]; !ok {
				var pkg string
				// When target comes from imported package we
				// need to add the package name qualifier.
				if tgt.PkgName != MainName {
					pkg = pkgID + "."
				}
				format := "var %s %s%s\n"
				code = fmt.Sprintf(format, gen.VarName, pkg, tgt.Receiver)
				buf.WriteString(code)
				vars[gen.VarName] = struct{}{}
			}
		}
		snippet := gen.GoCode(qt)
		code = fmt.Sprintf("tgt = &%s\n", snippet)
		code += "targets = append(targets, tgt)\n\n"
		buf.WriteString(code)
	}

	ret := buf.String()
	if len(tgs.list) > 0 {
		ret = ret[:len(ret)-1]
	}
	return ret
}
