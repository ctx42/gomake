// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

// Package builtintest provides test built-in targets.
package builtintest

import (
	"context"
	_ "embed"
	"slices"

	"github.com/ctx42/ring/pkg/ring"

	"github.com/ctx42/gomake/internal/mkf"
)

// ctxKey is the type for context keys set by this package.
type ctxKey string

func (key ctxKey) String() string { return string(key) }

// preRunKey is the context key under which [TestPreRun] stores its counter.
const preRunKey ctxKey = "PRE_RUN"

// Generates test built-in targets.
//go:generate go run 00_generate_main.go

// tgsMainSrc holds the generated targets_main.go_ source embedded
// at build time for use by in-process test target packages.
//
//go:embed data/targets_main.go_
var tgsMainSrc []byte

// TstProvider is a built-in target provider for use in tests.
type TstProvider struct{ pre []mkf.PreRunFn }

// NewTstProvider returns test built-in targets provider.
func NewTstProvider(pre ...mkf.PreRunFn) *TstProvider {
	return &TstProvider{pre: pre}
}

// implements [builtin.Provider].
func (tst *TstProvider) Targets() []*mkf.Target {
	return targetsBuiltIn()
}

// implements [builtin.Provider].
func (tst *TstProvider) Source() []byte {
	return slices.Clone(tgsMainSrc)
}

// implements [builtin.Provider].
func (tst *TstProvider) PreRuns() []mkf.PreRunFn {
	return slices.Clone(tst.pre)
}

// TestTargets returns the test built-in target list.
func TestTargets() []*mkf.Target { return targetsBuiltIn() }

// TestTargetsSrc returns the generated source for the test built-in targets.
func TestTargetsSrc() []byte { return slices.Clone(tgsMainSrc) }

// TestPreRun appends "a" to the PRE_RUN environment variable and adds one to
// the PRE_RUN context value every time it is called.
func TestPreRun(
	ctx context.Context,
	rng *ring.Ring,
) (context.Context, *ring.Ring, error) {

	rng.EnvSet(preRunKey.String(), rng.EnvGet(preRunKey.String())+"a")
	if val, ok := ctx.Value(preRunKey).(int); ok {
		ctx = context.WithValue(ctx, preRunKey, val+1)
	} else {
		ctx = context.WithValue(ctx, preRunKey, 1)
	}
	return ctx, rng, nil
}
