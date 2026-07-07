// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

// Package pkg1 is used to test "gomake:import".
package pkg1

import (
	"context"
	"fmt"

	"github.com/ctx42/ring/pkg/ring"
)

func Pkg1(_ context.Context, rng *ring.Ring) error {
	_, _ = fmt.Fprint(rng.Stdout(), "pkg1.Pkg1")
	return nil
}
