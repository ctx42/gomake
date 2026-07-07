// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"fmt"

	"github.com/ctx42/ring/pkg/ring"

	"github.com/ctx42/gomake/pkg/gomake"
)

// Show prints the message from its gomake.yaml configuration block.
func Show(_ context.Context, rng *ring.Ring) error {
	var cfg struct {
		Message string `json:"message"`
	}
	if err := gomake.TargetConfig(rng, &cfg); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(rng.Stdout(), "message=%s", cfg.Message)
	return nil
}
