// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

// Package pkg5 is used to test "gomake:import" comment which in turn imports
// one more package.
package pkg5

import (
	"context"
	"fmt"

	"github.com/ctx42/ring/pkg/ring"

	_ "github.com/ctx42/gomake/testdata/imports/pkg3" //gomake:import
)

func Pkg5(_ context.Context, rng *ring.Ring) error {
	_, _ = fmt.Fprint(rng.Stdout(), "pkg5.Pkg5")
	return nil
}
