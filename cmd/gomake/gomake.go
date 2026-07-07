// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

// Command gomake is the gomake binary entry point. It loads built-in targets
// (including any external targets compiled in via targets.yaml), resolves the
// requested target, and runs it.
package main

import (
	"context"
	"os"

	"github.com/ctx42/ring/pkg/ring"

	"github.com/ctx42/gomake/internal/builtin"
	"github.com/ctx42/gomake/internal/cli"
	"github.com/ctx42/gomake/internal/version"
)

func main() {
	ctx := context.Background()
	rng := ring.New()
	ver := version.Version("gomake")
	bip := builtin.Generated()
	code := cli.Main(ctx, rng, ver, bip)
	os.Exit(code)
}
