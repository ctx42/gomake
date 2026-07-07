// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

//go:build gomake

package main

import (
	"context"

	"github.com/ctx42/ring/pkg/ring"
)

func Invalid(context.Context, ring.Ring) int { return 0 }
