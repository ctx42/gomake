---
title: "GoMake"
description: "Go-native build automation — write targets as functions, not shell scripts."
---

{{< hero
  badge="Go-native build automation"
  title="Build automation<br>written in Go"
  tagline="Write ordinary Go functions. GoMake discovers them as runnable targets — with docs, arguments, namespaces, cross-package imports, and full testability."
  cta1_text="Get started"
  cta1_url="/docs/getting-started/"
  cta2_text="View source"
  cta2_url="https://github.com/ctx42/gomake"
>}}

{{< terminal >}}
{{< highlight go >}}
// Build initializes the target with a [context.Context] for cancellation and a
// [ring.Ring] encapsulating I/O streams, environment variables, and arguments.
// Dependencies are injected explicitly to simplify testing.
func Build(ctx context.Context, rng *ring.Ring) error {
    return exec.CommandContext(ctx, "go", "build", "./...").Run()
}
{{< /highlight >}}
{{< highlight bash >}}
$ gomake build
✓ done in 1.2s

$ gomake --list
build      compiles the project
test       runs the test suite
ci:lint    runs golangci-lint
{{< /highlight >}}
{{< /terminal >}}

{{< pillars
  label="Why GoMake"
  title="Three reasons teams reach for it"
  sub="One signature, plain Go, a compiled-and-cached binary. What sets GoMake apart comes down to three things."
>}}

{{< pillar num="1" title="Build automation written in Go" link_url="/docs/writing-targets/" link_text="Writing targets" >}}
Targets are ordinary Go functions — same language, same IDE, same tooling.
No DSL, no YAML, no shell quoting. Use any package from the ecosystem.
{{< /pillar >}}

{{< pillar num="2" title="Shared across projects and people" link_url="/docs/imports/" link_text="Imports guide" >}}
Publish your build logic once. Every repo and every teammate pulls it in with
a single `//gomake:import` — and `go get -u` ships a fix to all of them.
{{< /pillar >}}

{{< pillar num="3" title="Fully testable" link_url="/docs/testing/" link_text="Testing guide" >}}
Targets receive I/O, args, and env through `ring.Ring`. Call them directly in
tests, assert their output — no subprocess, no global state, no temp files.
{{< /pillar >}}

{{< /pillars >}}

{{< cols
  label="Sharing"
  title="Write a target once. The whole team runs it."
  sub="Build logic doesn't have to live in one repo. Put your targets in a shared package, and every project imports them with one comment. Fix a bug once and `go get -u` propagates it to every repo and every teammate — no copy-paste, no drift."
  link_url="/docs/imports/"
  link_text="Read the imports guide →"
>}}

{{< col title="Author once — a shared package" >}}
```go
// Package lint holds the org's standard
// linting targets, shared across every repo.
package lint

// Lint runs the shared golangci-lint config.
func Lint(ctx context.Context, rng *ring.Ring) error {
    cmd := exec.CommandContext(ctx,
        "golangci-lint", "run")
    cmd.Stdout = rng.Stdout()
    cmd.Stderr = rng.Stderr()
    return cmd.Run()
}
```
{{< /col >}}

{{< col title="Import anywhere — one comment" >}}
```go
//go:build gomake

package main

import (
    // Merged into the root namespace.
    _ "acme.dev/mk/lint" //gomake:import

    // Prefixed under "release:".
    _ "acme.dev/mk/release" //gomake:import release
)
```
{{< /col >}}

{{< col title="Local and shared targets, side by side" span="2" >}}
```shell
$ gomake --list
build            compiles the project                    # local
lint             runs the shared golangci-lint config    # imported
release:tag      tags the next release                   # imported
release:publish  publishes the build artifacts           # imported
```
{{< /col >}}

{{< /cols >}}

{{< features
  label="More"
  title="And the rest of the toolbox"
  sub="Everything else GoMake gives you out of the box — no configuration required."
>}}

{{< feature icon="🔌" title="Builtin targets" >}}
Customise your binary at install time — compile shared target packages in so
they are available in every project without a `makefile.go`.
{{< /feature >}}

{{< feature icon="📂" title="Namespaces" >}}
Group related targets under a prefix using typed struct receivers. Nest
namespaces to get CLI names like `ci:docker:build`.
{{< /feature >}}

{{< feature icon="📝" title="Docs from comments" >}}
First sentence of a doc comment becomes the synopsis. Full comment appears in
`--help <target>`.
{{< /feature >}}

{{< feature icon="🖥️" title="Cross-platform" >}}
Go build tags isolate platform-specific targets. `GOOS` and `GOARCH` control
compilation; `go.work` supported.
{{< /feature >}}

{{< feature icon="🔧" title="Standalone binary" >}}
Use `--bin` to compile a self-contained binary for CI or Docker images that
don't have gomake installed.
{{< /feature >}}

{{< feature icon="⚡" title="Cached builds" >}}
Your targets compile once, keyed by a SHA-256 of the sources. Unchanged runs
skip compilation entirely and start instantly.
{{< /feature >}}

{{< /features >}}

{{< cols
  label="Testing"
  title="Targets are plain functions — test them that way"
  sub="Because all I/O flows through `ring.Ring`, you can call a target directly in a test, inject a buffer for stdout, set env vars, pass args — and assert the output. No subprocesses, no temp files, no global state."
  link_url="/docs/testing/"
  link_text="Read the testing guide →"
>}}

{{< col title="makefile.go" >}}
```go
func Greet(ctx context.Context, rng *ring.Ring) error {
    name := rng.EnvGet("GREETER_NAME")
    if name == "" {
        name = "World"
    }
    if args := rng.Args(); len(args) > 0 {
        name = args[0]
    }
    out := rng.Stdout()
    _, err := fmt.Fprintf(out, "Hello, %s!\n", name)
    return err
}
```
{{< /col >}}

{{< col title="makefile_test.go" >}}
```go
func TestGreet(t *testing.T) {
    // --- Given ---
    var out bytes.Buffer
    rng := ring.New(
        ring.WithArgs([]string{"Alice"}),
        ring.WithEnv([]string{"GREETER_NAME=Bob"}),
    )
    rng.SetStdout(&out)

    // --- When ---
    err := Greet(t.Context(), rng)

    // --- Then ---
    assert.NoError(t, err)
    // Argument value wins over environment variable.
    assert.Equal(t, "Hello, Alice!\n", out.String())
}
```
{{< /col >}}

{{< /cols >}}

{{< compare
  label="At a glance"
  title="Everything GoMake gives you"
  sub="One signature, plain Go, and a compiled-and-cached binary. Here is what you get out of the box."
>}}

| Capability            | How GoMake does it                              |
|-----------------------|-------------------------------------------------|
| Target signature      | `func(ctx, *ring.Ring) error` — one form        |
| I/O access            | `rng.Stdout()` / `rng.Stderr()` — injectable     |
| Arguments             | `rng.Args()` — the full `[]string`              |
| Environment           | `rng.EnvGet` — isolated and seedable in tests   |
| Testing               | Call the target directly with a controlled Ring |
| Namespaces            | Methods on `//gomake:ns_root` structs, nestable |
| Cross-package imports | `//gomake:import` comment tag                   |
| Built-in targets      | `targets.yaml` — compiled into the binary       |
| Binary cache          | SHA-256 of sources + `go.sum` + version + arch  |
| Progress feedback     | Automatic, after 500 ms                         |
| Cross-platform        | Go build tags + `GOOS`/`GOARCH`; `go.work` too  |
| Standalone binary     | `gomake --bin ./make`                           |

{{< /compare >}}

{{< cta
  title="Ready to start?"
  sub="Read the docs, learn to test your targets, or jump straight to the source."
  btn1_text="Read the docs"
  btn1_url="/docs/getting-started/"
  btn2_text="Testing guide"
  btn2_url="/docs/testing/"
>}}
