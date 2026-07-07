// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

//go:build gomake

package main

import (
	"context"

	"github.com/ctx42/ring/pkg/ring"
)

func targetArch(_ context.Context, rng *ring.Ring) error {
	_, _ = fmt.Fprint(rng.Stdout(), "arch:arm64")
	return nil
}
