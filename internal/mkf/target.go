// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package mkf

import (
	"context"

	"github.com/ctx42/ring/pkg/ring"
)

// --- CODE MARK ---

// targetFn is the signature for a gomake target. While the target runs, its
// name is available via [github.com/ctx42/gomake/pkg/gomake.TargetName].
type targetFn func(ctx context.Context, rng *ring.Ring) error

// PreRunFn is the function signature that runs before a target. The function
// must return the received context and the ring even when the error is not nil.
type PreRunFn func(
	ctx context.Context,
	rng *ring.Ring,
) (context.Context, *ring.Ring, error)

// Target represents a runnable gomake target. The zero value must not be
// executed ([Target.Run] is nil); use [NewTarget] or the parser to obtain
// valid targets. Valid target signature:
//
//	func Target(ctx context.Context, rng *ring.Ring) error
type Target struct {
	// Import spec the target belongs to. It's empty for main packages.
	ImpSpec string

	// Absolute path to the package directory target belongs to.
	ImpPath string

	// Package name target belongs to.
	PkgName string

	// Namespace to prefix all targets in the package with. It's set if the
	// target is part of the package imported with a namespace.
	//
	// Example namespaced import:
	//   import "my/package" //gomake:import
	//   import "my/package" //gomake:import ns
	//
	// The second example would result in PkgNS to be set to "ns".
	PkgNS string

	// Represents the list of crumbs (elements) leading from the namespace root
	// to the type the target is method of.
	//
	// Imagine we have types like below:
	//
	//  type A struct{} //gomake:ns_root
	//  type B A
	//  type C B
	//
	// the resulting breadcrumbs will be:
	//
	//  []string{"__root__", "A", "B", "C"}
	//
	// The full target name is built from PkgNS and Breadcrumbs.
	Breadcrumbs []string

	// Holds type name for targets which are methods. For functions, it's empty.
	Receiver string

	// Name of the target's function or method.
	FuncName string

	// Target's documentation string.
	Doc string

	// Runs the target with given context and arguments.
	Run targetFn

	// Is the target the default target?
	//
	// Examples:
	//   var Default = Target
	//   var Default = NS.Target
	//   var Default = pkg.Target
	//   var Default = pkg.NS.Target
	//
	Default bool

	// Target will be hidden from the target list if this is set to true.
	//
	// Hidden is a derived field, calculated based on Doc.
	Hidden bool

	// Name used when listing or executing target (required).
	//
	// Examples:
	//   - target
	//   - ns:target
	//   - ns:target-kebab
	//
	// Name is a derived field, calculated based on PkgNS, Breadcrumbs,
	// Receiver and FuncName.
	Name string

	// Target's synopsis based on documentation string.
	//
	// Synopsis is a derived field, calculated based on FuncName and Doc.
	Synopsis string

	// Variable name to create for namespaced targets.
	//
	// To create namespaced targets, gomake uses structures (types) which must
	// be declared so the methods can be called. This field holds the name of
	// the variable name to use. The value of the Receiver indicates the type
	// name.
	//
	// Examples:
	//   var _vNS pkg.Name
	//
	// VarName is a derived field, calculated based on PkgName and Receiver.
	VarName string

	// Code reference (required).
	//
	// Value is used as a callable in [Target.Run] function.
	//
	// Examples:
	//   - Target
	//   - _vNS.Target
	//   - pkg.Target
	//
	// CodeRef is a derived field, calculated based on PkgName, VarName and
	// FuncName.
	CodeRef string

	// Code reference used to declare this target as default.
	//
	// Example:
	//   var Default = NS.Target
	//
	// DefRef is a derived field, calculated based on PkgName, Receiver and
	// FuncName.
	DefRef string
}

// NewTarget returns a new instance of [Target] with all fields set to their
// zero values except [Target.Run] field which is set to no-op function.
func NewTarget() *Target {
	return &Target{Run: nop}
}

// nop is a function matching [Target.Run] signature that does nothing and can
// be used as a placeholder.
func nop(context.Context, *ring.Ring) error { return nil }
