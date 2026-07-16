// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

// Package gomake provides the toolkit available to gomake target authors:
// reading the running target's name ([TargetName]), locating the module root
// ([Root]), reading per-target configuration ([TargetConfig], [GetCfg]), and
// small filesystem and environment helpers. It is the only gomake package a
// makefile.go or a built-in target package needs to import.
package gomake
