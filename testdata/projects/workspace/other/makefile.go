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
	_, _ = fmt.Fprintf(rng.Stdout(), "other %v", rng.Args())
	return nil
}
