// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package parser

import (
	"errors"
	"go/ast"
	"go/doc"
	"strings"

	"github.com/ctx42/gomake/internal/mkf"
)

// Target related errors.
var (
	// errNotExported is error returned when target is not exported.
	errNotExported = errors.New("not exported target")

	// errInvResult is an error used to indicate a target return is invalid.
	// All targets must have only return of type error.
	errInvResult = errors.New("target must have single return of type error")

	// errInvArg is an error indicating a target has invalid argument(s).
	errInvArg = errors.New("invalid target argument")

	// ErrDupTarget is an error indicating a target (name) is a duplicate
	// of already existing target.
	ErrDupTarget = errors.New("duplicated target")
)

// newTarget returns new Target instance based on function / method
// documentation and package it belongs to. The crumbs define the target's
// breadcrumbs.
func newTarget(
	pkg *Package,
	df *doc.Func,
	crumbs ...string,
) (*mkf.Target, error) {

	if !ast.IsExported(df.Name) {
		return nil, errNotExported
	}

	tgt := mkf.NewTarget()
	tgt.ImpSpec = pkg.ImpSpec
	tgt.ImpPath = pkg.ImpPath
	tgt.PkgName = pkg.Name
	tgt.PkgNS = pkg.PkgNS
	tgt.Breadcrumbs = crumbs
	tgt.Receiver = df.Recv
	tgt.FuncName = df.Name

	ft := df.Decl.Type

	if err := checkParams(ft.Params); err != nil {
		return nil, err
	}

	if err := checkResults(ft.Results); err != nil {
		return nil, err
	}

	tgt.Doc, tgt.Hidden = isHidden(df.Doc)
	tgt.Doc = removeNoLintComments(tgt.Doc)
	tgt.Doc = toOneLine(tgt.Doc)
	tgt.Doc = strings.TrimPrefix(tgt.Doc, tgt.FuncName+" ")
	setDerivedFields(tgt)

	return tgt, nil
}

// setDerivedFields sets target's calculated fields.
func setDerivedFields(tgt *mkf.Target) {
	tgt.Name = targetName(tgt.PkgNS, tgt.Breadcrumbs, tgt.FuncName)

	// Code reference.
	var ref string
	if tgt.PkgName != MainName {
		ref = tgt.PkgName + "."
	}
	if len(tgt.Breadcrumbs) >= 2 {
		tgt.VarName = "_v" + tgt.PkgName + tgt.Receiver
		tgt.CodeRef = tgt.VarName + "." + tgt.FuncName
		ref = ref + tgt.Receiver + "."
	} else {
		tgt.CodeRef = ref + tgt.FuncName
	}
	tgt.DefRef = ref + tgt.FuncName

	// Help / Documentation.
	tgt.Synopsis = targetSynopsis(tgt.FuncName, tgt.Doc)
}

// targetName returns CLI target name based on package namespace, breadcrumbs
// and function / method name.
func targetName(pkgNS string, crumbs []string, funcName string) string {
	var name string
	if pkgNS != "" {
		name = pkgNS + ":"
	}
	if len(crumbs) >= 2 {
		for i, ns := range crumbs {
			if ns == rootCrumb {
				continue
			}
			name += strings.ToLower(toKebabCase(ns))
			if i != len(crumbs)-1 {
				name += ":"
			}
		}
		if funcName != "Default" {
			name += ":"
			name += toKebabCase(funcName)
		}
		return name
	}
	return name + toKebabCase(funcName)
}

// checkParams checks target parameters. For invalid parameters returns
// [errInvArg] error. The second parameter must be `*ring.Ring` (pointer).
func checkParams(params *ast.FieldList) error {
	if params == nil || len(params.List) != 2 {
		return errInvArg
	}

	ctxCnt := 0
	argsCnt := 0

	typ := qIdent(params.List[0].Type)
	if typ != "context.Context" {
		return errInvArg
	}
	ctxCnt += len(params.List[0].Names)

	// Require an explicit pointer; qIdent strips * so compare StarExpr first.
	// Known limitation: aliased ring imports (e.g. r "…/ring") are not
	// recognised and will cause errInvArg here.
	star, ok := params.List[1].Type.(*ast.StarExpr)
	if !ok || qIdent(star.X) != "ring.Ring" {
		return errInvArg
	}
	argsCnt += len(params.List[1].Names)

	if ctxCnt > 1 || argsCnt > 1 {
		return errInvArg
	}
	return nil
}

// checkResults checks target results. There should be only one result of
// type error.
func checkResults(res *ast.FieldList) error {
	if res.NumFields() != 1 {
		return errInvResult
	}
	if qIdent(res.List[0].Type) != "error" {
		return errInvResult
	}
	return nil
}
