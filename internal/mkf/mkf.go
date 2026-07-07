// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

// Package mkf provides the makefile target model and execution runtime. Its
// source (after the CODE MARK in makefile.go, target.go and helpers.go) is
// inlined into generated makefile binaries, so the code there must stay
// self-contained except for the public [github.com/ctx42/gomake/pkg/gomake]
// and [github.com/ctx42/xflag/pkg/xflag] packages, whose imports the generator
// guarantees (xflag is injected into the build "go.mod" during compilation).
package mkf

import _ "embed"

// The gomake file names.
const (
	// MakefileMain represents the name of the main makefile. This is the file
	// gomake looks for to get user-defined targets.
	MakefileMain = "makefile.go"

	// MakefileGen represents the name of the generated makefile. Once the
	// gomake parses the MakefileMain and extracts targets, it creates this
	// file as a "glue code".
	MakefileGen = "makefile_gen.go"

	// MakefileUser represents the name of the generated makefile with
	// user-defined target descriptions.
	MakefileUser = "makefile_user.go"
)

// MakefileSrc is the source for [Makefile] related code.
//
//go:embed makefile.go
var MakefileSrc string

// TargetSrc is the source for [Target] type.
//
//go:embed target.go
var TargetSrc string

// HelpersSrc is the source for helper functions.
//
//go:embed helpers.go
var HelpersSrc string
