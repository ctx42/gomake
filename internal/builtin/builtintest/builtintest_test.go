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
	t.Run("no hooks", func(t *testing.T) {
		// --- When ---
		have := NewTstProvider()

		// --- Then ---
		assert.Len(t, 0, have.pre)
	})

	t.Run("clones the hooks", func(t *testing.T) {
		// --- Given ---
		pre := []mkf.PreRunFn{TestPreRun}

		// --- When ---
		have := NewTstProvider(pre...)

		// --- Then ---
		assert.Len(t, 1, have.pre)
		assert.Same(t, TestPreRun, have.pre[0])
		assert.NotSame(t, pre, have.pre)
	})
}

func Test_TstProvider_Targets(t *testing.T) {
	// --- When ---
	have := NewTstProvider().Targets()

	// --- Then ---
	assert.Len(t, 2, have)
	assert.Equal(t, ":panic-string", have[0].Name)
	assert.Equal(t, ":print", have[1].Name)
}

func Test_TstProvider_Source(t *testing.T) {
	// --- When ---
	have := NewTstProvider().Source()

	// --- Then ---
	assert.NotSame(t, tgsMainSrc, have)
	assert.Contain(t, "func targetsBuiltIn()", string(have))
}

func Test_TstProvider_PreRuns(t *testing.T) {
	// --- Given ---
	prv := NewTstProvider(TestPreRun)

	// --- When ---
	have := prv.PreRuns()

	// --- Then ---
	assert.Len(t, 1, have)
	assert.Same(t, TestPreRun, have[0])
	assert.NotSame(t, prv.pre, have)
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
	assert.NotSame(t, tgsMainSrc, have)
	assert.Contain(t, "func targetsBuiltIn()", string(have))
}

func Test_TestPreRun(t *testing.T) {
	t.Run("first call", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()

		// --- When ---
		hCtx, hRng, err := TestPreRun(t.Context(), rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.Same(t, rng, hRng)
		assert.Equal(t, "a", hRng.EnvGet(preRunKey.String()))
		assert.Equal(t, 1, hCtx.Value(preRunKey))
	})

	t.Run("second call", func(t *testing.T) {
		// --- Given ---
		ctx, rng, _ := TestPreRun(t.Context(), ring.New())

		// --- When ---
		hCtx, hRng, err := TestPreRun(ctx, rng)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "aa", hRng.EnvGet(preRunKey.String()))
		assert.Equal(t, 2, hCtx.Value(preRunKey))
	})
}
