// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package parser

import (
	"testing"

	"github.com/ctx42/ring/pkg/ring"
	"github.com/ctx42/testkit/pkg/modkit"
)

// Benchmark_NewPackage_local isolates the cost of the `go list` subprocess for
// a local package directory. Compare it against Benchmark_AstAndDocPkg_local to
// see how the subprocess dominates the parser path.
func Benchmark_NewPackage_local(b *testing.B) {
	rng := SetBuildTag(ring.New())
	dir := modkit.Path("testdata/projects/showcase_targets/project")

	b.ReportAllocs()
	for b.Loop() {
		if _, err := NewPackage(rng, dir); err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark_AstAndDocPkg_local isolates the in-process AST + doc parsing cost
// (no subprocess) for the largest realistic local makefile fixture.
func Benchmark_AstAndDocPkg_local(b *testing.B) {
	dir := modkit.Path("testdata/projects/showcase_targets/project")

	b.ReportAllocs()
	for b.Loop() {
		if _, _, err := astAndDocPkg(dir, nil); err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark_NewMakefile_big measures the full local discovery path (`go list`
// plus parse plus target extraction) for the largest realistic fixture.
func Benchmark_NewMakefile_big(b *testing.B) {
	rng := SetBuildTag(ring.New())
	dir := modkit.Path("testdata/projects/showcase_targets/project")

	b.ReportAllocs()
	for b.Loop() {
		if _, err := NewMakefile(rng, dir); err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark_NewMakefile_small measures the full local discovery path for a
// minimal tagged makefile fixture (the common project shape).
func Benchmark_NewMakefile_small(b *testing.B) {
	rng := SetBuildTag(ring.New())
	dir := modkit.Path("testdata/projects/simple_tagged/project")

	b.ReportAllocs()
	for b.Loop() {
		if _, err := NewMakefile(rng, dir); err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark_TargetsFromSpecs measures the import-spec discovery path, which
// resolves each spec with its own `go list` subprocess before parsing.
func Benchmark_TargetsFromSpecs(b *testing.B) {
	rng := ring.New()
	specs := []string{
		"github.com/ctx42/gomake/testdata/imports/pkg0",
		"github.com/ctx42/gomake/testdata/imports/pkg1",
		"github.com/ctx42/gomake/testdata/imports/pkg7",
	}

	b.ReportAllocs()
	for b.Loop() {
		if _, err := TargetsFromSpecs(rng, specs); err != nil {
			b.Fatal(err)
		}
	}
}
