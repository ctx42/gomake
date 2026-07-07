// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

// Package pkg is used to test "gomake:import".
package pkg

import (
	"context"
	"fmt"

	"github.com/ctx42/ring/pkg/ring"
)

func Pkg(_ context.Context, rng *ring.Ring) error {
	_, _ = fmt.Fprint(rng.Stdout(), "xx-pkg.Pkg")
	return nil
}
