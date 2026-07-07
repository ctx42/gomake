// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

// Package main is a simple makefile example.
package main

import (
	"context"
	"fmt"

	"github.com/ctx42/ring/pkg/ring"

	_ "github.com/ctx42/gomake/testdata/imports/pkg0" //gomake:import
	_ "github.com/ctx42/gomake/testdata/imports/pkg1" //gomake:import ns
	_ "github.com/ctx42/gomake/testdata/imports/pkg2" //gomake:import MX
	_ "github.com/ctx42/gomake/testdata/imports/pkg9" //gomake:import abc

	// Invalid imports.

	_ "github.com/ctx42/gomake/testdata/imports/pkg3" //
	_ "github.com/ctx42/gomake/testdata/imports/pkg4" // gomake
	_ "github.com/ctx42/gomake/testdata/imports/pkg5" // gomake:
	_ "github.com/ctx42/gomake/testdata/imports/pkg6" // gomake:import ns mx
	_ "github.com/ctx42/gomake/testdata/imports/pkg7" // notgomake:import

	// Regular imports.

	"github.com/ctx42/gomake/testdata/imports/pkg8"
)

func Local(ctx context.Context, rng *ring.Ring) error {
	_, _ = fmt.Fprint(rng.Stdout(), "local target says hello")
	return nil
}

func Imported(ctx context.Context, rng *ring.Ring) error {
	return pkg8.Pkg8F1(ctx, rng)
}

func Invalid(context.Context, ring.Ring) int { return 0 }
