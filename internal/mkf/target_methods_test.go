// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package mkf

import (
	"strings"
	"testing"

	"github.com/ctx42/ring/pkg/ring"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/goldy"
	"github.com/ctx42/testkit/pkg/pathkit"
)

func Test_NewTarget(t *testing.T) {
	// --- When ---
	tgt := NewTarget()

	// --- Then ---
	assert.Equal(t, "", tgt.ImpSpec)
	assert.Equal(t, "", tgt.ImpPath)
	assert.Equal(t, "", tgt.PkgName)
	assert.Equal(t, "", tgt.PkgNS)
	assert.Nil(t, tgt.Breadcrumbs)
	assert.Equal(t, "", tgt.Receiver)
	assert.Equal(t, "", tgt.FuncName)
	assert.Equal(t, "", tgt.Name)
	assert.Equal(t, "", tgt.VarName)
	assert.Equal(t, "", tgt.CodeRef)
	assert.Equal(t, "", tgt.DefRef)
	assert.False(t, tgt.Default)
	assert.Equal(t, "", tgt.Synopsis)
	assert.Equal(t, "", tgt.Doc)
	assert.False(t, tgt.Hidden)
	assert.NotNil(t, tgt.Run)
	assert.NoError(t, tgt.Run(nil, &ring.Ring{}))
	assert.Fields(t, 16, tgt)
}

func Test_Target_GoCode(t *testing.T) {
	t.Run("all fields set", func(t *testing.T) {
		// --- Given ---
		gfp := "testdata/target_all_fields.gld"
		gld := goldy.Open(t, pathkit.AbsPath(t, gfp))
		tgt := &Target{
			ImpSpec:     "ImpSpec",
			ImpPath:     "ImpPath",
			PkgName:     "PkgName",
			PkgNS:       "Namespace",
			Breadcrumbs: []string{"a", "b", "c"},
			Receiver:    "Receiver",
			FuncName:    "FuncName",
			Name:        "Name",
			VarName:     "VarName",
			CodeRef:     "CodeRef",
			DefRef:      "DefRef",
			Default:     true,
			Synopsis:    "Synopsis",
			Doc:         "Doc",
			Hidden:      true,
		}

		// --- When ---
		have := tgt.GoCode(false)

		// --- Then ---
		assert.Equal(t, gld.String(), have)
	})

	t.Run("with mkf qualified identifier", func(t *testing.T) {
		// --- Given ---
		tgt := &Target{}

		// --- When ---
		have := tgt.GoCode(true)

		// --- Then ---
		assert.True(t, strings.HasPrefix(have, "mkf.Target{\n"))
	})

	t.Run("target without args", func(t *testing.T) {
		// --- Given ---
		gfp := "testdata/target_without_args.gld"
		gld := goldy.Open(t, pathkit.AbsPath(t, gfp))
		tgt := &Target{
			Name:    "Name",
			CodeRef: "CodeRef",
		}

		// --- When ---
		have := tgt.GoCode(false)

		// --- Then ---
		assert.Equal(t, gld.String(), have)
	})

	t.Run("Doc and Synopsis with quotes", func(t *testing.T) {
		// --- Given ---
		gfp := "testdata/target_with_quotes_in_docs.gld"
		gld := goldy.Open(t, pathkit.AbsPath(t, gfp))
		tgt := &Target{
			Name:     "Name",
			CodeRef:  "CodeRef",
			Synopsis: "Synopsis for \"a.go\"",
			Doc:      "Doc for \"a.go\"",
		}

		// --- When ---
		have := tgt.GoCode(false)

		// --- Then ---
		assert.Equal(t, gld.String(), have)
	})
}

func Test_Target_genRunCode(t *testing.T) {
	t.Run("with args", func(t *testing.T) {
		// --- Given ---
		tgt := &Target{
			Name:    "hello",
			CodeRef: "pkt.Hello",
		}

		// --- When ---
		have := tgt.genRunCode(0)

		// --- Then ---
		want := "return pkt.Hello(ctx, rng)"
		assert.Equal(t, want, have)
	})

	t.Run("indented", func(t *testing.T) {
		// --- Given ---
		tgt := &Target{
			Name:    "hello",
			CodeRef: "pkt.Hello",
		}

		// --- When ---
		have := tgt.genRunCode(3)

		// --- Then ---
		want := "\t\t\treturn pkt.Hello(ctx, rng)"
		assert.Equal(t, want, have)
	})
}

func Test_Target_IsCore_tabular(t *testing.T) {
	tt := []struct {
		testN string

		name string
		want bool
	}{
		{"1", ":abc", true},
		{"2", "abc", false},
		{"3", "", false},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- Given ---
			tgt := &Target{Name: tc.name}

			// --- When ---
			have := tgt.IsCore()

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}
