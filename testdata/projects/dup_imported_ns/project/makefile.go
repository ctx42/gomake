// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"fmt"

	"github.com/ctx42/ring/pkg/ring"

	_ "github.com/ctx42/gomake/testdata/imports/pkg4" //gomake:import
)

type NS struct{} //gomake:ns_root

func (NS) M0(_ context.Context, rng *ring.Ring) error {
	_, _ = fmt.Fprint(rng.Stdout(), "NS.M0")
	return nil
}
