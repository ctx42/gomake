// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

// Package osarch provides the set of GOOS and GOARCH values supported by Go,
// used to validate makefile filename suffixes (makefile_<GOOS>.go,
// makefile_<GOARCH>.go, makefile_<GOOS>_<GOARCH>.go).
//
// GOOS and GOARCH values are only added in newer Go releases, so a single list
// generated from the latest Go is a safe superset for any project. The data is
// generated from `go tool dist list` (see 00_generate_versions.go) into
// versions_gen.go. Regenerate it with `go generate ./internal/osarch`; set
// GOMAKE_GO_VERSION to pin the Go version used to populate the list.
package osarch

import "slices"

//go:generate go run 00_generate_versions.go

// IsGOOS reports whether s is a GOOS value supported by Go.
func IsGOOS(s string) bool { return slices.Contains(goos, s) }

// IsGOARCH reports whether s is a GOARCH value supported by Go.
func IsGOARCH(s string) bool { return slices.Contains(goarch, s) }
