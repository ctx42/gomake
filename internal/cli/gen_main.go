// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package cli

import (
	"os"
	"strings"
	"text/template"

	"github.com/ctx42/gomake/internal/mkf"
	"github.com/ctx42/gomake/pkg/gomake"
)

// codeMarker represents string marking beginning of the file source code. It
// must be placed after `package` and `import` lines.
const codeMarker = "// --- CODE MARK ---\n\n"

// genMain generates "makefile_gen.go" file at path with given version.
func genMain(pth, ver string) error {
	fil, err := os.Create(pth)
	if err != nil {
		return err
	}
	defer func() { _ = fil.Close() }()

	_, mkfSrc, _ := strings.Cut(mkf.MakefileSrc, codeMarker)
	_, infoSrc, _ := strings.Cut(mkf.TargetSrc, codeMarker)
	_, helpersSrc, _ := strings.Cut(mkf.HelpersSrc, codeMarker)

	data := map[string]any{
		"mkf_src":     mkfSrc,
		"info_src":    infoSrc,
		"helpers_src": helpersSrc,
		"version":     ver,
		"config_arg":  targetConfigArg,
		"config_mkey": gomake.ConfigMetaKey,
	}
	return mainTplParsed.Execute(fil, data)
}

// mainTplParsed represents parsed main makefile template.
var mainTplParsed = template.
	Must(template.New("main_mf").
		Funcs(map[string]any{
			"indent": mkf.Indent,
		}).
		Parse(mainTpl))

// mainTpl represents main makefile template.
const mainTpl = `package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/ctx42/ring/pkg/ring"
	"github.com/ctx42/xflag/pkg/xflag"
)

// targets hold list of all targets.
var targets []*Target

func main() { os.Exit(run(context.Background(), ring.New())) }

// run runs the generated makefile.
func run(ctx context.Context, rng *ring.Ring) (ec int) {
	defer func() {
		if v := recover(); v != nil {
			_, _ = fmt.Fprintln(rng.Stderr(), v)
			ec = ExitCode(RecoverError(v))
		}
	}()

	// Load the invoked target's configuration, passed by the gomake process as
	// an internal argument, into the ring meta store the target reads. The
	// argument is stripped so it never reaches the target.
	args := os.Args[1:]
	const cfgPfx = "{{ .config_arg }}="
	for i, a := range args {
		if strings.HasPrefix(a, cfgPfx) {
			rng.MetaSet("{{ .config_mkey }}", a[len(cfgPfx):])
			args = append(args[:i:i], args[i+1:]...)
			break
		}
	}

	ringOF := WithMakefileRing(rng)
	verOF := WithMakefileVersion("{{ .version }}")
	argsOF := WithMakefileArgs(args...)

	mf, err := NewMakefile(targets, ringOF, verOF, argsOF)
	if err != nil {
		_, _ = fmt.Fprintln(rng.Stderr(), err)
		return ExitCode(err)
	}

	err = mf.Execute(ctx)
	if err != nil {
		_, _ = fmt.Fprintln(rng.Stderr(), err)
		return ExitCode(err)
	}
	return 0
}

{{ .info_src }}
{{ .mkf_src }}
{{ .helpers_src }}
`
