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
	have := NewTarget()

	// --- Then ---
	assert.Equal(t, "", have.ImpSpec)
	assert.Equal(t, "", have.ImpPath)
	assert.Equal(t, "", have.PkgName)
	assert.Equal(t, "", have.PkgNS)
	assert.Nil(t, have.Breadcrumbs)
	assert.Equal(t, "", have.Receiver)
	assert.Equal(t, "", have.FuncName)
	assert.Equal(t, "", have.Name)
	assert.Equal(t, "", have.VarName)
	assert.Equal(t, "", have.CodeRef)
	assert.Equal(t, "", have.DefRef)
	assert.False(t, have.Default)
	assert.Equal(t, "", have.Synopsis)
	assert.Equal(t, "", have.Doc)
	assert.False(t, have.Hidden)
	assert.NotNil(t, have.Run)
	assert.NoError(t, have.Run(nil, &ring.Ring{}))
	assert.Fields(t, 16, have)
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

		n := 0

		// --- When ---
		have := tgt.genRunCode(n)

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

		n := 3

		// --- When ---
		have := tgt.genRunCode(n)

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
		{"leading colon", ":abc", true},
		{"plain name", "abc", false},
		{"empty name", "", false},
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
