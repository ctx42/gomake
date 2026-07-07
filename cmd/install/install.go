// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

// Command `install` builds and installs the gomake binary into GOBIN. Set
// GOBIN (or GOPATH) in the environment to control where the binary lands.
//
// When run from a published version it fetches the module source automatically.
// When run from the local source (go run ./cmd/install) it builds from the
// current working directory.
//
// Typical usage:
//
//	go run github.com/ctx42/gomake/cmd/install@latest
//	go run ./cmd/install
//	go run ./cmd/install --targets=path/to/targets.yaml
//	go run ./cmd/install --targets=http://example.com/targets.yaml
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime/debug"

	"github.com/ctx42/ring/pkg/ring"

	"github.com/ctx42/gomake/internal/install"
)

func main() {
	targets := flag.String("targets", "", "path or URL to a targets.yaml file")
	flag.Parse()

	rng := ring.New()
	info, _ := debug.ReadBuildInfo()
	if err := install.Main(rng, info, *targets); err != nil {
		_, _ = fmt.Fprintln(rng.Stderr(), err)
		os.Exit(1)
	}
}
