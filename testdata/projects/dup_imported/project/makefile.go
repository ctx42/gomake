// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"fmt"

	"github.com/ctx42/ring/pkg/ring"

	_ "github.com/ctx42/gomake/testdata/imports/pkg1" //gomake:import
)

func PKG1(_ context.Context, rng *ring.Ring) error {
	_, _ = fmt.Fprint(rng.Stdout(), "gomake says hello")
	return nil
}
