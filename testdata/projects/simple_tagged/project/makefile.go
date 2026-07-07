// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

//go:build gomake

package main

import (
	"context"
	"fmt"

	"github.com/ctx42/ring/pkg/ring"
)

func Hello(_ context.Context, rng *ring.Ring) error {
	_, _ = fmt.Fprintf(rng.Stdout(), "hello %v", rng.Args())
	return nil
}

func Bye(_ context.Context, rng *ring.Ring) error {
	_, _ = fmt.Fprint(rng.Stdout(), "bye", fmt.Sprint(args))
	return nil
}
