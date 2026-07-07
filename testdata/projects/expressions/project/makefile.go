// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package main

import (
	"context"

	"github.com/ctx42/ring/pkg/ring"

	"github.com/ctx42/gomake/testdata/imports/pkg0"
	"github.com/ctx42/gomake/testdata/imports/pkg4"
)

var A = Hello
var B = pkg0.Pkg0
var C = pkg4.NS.M0

func Hello(context.Context, ring.Ring) error { return nil }

type NS struct{} //gomake:ns_root

func (NS) Hello(context.Context, ring.Ring) error { return nil }
