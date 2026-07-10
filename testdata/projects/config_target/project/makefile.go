// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/ctx42/ring/pkg/ring"

	"github.com/ctx42/gomake/pkg/gomake"
)

// Show prints the message from its gomake.yaml configuration block.
func Show(_ context.Context, rng *ring.Ring) error {
	cfg, err := gomake.TargetConfig(rng)
	if err != nil {
		return err
	}
	msg, err := gomake.GetCfg[string](cfg, "message")
	if err != nil && !errors.Is(err, gomake.ErrMiss) {
		return err
	}
	_, _ = fmt.Fprintf(rng.Stdout(), "message=%s", msg)
	return nil
}
