// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

// Package gomake provides the toolkit available to gomake target authors:
// reading the running target's name ([TargetName]), locating the module root
// ([Root]), reading per-target configuration ([TargetConfig], [GetCfg]), and
// small filesystem and environment helpers. It is the only gomake package a
// makefile.go or a built-in target package needs to import.
package gomake

import "errors"

// ErrNoGoMod is returned when [Root] cannot find the project root directory.
var ErrNoGoMod = errors.New("cannot find \"go.mod\" file")

// Configuration lookup errors.
var (
	// ErrMiss indicates a path that does not resolve to a value: an absent
	// key or an out-of-range or non-numeric array index.
	ErrMiss = errors.New("config path not found")

	// ErrType indicates a value that cannot be represented as the requested
	// type, including descending through a scalar leaf.
	ErrType = errors.New("config type mismatch")

	// ErrConfig indicates a configuration block that is not a single valid
	// JSON object.
	ErrConfig = errors.New("invalid target config")

	// ErrPath indicates a malformed path: an empty path, an unterminated
	// quote, or a quote not at a segment boundary.
	ErrPath = errors.New("config path malformed")
)

// ConfigMetaKey is the ring meta-store key under which gomake places the
// running target's configuration block as a JSON string. Read the block with
// [TargetConfig] rather than accessing the key directly.
const ConfigMetaKey = "github.com/ctx42/gomake/pkg/gomake.targetConfig"

// Public gomake environment contract keys.
const (
	// VersionEnvKey is the environment variable carrying the gomake version
	// string. The runtime sets it on the process environment (and ring meta)
	// so a target can read the version of the gomake tool that invoked it.
	VersionEnvKey = "GOMAKE_VERSION"

	// ProjectDirEnvKey is the environment variable carrying the project
	// directory (the --src path). The runtime sets it on the process
	// environment (and ring meta) so a target can locate the project root.
	ProjectDirEnvKey = "GOMAKE_PROJECT_DIR"
)
