// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package main

import (
	"flag"
	"testing"

	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/must"
	"github.com/ctx42/xflag/pkg/xflag"
)

func Test_emptyTargetsNote_tabular(t *testing.T) {
	tt := []struct {
		testN string

		args []string
		want string
	}{
		{"flag absent", []string{}, ""},
		{"flag empty", []string{"--targets="}, "no external targets provided"},
		{
			"flag whitespace",
			[]string{"--targets=  "},
			"no external targets provided",
		},
		{"flag with path", []string{"--targets=./targets.yaml"}, ""},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- Given ---
			fs := xflag.NewFlagSet("install", flag.ContinueOnError)
			tgs := fs.String("targets", "", "")
			must.Nil(fs.Parse(tc.args))

			// --- When ---
			have := emptyTargetsNote(fs, *tgs)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}
