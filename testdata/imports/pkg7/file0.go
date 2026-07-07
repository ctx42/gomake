// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

// Package pkg7 is used to test "gomake:import".
package pkg7

import (
	"context"
	"fmt"

	"github.com/ctx42/ring/pkg/ring"
)

func Print(_ context.Context, rng *ring.Ring) error {
	_, _ = fmt.Fprint(rng.Stdout(), "pkg7.Pkg")
	return nil
}
