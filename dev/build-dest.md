# gomake build & run destinations

We use gomake in two cases, plus a testing constraint.

## 1) Installation

It compiles gomake and installs it into **GOBIN** (or `$GOPATH/bin` when GOBIN
is unset). Set GOBIN to install elsewhere — e.g. `GOBIN=$PWD/dist` for a
project-local build. There is no install flag for the destination; GOBIN is the
only knob.

There are two compilation modes:

### a) With external targets

For a **published** install (`go run …/cmd/install@version`), the gomake
source is copied from the **read-only module cache** to a temporary directory
(we must never write into the module cache). For a **devel** install
(`go run ./cmd/install`) it builds **in the working tree in place** — no temp
copy.

In the writable build tree, `go get` fetches the imported target modules and the
built-in targets are regenerated; if `--targets` was given, that `targets.yaml`
is written into the build dir (otherwise the source's own `targets.yaml` is
used). Then gomake is compiled and placed in GOBIN.

### b) Without external targets

Compiled directly from the read-only source in place (working tree for devel,
module-cache dir for published) — no copy, no `go get`, no regeneration —
and placed in GOBIN.

## 2) Gomake is run in a project

### a) With the user's `makefile.go` and targets

The user makefile(s) are copied to a build (temp) directory, together with
generated glue/main and empty built-in targets, compiled into a binary in that
directory, and run as a subprocess with the **working directory set to `--wd`
(defaults to the invocation cwd, which is also where `makefile.go` is found by
default)**. GOBIN is not involved in the run flow — it only affects where
`install` places the gomake binary. A compiled makefile is cached and reused
when sources, version, and GOOS/GOARCH are unchanged.

### b) In a directory without `makefile.go`

Runs the built-in targets embedded in the current gomake binary in-process,
without compiling anything. A non-built-in target name yields an "unknown
target" error.

## 3) Testing gomake

### a) Installation tests must never write to the real GOBIN

`install.Main` resolves the destination from GOBIN, but tests call the internal
`installTo(rng, dst, …)` indirection with an explicit per-test temporary `dst`,
so nothing lands in the real GOBIN or the project tree. The one test that must
go through `Main` points GOBIN at a temp dir via `rng.EnvSet("GOBIN", …)`.
