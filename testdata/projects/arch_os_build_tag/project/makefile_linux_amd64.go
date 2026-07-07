// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

//go:build gomake

package main

import (
	"context"
	"fmt"

	"github.com/ctx42/ring/pkg/ring"
)

func targetOSArch(_ context.Context, rng *ring.Ring) error {
	_, _ = fmt.Fprint(rng.Stdout(), "os-arch:linux-amd64")
	return nil
}
