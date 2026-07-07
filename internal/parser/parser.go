// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

// Package parser is responsible for parsing go code to extract gomake targets.
package parser

import (
	_ "embed"
	"errors"
)

// Generates built-in targets.
//go:generate go run main.go

// TgsUserEmptySrc embeds code for empty user targets file.
//
//go:embed data/makefile_user_empty.go_
var TgsUserEmptySrc []byte

// BuildTag is Go build tag that might be used by files defining targets.
const BuildTag = "gomake"

// BuildTagLine represents how the build tag line should look like in the file.
const BuildTagLine = "//go:build " + BuildTag + "\n"

// nsTag represents a comment to tag a type to be namespace root.
//
//	type NS0 struct{} //gomake:ns_root
const nsTag = "gomake:ns_root"

// importTag represents a comment used to tag package imports from which
// you wish to import targets.
//
// Examples:
//
//	import github.com/user/targets/git //gomake:import
//	import github.com/user/targets/git //gomake:import ns
const importTag = "gomake:import"

// hiddenTag represents a comment used to tag a target as hidden.
//
// Example usage:
//
//	// Not yet ready for prime time.
//	//
//	// gomake:hidden optional explanation why target is hidden
//	func Target(ctx context.Context, rng *ring.Ring) error { ... }
//
// Notice there must be a space between "//" and "gomake:".
const hiddenTag = "gomake:hidden"

// metaBuildTag represents [ring.Ring] metadata key for build tag.
const metaBuildTag = "~~gomake-build-tag~~"

// Go source parsing errors.
var (
	// errAstMultiPkg is returned when a path contains more than one Go package.
	errAstMultiPkg = errors.New("multiple AST packages")

	// errAstParse is returned when parsing Go source fails.
	errAstParse = errors.New("error parsing sources")

	// ErrAstEmpty is returned when no Go packages were found at a path.
	ErrAstEmpty = errors.New("empty AST for package")

	// ErrGoList represents an error executing `go list` command.
	ErrGoList = errors.New("error executing go list")

	// ErrAbsPath is returned when package path is not absolute.
	ErrAbsPath = errors.New("path must be absolute")
)

// GenMain represents an entry point for the program called by "go generate".
// It generates [mkf.MakefileUser] placeholder file. Placeholder file defines
// targetsUser function returning empty slice of targets.
func GenMain(dst string) error {
	gen := NewGenerator(NewTargets())
	code, err := gen.Generate(WithGenNames(MainName, "User"))
	if err != nil {
		return err
	}
	if err = CreateFile(dst, code); err != nil {
		return err
	}
	return nil
}
