// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

// Package pkg0 is used to test "gomake:import".
package pkg0

import (
	"context"
	"fmt"

	"github.com/ctx42/ring/pkg/ring"
)

// Pkg0 is an example target. It prints its name along with the package name
// it belongs to. This sentence is only to have the documentation bit longer.
func Pkg0(_ context.Context, rng *ring.Ring) error {
	_, _ = fmt.Fprint(rng.Stdout(), "pkg0.Pkg0")
	return nil
}
