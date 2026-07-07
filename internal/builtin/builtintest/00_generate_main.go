// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

//go:build ignore

package main

import (
	"fmt"
	"os"

	"github.com/ctx42/gomake/internal/builtin"
)

// main is intended to be called by "go generate" triggered in parser.go file.
func main() {
	impSpecs := []string{"github.com/ctx42/gomake/testdata/imports/pkg2"}
	err := builtin.GenMain(
		impSpecs,
		builtin.WithGenName("builtintest"),
		builtin.WithoutGenEmptySrc,
	)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
