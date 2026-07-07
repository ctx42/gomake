# Contributing to GoMake

Thanks for your interest in improving GoMake! This guide covers how to build,
test, and submit changes. For the internal architecture and project-specific
conventions, see [`AGENTS.md`](AGENTS.md) and [`dev/README.md`](dev/README.md).

## Prerequisites

- **Go 1.26+** — the module sets this floor in `go.mod`.
- **golangci-lint** — required to run the linter locally (optional but
  recommended before opening a pull request).
- For the documentation site under `docs/`: **Hugo (extended)** and
  **Node.js ≥ 18**. See [`dev/README.md`](dev/README.md).

## Getting started

```shell
# Clone your fork
git clone https://github.com/<you>/gomake
cd gomake

# Build the library packages and the binary
go build ./...
go build -o dist/gomake ./cmd/gomake
```

Do not leave a `./gomake` binary in the repo root — build to `dist/` (which is
git-ignored) as shown above.

## Development workflow

```shell
# Build and vet
go build ./...
go vet ./...

# Run the full test suite with the race detector.
# It takes ~15-25s because pkg/gomake tests compile and run generated makefiles.
go test -race ./...

# Run a targeted test while iterating
go test ./internal/parser -run TestFoo

# Lint
golangci-lint run ./...
```

Some packages generate code. If you change core templates (`makefile.go`,
`target.go`, `helpers.go` in `pkg/gomake`) or generator logic, regenerate:

```shell
go generate ./internal/parser
go generate ./internal/builtin

# After a Go upgrade, refresh the supported GOOS/GOARCH table
go generate ./internal/osarch
```

Commit the regenerated files (including any updated `*.gld` golden files under
`testdata/`) alongside your change.

## Coding conventions

Conventions are documented in detail in [`AGENTS.md`](AGENTS.md). The
essentials:

- **Formatting** — Go files use tabs; Markdown wraps at 80 columns. See
  `.editorconfig`.
- **Tests** — every exported or unexported function/method has a matching
  test named `Test_<FunctionName>` or `Test_<Type>_<Method>` (append `_tabular`
  for table-driven tests). Use `github.com/ctx42/testing/pkg/assert` and the
  `// --- Given/When/Then ---` structure inside subtests.
- **Errors** — define exported sentinel `var Err… = errors.New(…)`, wrap with
  `%w`, and check with `errors.Is` / `errors.As`.
- **I/O** — never use `os.Stdout`/`os.Stderr` directly in library code; thread
  `*ring.Ring` for env, output, args, and metadata. Context is always the
  first parameter.

## Submitting changes

1. Open an issue first for anything non-trivial so we can agree on the
   approach before you invest time.
2. Branch off the default branch and keep your change focused.
3. Ensure `go build ./...`, `go vet ./...`, `go test -race ./...`, and
   `golangci-lint run ./...` all pass.
4. Write clear commit messages. This project uses
   [Conventional Commits](https://www.conventionalcommits.org/) (e.g.
   `feat:`, `fix:`, `docs:`, `chore:`).
5. Open a pull request and fill in the template. Link the issue it resolves.

By contributing, you agree that your contributions are licensed under the
project's [MIT License](LICENSE.md).
