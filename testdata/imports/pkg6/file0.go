// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

// Package pkg6 is used to test "gomake:import".
package pkg6

import (
	"context"
	"fmt"

	"github.com/ctx42/ring/pkg/ring"
)

func Print(_ context.Context, rng *ring.Ring) error {
	_, _ = fmt.Fprint(rng.Stdout(), "pkg6.Pkg")
	return nil
}
