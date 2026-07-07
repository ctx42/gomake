// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

// Package pkg8 is used to test "gomake:import".
package pkg8

import (
	"context"
	"fmt"

	"github.com/ctx42/ring/pkg/ring"
)

func Pkg8F0(_ context.Context, rng *ring.Ring) error {
	_, _ = fmt.Fprint(rng.Stdout(), "pkg8.Pkg8F0")
	return nil
}
