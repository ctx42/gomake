// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"fmt"

	"github.com/ctx42/ring/pkg/ring"
)

func EnvKey(_ context.Context, rng *ring.Ring) error {
	args := rng.Args()
	if len(args) == 0 {
		for _, v := range rng.EnvAll() {
			_, _ = fmt.Fprintln(rng.Stdout(), v)
		}
		return nil
	}
	key := args[0]
	val, exist := rng.EnvLookup(key)
	_, _ = fmt.Fprintf(rng.Stdout(), "%s (%t): %#q", key, exist, val)
	return nil
}
