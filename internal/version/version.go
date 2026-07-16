// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

// Package version holds build metadata set via ldflags during install.
package version

import (
	"runtime/debug"
	"strings"
	"time"

	"github.com/ctx42/ring/pkg/ring"
	"github.com/ctx42/xdef/pkg/xdef"

	"github.com/ctx42/gomake/pkg/gomake"
)

// importPath is the package path used in -X linker definitions.
const importPath = "github.com/ctx42/gomake/internal/version"

// Variables set by ldflags representing the build version.
var (
	// buildDate holds the UTC RFC-3339 timestamp of the build.
	buildDate = xdef.NotSet

	// scmRev holds the source control revision tag, e.g. "v1.2.3".
	scmRev = xdef.NotSet

	// scmHash holds the short commit hash, e.g. "12ab23c".
	scmHash = xdef.NotSet

	// scmState represents the working directory state: clean, dirty.
	scmState = xdef.NotSet

	// ccid represents CI/CD job identifier.
	ccid = xdef.NotSet
)

// Get returns the current build-metadata values: date, rev, hash, state, cc.
func Get() (string, string, string, string, string) {
	return buildDate, scmRev, scmHash, scmState, ccid
}

// Set assigns all five build-metadata variables at once.
func Set(date, rev, hash, state, cc string) {
	buildDate = date
	scmRev = rev
	scmHash = hash
	scmState = state
	ccid = cc
}

// PopulateVersion reads version metadata embedded by the Go toolchain in info
// and stores the results in the package variables. The module version and the
// vcs.revision/vcs.modified build settings are used when present and
// meaningful; any field info does not carry is recorded as [xdef.NotSet]. It
// relies only on the Go toolchain and never shells out to a VCS such as git,
// so installation works on machines that have only Go installed. The CI/CD job
// identifier is read from the [gomake.CCIDEnvKey] environment variable.
func PopulateVersion(rng *ring.Ring, info *debug.BuildInfo) {
	rev, hash, state := buildInfoFields(info)
	date := rng.Clock()().UTC().Format(time.RFC3339)
	Set(
		date,
		orNotSet(rev),
		orNotSet(hash),
		orNotSet(state),
		orNotSet(rng.EnvGet(gomake.CCIDEnvKey)),
	)
}

// orNotSet returns s when it is non-empty, otherwise [xdef.NotSet].
func orNotSet(s string) string {
	if s == "" {
		return xdef.NotSet
	}
	return s
}

// Version returns a human-readable version line.
func Version(cmd string) string {
	var b strings.Builder
	b.WriteString(cmd)
	b.WriteByte(' ')
	b.WriteString(scmRev)
	writeField(&b, "hash", scmHash)
	writeField(&b, "build date", buildDate)
	writeField(&b, "scm state", scmState)
	writeField(&b, "cc tag", ccid)
	return b.String()
}

// LDFlags returns linker -X flags for the current package variables.
//
// During installation, populate the variables first, then pass the result to
// `go build -ldflags`.
func LDFlags() string {
	flags := []string{
		ldflag(xdef.VarBuildDate, buildDate),
		ldflag(xdef.VarScmRev, scmRev),
		ldflag(xdef.VarScmHash, scmHash),
		ldflag(xdef.VarScmState, scmState),
		ldflag(xdef.VarCCID, ccid),
	}
	return strings.Join(flags, " ")
}

// buildInfoFields extracts the SCM revision, short commit hash, and working
// tree state from info. It returns an empty string for any field info does not
// carry. The module version "(devel)", reported for local builds, is treated
// as absent.
func buildInfoFields(info *debug.BuildInfo) (rev, hash, state string) {
	if info == nil {
		return "", "", ""
	}
	if v := info.Main.Version; v != "" && v != "(devel)" {
		rev = v
	}
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			if len(s.Value) >= 7 {
				hash = s.Value[:7]
			} else if s.Value != "" {
				hash = s.Value
			}
		case "vcs.modified":
			switch s.Value {
			case "true":
				state = "dirty"
			case "false":
				state = "clean"
			}
		}
	}
	return rev, hash, state
}

// writeField appends ", label: val" to b.
func writeField(b *strings.Builder, label, val string) {
	b.WriteString(", ")
	b.WriteString(label)
	b.WriteString(": ")
	b.WriteString(val)
}

// ldflag returns one -X definition for importPath.field=value. Values with
// spaces or quotes are quoted so `go build` receives a single linker argument.
// Values that contain both single and double quotes cannot be represented with
// cmd/internal/quoted.Split without escaping, so those characters are replaced
// with spaces before quoting.
func ldflag(field, value string) string {
	if strings.Contains(value, `"`) && strings.Contains(value, `'`) {
		value = strings.Map(func(r rune) rune {
			if r == '"' || r == '\'' {
				return ' '
			}
			return r
		}, value)
	}
	def := importPath + "." + field + "=" + value
	if !needsLDQuote(def) {
		return "-X " + def
	}
	// go build splits -ldflags with cmd/internal/quoted.Split, which treats
	// quoted content literally and performs no backslash unescaping. A value
	// with a double quote is wrapped in single quotes; otherwise double quotes
	// suffice and no escaping is applied.
	if strings.Contains(value, `"`) {
		return `-X '` + def + `'`
	}
	return `-X "` + def + `"`
}

// needsLDQuote reports whether the ldflag definition string contains
// characters that require shell quoting.
func needsLDQuote(def string) bool {
	return strings.ContainsAny(def, " \t\"'\\")
}
