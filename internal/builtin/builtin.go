// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

// Package builtin provides built-in gomake targets and code generation for
// wiring external targets into a compiled binary.
//
//go:generate go run 00_generate_main.go
package builtin

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ctx42/ring/pkg/ring"

	"github.com/ctx42/gomake/internal/mkf"
	"github.com/ctx42/gomake/internal/parser"
)

// Filenames.
const (
	// targetsFN represents the filename to which code for built-in targets is
	// generated. The code, once generated, becomes part of this (BuiltIn)
	// package.
	targetsFN = "targets.go"

	// mainFN represents the filename to which code for built-in targets is
	// generated. The code, once generated, may be used in "main" package.
	mainFN = "targets_main.go_"

	// mainEmptyFN represents the filename to which code defining empty
	// built-in targets is generated. The code, once generated, may be used in
	// "main" package.
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

// Provider provides built-in targets and their code. Each method returns a
// copy the caller may modify.
type Provider interface {
	Targets() []*mkf.Target

	// Source returns the Go source code defining the targets, for the
	// generated makefile's main package.
	Source() []byte

	// PreRuns returns the functions to run just before a target.
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
func Empty() Provider { return newTargets(nil, nil, nil) }

// Generated returns the built-in targets from the generated targets.go: the
// external targets compiled in through the cli.TargetsFile config.
func Generated() Provider {
	return newTargets(targetsBuiltIn(), tgsMainSrc, nil)
}

// newTargets returns a targets instance with the given targets and the Go
// source code defining them.
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

// GenOption is the signature for [GenMain] and [GenImports] options.
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

	// Generate mainEmptyFN file. Default: true.
	empty bool

	// Root the generated ImpPath values are made relative to. Default: empty,
	// keeping absolute paths.
	root string
}

// WithGenName is an option for [GenImports] setting the package name to use
// for generated code. By default, "builtin" is used.
func WithGenName(name string) GenOption {
	return func(opts *genOpts) { opts.name = name }
}

// WithGenEnv is an option for [GenImports] setting the build environment.
// By default, [ring.New] is used.
func WithGenEnv(rng *ring.Ring) GenOption {
	return func(opts *genOpts) { opts.rng = rng }
}

// WithGenDst is an option for [GenImports] setting the destination directory
// for generated files. By default, the empty string means the current working
// directory.
func WithGenDst(dst string) GenOption {
	return func(opts *genOpts) { opts.dst = dst }
}

// WithGenWorkDir is an option for [GenImports] setting the directory whose
// go.mod resolves the import specs. By default, the empty string means the
// current working directory.
func WithGenWorkDir(dir string) GenOption {
	return func(opts *genOpts) { opts.dir = dir }
}

// WithGenImpPathRoot is an option for [GenImports] making each target's ImpPath
// relative to root when it lies under root, so generated files carry no
// machine-specific directory. By default, ImpPath stays absolute.
func WithGenImpPathRoot(root string) GenOption {
	return func(opts *genOpts) { opts.root = root }
}

// WithoutGenEmptySrc is an option for [GenImports] turning off generating the
// mainEmptyFN file.
func WithoutGenEmptySrc(opts *genOpts) { opts.empty = false }

// GenMain generates built-in target code from import specs and registers
// every target in the root namespace. Use [GenImports] to compile in targets
// under a namespace. go generate runs main, which calls GenImports.
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
//   - dst/targetsFN
//   - dst/data/mainFN
//   - dst/data/mainEmptyFN
func GenImports(imports []parser.Import, options ...GenOption) error {
	opts := genOpts{
		name:  "builtin",
		empty: true,
	}
	for _, opt := range options {
		opt(&opts)
	}
	if !validGoIdent(opts.name) {
		return fmt.Errorf("package name %q is not a Go identifier", opts.name)
	}
	if opts.rng == nil {
		opts.rng = ring.New()
	}

	// Generate every file before writing any of them, so a generation
	// failure writes nothing; writeFiles extends that to write failures.
	tgs, err := parser.TargetsFromImports(
		opts.rng,
		opts.dir,
		imports,
		parser.BuiltInCB,
	)
	if err != nil {
		return fmt.Errorf("parsing target specs: %w", err)
	}
	if opts.root != "" {
		tgs.Map(relImpPath(opts.root))
	}
	files := make([]genFile, 0, 3)
	code, err := parser.NewGenerator(tgs).
		Generate(parser.WithGenNames(opts.name, "BuiltIn"))
	if err != nil {
		return fmt.Errorf("generating %s: %w", targetsFN, err)
	}
	dst := filepath.Join(opts.dst, targetsFN)
	files = append(files, genFile{dst: dst, code: code})

	code, err = parser.NewGenerator(tgs).
		Generate(
			parser.WithGenNames(parser.MainName, "BuiltIn"),
			parser.WithGenReg,
		)
	if err != nil {
		return fmt.Errorf("generating %s: %w", mainFN, err)
	}
	dst = filepath.Join(opts.dst, "data", mainFN)
	files = append(files, genFile{dst: dst, code: code})

	if opts.empty {
		gen := parser.NewGenerator(parser.NewTargets())
		code, err = gen.Generate(
			parser.WithGenNames(parser.MainName, "BuiltIn"),
		)
		if err != nil {
			return fmt.Errorf("generating %s: %w", mainEmptyFN, err)
		}
		dst = filepath.Join(opts.dst, "data", mainEmptyFN)
		files = append(files, genFile{dst: dst, code: code})
	}

	// Create the destination tree; dst/data covers both dst and dst/data.
	if err = os.MkdirAll(filepath.Join(opts.dst, "data"), 0o750); err != nil {
		return fmt.Errorf("creating destination tree: %w", err)
	}
	return writeFiles(files)
}

// writeFiles writes every file to a temporary file beside its destination
// before renaming any into place, so failing to write one file leaves every
// destination untouched.
func writeFiles(files []genFile) error {
	tmps := make([]string, len(files))
	defer func() {
		for _, tmp := range tmps {
			if tmp != "" {
				_ = os.Remove(tmp)
			}
		}
	}()
	for i, fil := range files {
		name := filepath.Base(fil.dst) + ".*.tmp"
		tmp, err := os.CreateTemp(filepath.Dir(fil.dst), name)
		if err != nil {
			return fmt.Errorf("writing %s: %w", fil.dst, err)
		}
		tmps[i] = tmp.Name()
		_, err = tmp.Write(fil.code)
		if cerr := tmp.Close(); err == nil {
			err = cerr
		}
		if err == nil {
			err = os.Chmod(tmp.Name(), 0o644)
		}
		if err != nil {
			return fmt.Errorf("writing %s: %w", fil.dst, err)
		}
	}
	for i, fil := range files {
		if err := os.Rename(tmps[i], fil.dst); err != nil {
			return fmt.Errorf("writing %s: %w", fil.dst, err)
		}
		tmps[i] = ""
	}
	return nil
}

// relImpPath returns a [parser.Targets.Map] callback making each target's
// ImpPath relative to root when it lies under root. A relative root is
// resolved against the working directory.
func relImpPath(root string) parser.TgsMapCB {
	if abs, err := filepath.Abs(root); err == nil {
		root = abs
	}
	return func(_ *parser.Targets, tgt *mkf.Target) {
		rel, err := filepath.Rel(root, tgt.ImpPath)
		if err != nil || rel == ".." ||
			strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return
		}
		tgt.ImpPath = filepath.ToSlash(rel)
	}
}

// genFile is one generated file waiting to be written.
type genFile struct {
	dst  string
	code []byte
}
