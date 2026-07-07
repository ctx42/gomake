// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package mkf

import (
	"fmt"
	"strings"
)

// GoCode returns Go source code defining the target. If qualify is true, the
// package qualifier is added to the [Target] references (e.g.: mkf.Target vs
// Target).
func (tgt *Target) GoCode(qualify bool) string {
	format := "" +
		"\tImpSpec:     %q,\n" +
		"\tImpPath:     %q,\n" +
		"\tPkgName:     %q,\n" +
		"\tPkgNS:       %q,\n" +
		"\tBreadcrumbs: %s,\n" +
		"\tReceiver:    %q,\n" +
		"\tFuncName:    %q,\n" +
		"\tName:        %q,\n" +
		"\tVarName:     %q,\n" +
		"\tCodeRef:     %q,\n" +
		"\tDefRef:      %q,\n" +
		"\tDefault:     %t,\n" +
		"\tSynopsis:    %q,\n" +
		"\tDoc:         %q,\n" +
		"\tHidden:      %t,\n" +
		"\tRun: func(ctx context.Context, rng *ring.Ring) error {\n" +
		"\t%s\n" +
		"\t},\n"

	nsp := "nil"
	if tgt.Breadcrumbs != nil {
		nsp = fmt.Sprintf("%#v", tgt.Breadcrumbs)
	}

	code := fmt.Sprintf(
		format,
		tgt.ImpSpec,
		tgt.ImpPath,
		tgt.PkgName,
		tgt.PkgNS,
		nsp,
		tgt.Receiver,
		tgt.FuncName,
		tgt.Name,
		tgt.VarName,
		tgt.CodeRef,
		tgt.DefRef,
		tgt.Default,
		tgt.Synopsis,
		tgt.Doc,
		tgt.Hidden,
		tgt.genRunCode(1),
	)

	qual := ""
	if qualify {
		qual = "mkf."
	}
	return qual + "Target{\n" + code + "}"
}

// genRunCode generates code for run function.
func (tgt *Target) genRunCode(indent int) string {
	tabs := strings.Repeat("\t", indent)
	return fmt.Sprintf(tabs+"return %s(ctx, rng)", tgt.CodeRef)
}

// IsCore returns true if the target is the core target.
func (tgt *Target) IsCore() bool {
	return tgt.Name != "" && tgt.Name[0] == ':'
}
