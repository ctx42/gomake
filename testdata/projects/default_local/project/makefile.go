// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"fmt"

	"github.com/ctx42/ring/pkg/ring"
)

var Default = Hello
var Aaa = Bye // Variable not named Default but with target reference.

// Hello prints a short message to stdout.
func Hello(_ context.Context, rng *ring.Ring) error {
	_, _ = fmt.Fprint(rng.Stdout(), "gomake says hello")
	return nil
}

// Bye prints a short message to stderr.
func Bye(_ context.Context, rng *ring.Ring) error {
	_, _ = fmt.Fprint(rng.Stderr(), "gomake says bye")
	return nil
}
