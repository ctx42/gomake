// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

//go:build ignore

package main

import (
	"fmt"
	"os"

	"github.com/ctx42/gomake/internal/parser"
)

// main is intended to be called by "go generate" triggered in parser.go file.
func main() {
	dst := "data/makefile_user_empty.go_"
	if err := parser.GenMain(dst); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
