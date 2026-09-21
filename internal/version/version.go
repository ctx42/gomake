// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

// Package version reports the version of the running binary.
package version

import (
	"runtime/debug"

	"github.com/ctx42/xdef/pkg/xdef"
)

// importPath is the package path used in the -X linker definition.
const importPath = "github.com/ctx42/gomake/internal/version"

// devel is the module version the Go toolchain reports for a build it cannot
// version by itself, such as one made from a copy of the module source.
const devel = "(devel)"

// scmRev holds the version injected with "-ldflags -X", empty in a build
// nothing stamped. It is named by [xdef.VarScmRev] so that the ctx42 build
// tooling injects into the same variable [LDFlags] targets.
var scmRev string

// Version returns a human-readable version line for cmd.
func Version(cmd string) string {
	return cmd + " " + revision()
}

// LDFlags returns the "go build" linker flags stamping ver into the binary
// being built. It returns an empty string for a version the Go toolchain
// works out on its own, leaving that value to stand; a caller passes the
// version only for a build made outside the module's repository, where the
// toolchain has nothing to read. No quoting is needed because a module
// version holds no whitespace or quotes.
func LDFlags(ver string) string {
	if ver == "" || ver == devel {
		return ""
	}
	return "-X " + importPath + "." + xdef.VarScmRev + "=" + ver
}

// revision returns the version of the running binary. The Go toolchain
// records one in the build info whenever it can work it out: the module
// version for a published install, and a pseudo-version carrying the last
// tag, the commit and the tree state for a build made in a repository. Only
// a build made from a copy of the module source has none, and there a stamp
// answers.
func revision() string {
	var built string
	if inf, ok := debug.ReadBuildInfo(); ok {
		built = inf.Main.Version
	}
	return pick(scmRev, built)
}

// pick chooses between the version stamped into the binary and the one the
// Go toolchain recorded for it. A stamp wins, being a deliberate statement by
// whoever built the binary. The toolchain reports devel when it had nothing
// to work from, which counts as no version at all.
func pick(stamped, built string) string {
	if stamped != "" {
		return stamped
	}
	if built != "" && built != devel {
		return built
	}
	return devel
}
