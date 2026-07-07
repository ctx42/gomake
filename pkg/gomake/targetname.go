// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gomake

import "context"

// targetNameKey is the context key under which the running target's name is
// stored.
//
// It is deliberately a plain string value rather than a defined key type. The
// value is set by the makefile runtime
// ([github.com/ctx42/gomake/internal/mkf]), whose source is inlined into a
// generated makefile as the "main" package; an inlined defined type cannot
// share identity with this package, so a string value matched by equality is
// the only key that works across that boundary. Keep this value in sync with
// the runtime's key in internal/mkf/makefile.go.
const targetNameKey = "github.com/ctx42/gomake/pkg/gomake.targetName"

// WithTargetName returns a copy of ctx carrying the name of the target about
// to run. The makefile runtime sets it before invoking a target; user targets
// read it back with [TargetName].
func WithTargetName(ctx context.Context, name string) context.Context {
	//nolint:staticcheck // SA1029: a string key is required so the value
	// matches the one set by the inlined runtime (see targetNameKey).
	return context.WithValue(ctx, targetNameKey, name)
}

// TargetName returns the name of the target being executed. The makefile
// runtime sets it via [WithTargetName] before calling the target.
func TargetName(ctx context.Context) (string, bool) {
	name, ok := ctx.Value(targetNameKey).(string)
	return name, ok
}
