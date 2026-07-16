// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

// Package builtin provides built-in gomake targets and code generation for
// wiring external targets into a compiled binary. Use CLI flags --help,
// --list, and --version for help, listing, and version output.
//
//go:generate go run 00_generate_main.go
package builtin

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/ctx42/ring/pkg/ring"

	"github.com/ctx42/gomake/internal/mkf"
	"github.com/ctx42/gomake/internal/parser"
)

// Filenames.
const (
	// targetsFN represents filename to which code for built-in targets is
	// generated. The code, once generated, becomes part of this (BuiltIn)
	// package.
	targetsFN = "targets.go"

	// mainFN represents filename to which code for built-in targets is
	// generated. The code, once generated, may be used in "main" package.
	mainFN = "targets_main.go_"

	// mainEmptyFN represents filename to which code defining empty built-in
	// targets is generated. The code, once generated, may be used in "main"
	// package.
	mainEmptyFN = "targets_main_empty.go_"
)

// tgsMainEmptySrc is the generated main-package stub used when no external
// targets are compiled in. It provides an empty targetsBuiltIn() with no
// init() registration.
//
//go:embed data/targets_main_empty.go_
var tgsMainEmptySrc []byte

// tgsMainSrc is the generated main-package source that wires compiled-in
// external targets into the binary via an init() call.
//
//go:embed data/targets_main.go_
var tgsMainSrc []byte

// Provider provides built-in targets and their code.
type Provider interface {
	// Targets returns array of built-in targets.
	Targets() []*mkf.Target

	// Source returns Go source code defining the built-in targets.
	Source() []byte

	// PreRuns returns a list of functions to run just before a target.
	PreRuns() []mkf.PreRunFn
}

// targets implements the Provider interface.
type targets struct {
	tgs []*mkf.Target  // Slice of built-in targets.
	src []byte         // Source code defining the built-in targets.
	pre []mkf.PreRunFn // Functions to run before a target.
}

var _ Provider = (*targets)(nil) // Compile time check.

// Empty returns a target provider with no targets.
func Empty() *targets { return newTargets(nil, nil, nil) }

// Generated returns built-in targets from generated targets.go — the external
// targets compiled in via [gomake.TargetsFile].
func Generated() *targets {
	return newTargets(targetsBuiltIn(), tgsMainSrc, nil)
}

// newTargets returns targets instance with given targets and Go source code
// defining them.
func newTargets(tgs []*mkf.Target, src []byte, pre []mkf.PreRunFn) *targets {
	if len(tgs) == 0 {
		src = tgsMainEmptySrc
	}
	return &targets{
		tgs: tgs,
		src: src,
		pre: slices.Clone(pre),
	}
}

func (tgs *targets) Targets() []*mkf.Target {
	return slices.Clone(tgs.tgs)
}

func (tgs *targets) PreRuns() []mkf.PreRunFn {
	return slices.Clone(tgs.pre)
}

func (tgs *targets) Source() []byte {
	return slices.Clone(tgs.src)
}

// GenOption is the signature for [GenMain] options.
type GenOption func(*genOpts)

// genOpts represents built-in generator options.
type genOpts struct {
	// Package name to use for generated files. Default: "builtin".
	name string

	// Build environment. Default [ring.New].
	rng *ring.Ring

	// Absolute path to directory where to generate files, if empty string then
	// current working directory is used.
	dst string

	// Directory whose go.mod resolves the import specs, if empty string then
	// the current working directory is used.
	dir string

	// Generate [mainEmptyFN] file. Default: true.
	empty bool
}

// WithGenName is option for [GenMain] setting package name to use for
// generated code. By default, "builtin" is used.
func WithGenName(name string) GenOption {
	return func(opts *genOpts) { opts.name = name }
}

// WithGenEnv is option for [GenMain] setting build environment to use.
// By default, [ring.New] is used.
func WithGenEnv(rng *ring.Ring) GenOption {
	return func(opts *genOpts) { opts.rng = rng }
}

// WithGenDst is option for [GenMain] setting destination directory where to
// generate files. By default, empty string meaning current working directory.
func WithGenDst(dst string) GenOption {
	return func(opts *genOpts) { opts.dst = dst }
}

// WithGenWorkDir is option for [GenMain] setting the directory whose go.mod
// resolves the import specs. By default, empty string meaning the current
// working directory.
func WithGenWorkDir(dir string) GenOption {
	return func(opts *genOpts) { opts.dir = dir }
}

// WithoutGenEmptySrc is option for [GenMain] turning off generating
// [mainEmptyFN] file.
func WithoutGenEmptySrc(opts *genOpts) { opts.empty = false }

// GenMain is an entry point for the program called by "go generate".
// It takes an import spec list and generates code describing built-in targets,
// registering every target in the root namespace. Use [GenImports] to compile
// in targets under a namespace.
func GenMain(specs []string, opts ...GenOption) error {
	imports := make([]parser.Import, len(specs))
	for i, spec := range specs {
		imports[i] = parser.Import{Path: spec}
	}
	return GenImports(imports, opts...)
}

// GenImports generates code describing built-in targets from the given
// imports, each of which may carry a namespace prefixing the target names its
// package contributes. It overwrites the files if they exist. By default, the
// "builtin" package name is used, the dst path is the current working
// directory, and all three files are generated.
//
// The destination tree (dst and dst/data) is created if missing, so callers
// need not pre-create it.
//
// Depending on options it generates files in paths:
//
//   - dst/[targetsFN]
//   - dst/data/[mainFN]
//   - dst/data/[mainEmptyFN]
func GenImports(imports []parser.Import, opts ...GenOption) error {
	def := genOpts{
		name:  "builtin",
		empty: true,
	}
	for _, opt := range opts {
		opt(&def)
	}
	if def.rng == nil {
		def.rng = ring.New()
	}

	// Create the destination tree; dst/data covers both dst and dst/data.
	if err := os.MkdirAll(filepath.Join(def.dst, "data"), 0o755); err != nil {
		return fmt.Errorf("creating destination tree: %w", err)
	}

	var code []byte

	// Generate code for built-in targets to include in builtin package.
	tgs, err := parser.TargetsFromImports(
		def.rng,
		def.dir,
		imports,
		parser.BuiltInCB,
	)
	if err != nil {
		return fmt.Errorf("parsing target specs: %w", err)
	}
	code, err = parser.NewGenerator(tgs).
		Generate(parser.WithGenNames(def.name, "BuiltIn"))
	if err != nil {
		return fmt.Errorf("generating %s: %w", targetsFN, err)
	}
	err = parser.CreateFile(filepath.Join(def.dst, targetsFN), code)
	if err != nil {
		return fmt.Errorf("writing %s: %w", targetsFN, err)
	}

	// Generate code for main package [mainFN].
	code, err = parser.NewGenerator(tgs).
		Generate(
			parser.WithGenNames(parser.MainName, "BuiltIn"),
			parser.WithGenReg,
		)
	if err != nil {
		return fmt.Errorf("generating %s: %w", mainFN, err)
	}
	err = parser.CreateFile(filepath.Join(def.dst, "data", mainFN), code)
	if err != nil {
		return fmt.Errorf("writing %s: %w", mainFN, err)
	}

	// Skip generating code for empty targets file.
	if !def.empty {
		return nil
	}

	// Generate code for empty targets file.
	gen := parser.NewGenerator(parser.NewTargets())
	code, err = gen.Generate(parser.WithGenNames(parser.MainName, "BuiltIn"))
	if err != nil {
		return fmt.Errorf("generating %s: %w", mainEmptyFN, err)
	}
	err = parser.CreateFile(filepath.Join(def.dst, "data", mainEmptyFN), code)
	if err != nil {
		return fmt.Errorf("writing %s: %w", mainEmptyFN, err)
	}
	return nil
}
