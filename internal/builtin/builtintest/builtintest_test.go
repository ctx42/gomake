// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package builtintest

import (
	"testing"

	"github.com/ctx42/ring/pkg/ring"
	"github.com/ctx42/testing/pkg/assert"

	"github.com/ctx42/gomake/internal/builtin"
	"github.com/ctx42/gomake/internal/mkf"
)

// Compile-time check placed in the test package: TstProvider's production file
// cannot import builtin without a test-build import cycle, because builtin's
// own in-package test imports builtintest.
var _ builtin.Provider = (*TstProvider)(nil)

func Test_ctxKey_String(t *testing.T) {
	// --- When ---
	have := preRunKey.String()

	// --- Then ---
	assert.Equal(t, "PRE_RUN", have)
}

func Test_NewTstProvider(t *testing.T) {
	// --- Given ---
	pre := []mkf.PreRunFn{TestPreRun}

	// --- When ---
	have := NewTstProvider(pre...)

	// --- Then ---
	assert.Len(t, 1, have.PreRuns())
	assert.Same(t, pre[0], have.PreRuns()[0])
	// PreRuns returns a clone so a caller cannot replace a hook.
	pr := have.PreRuns()
	pr[0] = nil
	assert.Same(t, TestPreRun, have.PreRuns()[0])
}

func Test_TestTargets(t *testing.T) {
	// --- When ---
	have := TestTargets()

	// --- Then ---
	assert.Len(t, 2, have)
	assert.Equal(t, ":panic-string", have[0].Name)
	assert.Equal(t, ":print", have[1].Name)
}

func Test_TestTargetsSrc(t *testing.T) {
	// --- When ---
	have := TestTargetsSrc()

	// --- Then ---
	assert.Equal(t, string(tgsMainSrc), string(have))
}

func Test_TestPreRun(t *testing.T) {
	t.Run("first call", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()

		rng := ring.New()

		// --- When ---
		hCtx, hRng, err := TestPreRun(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.Same(t, rng, hRng)
		assert.Equal(t, "a", hRng.EnvGet(preRunKey.String()))
		assert.Equal(t, 1, hCtx.Value(preRunKey))
	})

	t.Run("second call increments counter and appends env", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()

		rng := ring.New()

		// --- When ---
		ctx, rng, _ = TestPreRun(ctx, rng)
		hCtx, hRng, err := TestPreRun(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "aa", hRng.EnvGet(preRunKey.String()))
		assert.Equal(t, 2, hCtx.Value(preRunKey))
	})
}
