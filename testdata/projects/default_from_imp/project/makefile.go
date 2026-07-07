// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"fmt"

	"github.com/ctx42/ring/pkg/ring"

	"github.com/ctx42/gomake/testdata/imports/pkg1"
)

// Default case which is not currently supported by gomake. To use imported
// function as default; define target calling it and make it default.
var Default = pkg1.Pkg1

type NS struct{} //gomake:ns_root

func (NS) Hello(_ context.Context, rng *ring.Ring) error {
	_, _ = fmt.Fprint(rng.Stdout(), "NS says hello")
	return nil
}
