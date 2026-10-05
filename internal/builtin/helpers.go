// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package builtin

import "go/token"

// validGoIdent reports whether name can be a Go package name.
func validGoIdent(name string) bool {
	if name == "_" || !token.IsIdentifier(name) {
		return false
	}
	return !token.Lookup(name).IsKeyword()
}
