// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package clitest

import (
	"os"
	"testing"

	"github.com/ctx42/testkit/pkg/selfkit"
)

func TestMain(m *testing.M) {
	runTests, exitCode := selfkit.New().Run(os.Stdout, os.Stderr)
	if runTests {
		os.Exit(m.Run())
	}
	os.Exit(exitCode)
}
