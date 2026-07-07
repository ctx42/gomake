// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

//go:build gomake

package main

import (
	"context"
	"fmt"

	"github.com/ctx42/ring/pkg/ring"
)

func TargetMain(ctx context.Context, rng *ring.Ring) error {
	_, _ = fmt.Fprint(rng.Stdout(), "ArgTargets: ")
	_ = TargetArch(ctx, rng)
	_, _ = fmt.Fprint(rng.Stdout(), ", ")
	_ = TargetOS(ctx, rng)
	_, _ = fmt.Fprint(rng.Stdout(), ", ")
	_ = TargetOSArch(ctx, rng)
	_, _ = fmt.Fprint(rng.Stdout(), " ")
	_ = TargetHelper(rng)
	return nil
}

func TargetArch(ctx context.Context, rng *ring.Ring) error {
	_, _ = fmt.Fprint(rng.Stdout(), "TargetArch=")
	_ = targetArch(ctx, rng)
	return nil
}

func TargetOS(ctx context.Context, rng *ring.Ring) error {
	_, _ = fmt.Fprint(rng.Stdout(), "TargetOS=")
	return targetOS(ctx, rng)
}

func TargetOSArch(ctx context.Context, rng *ring.Ring) error {
	_, _ = fmt.Fprint(rng.Stdout(), "TargetOSArch=")
	return targetOSArch(ctx, rng)
}

func TargetHelper(rng *ring.Ring) error {
	_, _ = fmt.Fprint(rng.Stdout(), "END")
	return nil
}
