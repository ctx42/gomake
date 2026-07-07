// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

// Package pkg3 is used to test "gomake:import".
package pkg3

import (
	"context"
	"fmt"

	"github.com/ctx42/ring/pkg/ring"
)

func Pkg3(_ context.Context, rng *ring.Ring) error {
	_, _ = fmt.Fprint(rng.Stdout(), "pkg3.Pkg3")
	return nil
}
