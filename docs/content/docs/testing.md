---
title: "Testing Targets"
description: "Test targets as functions by injecting I/O, environment, and arguments."
weight: 6
---

## The testing advantage

Because every target receives its I/O, environment, and arguments through
`ring.Ring` rather than reading from global state (`os.Stdout`, `os.Getenv`,
`os.Args`), targets are functions in the truest sense: given the same Ring,
they produce the same output.

This sidesteps the awkward ways build logic is usually tested:

- spawning a subprocess and parsing text output,
- monkey-patching `os.Stdout`,
- writing to a temp file and reading it back.

With GoMake you call the function directly, hand it a Ring with controlled
I/O, and inspect the result. No process overhead, no flakiness, no cleanup.

```go
// makefile.go
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

```go
// makefile_test.go
func TestGreet(t *testing.T) {
    // --- Given ---
    var out bytes.Buffer
    rng := ring.New(ring.WithArgs([]string{"Alice"}))
    rng.SetStdout(&out)

    // --- When ---
    err := Greet(t.Context(), rng)

    // --- Then ---
    assert.NoError(t, err)
    assert.Equal(t, "Hello, Alice!\n", out.String())
}
```

That's it. The test runs in microseconds and exercises the real target code.

---

## Imports

```go
import (
    "bytes"
    "testing"

    "github.com/ctx42/ring/pkg/ring"
    "github.com/ctx42/ring/pkg/ring/ringtest"
)
```

---

## Capturing output

Redirect stdout and stderr to `bytes.Buffer` to inspect what a target writes:

```go
func TestBuild_PrintsArtifactPath(t *testing.T) {
    // --- Given ---
    var stdout, stderr bytes.Buffer
    rng := ring.New()
    rng.SetStdout(&stdout)
    rng.SetStderr(&stderr)

    // --- When ---
    err := Build(t.Context(), rng)

    // --- Then ---
    assert.NoError(t, err)
    assert.Contains(t, stdout.String(), "dist/myapp")
    assert.Empty(t, stderr.String())
}
```

The key discipline: **every write in a target must go through `rng.Stdout()`
or `rng.Stderr()`**. Any `fmt.Println` or `os.Stdout.Write` call bypasses the
injected buffer and makes the target untestable.

---

## Injecting arguments

`ring.WithArgs` sets the argument slice exactly as it arrives from the CLI
after the target name:

```go
func TestDeploy_ProdFlag(t *testing.T) {
    // --- Given ---
    var out bytes.Buffer
    rng := ring.New(ring.WithArgs([]string{"--env", "production"}))
    rng.SetStdout(&out)

    // --- When ---
    err := Deploy(t.Context(), rng)

    // --- Then ---
    assert.NoError(t, err)
    assert.Contains(t, out.String(), "production")
}
```

You can also set args on an existing Ring:

```go
rng := ring.New()
rng.SetArgs([]string{"--dry-run"})
```

---

## Injecting environment variables

`ring.WithEnv` provides a completely isolated environment — only the variables
you list exist. The test runner's own environment does not leak in:

```go
func TestGreet_FromEnv(t *testing.T) {
    // --- Given ---
    var out bytes.Buffer
    rng := ring.New(ring.WithEnv([]string{"GREETER_NAME=Bob"}))
    rng.SetStdout(&out)

    // --- When ---
    err := Greet(t.Context(), rng)

    // --- Then ---
    assert.NoError(t, err)
    assert.Equal(t, "Hello, Bob!\n", out.String())
}
```

Add or override individual variables with `EnvSet`:

```go
rng := ring.New()
rng.EnvSet("DATABASE_URL", "postgres://localhost/testdb")
rng.EnvSet("LOG_LEVEL", "debug")
```

Remove a variable with `EnvUnset`:

```go
rng.EnvUnset("CI") // pretend we're not in CI
```

---

## Using ringtest.Tester

`ringtest.Tester` is a structured test helper that catches unexpected I/O
automatically. Its signature approach is:

- **Dry buffers by default** — any write to stdout or stderr that you haven't
  declared as expected fails the test (reported via `t.Error` at test cleanup).
- **Wet buffers on demand** — call `WetStdout()` or `WetStderr()` to declare
  that a stream will be written to. A wet stream also expects you to read it
  back with `Stdout()` / `Stderr()`, so the examples below always do.

```go
func TestBuild(t *testing.T) {
    // --- Given ---
    tst := ringtest.New(t)
    tst.WetStdout()

    // --- When ---
    err := Build(t.Context(), tst.Ring())

    // --- Then ---
    assert.NoError(t, err)
    assert.Contains(t, tst.Stdout(), "compiled")
}
```

Pass arguments to `tst.Ring(...)`:

```go
rng := tst.Ring("--env", "staging", "v1.2.3")
// rng.Args() == []string{"--env", "staging", "v1.2.3"}
```

### Tester API reference

| Method                       | Description                                                          |
|------------------------------|----------------------------------------------------------------------|
| `ringtest.New(t, opts...)`   | Create a Tester; accepts any `ring.Option` for env, meta, clock, etc.|
| `tst.Ring(args ...string)`   | Return a `*ring.Ring` wired to test buffers, with args set           |
| `tst.WetStdout()`            | Expect writes to stdout; fail the test if none come                  |
| `tst.WetStderr()`            | Expect writes to stderr; fail the test if none come                  |
| `tst.Stdout()`               | Return all bytes written to stdout so far                            |
| `tst.Stderr()`               | Return all bytes written to stderr so far                            |
| `tst.ResetStdout()`          | Clear the stdout buffer (useful between sub-tests)                   |
| `tst.ResetStderr()`          | Clear the stderr buffer                                              |
| `tst.SetStdin(buf)`          | Provide content for targets that read standard input                 |

### Checking for unexpected writes

The dry-buffer behaviour is the most powerful part. Without it you would need
`assert.Empty(t, stderr.String())` on every success-path test. With dry
buffers, any unexpected write fails the test at cleanup, and the diagnostic
reports the bytes that were written to a stream expected to stay empty.

```go
func TestBuild_NoStderr(t *testing.T) {
    // --- Given ---
    tst := ringtest.New(t)
    tst.WetStdout()
    // Not calling tst.WetStderr() — any write to stderr fails the test.

    // --- When ---
    _ = Build(t.Context(), tst.Ring())

    // --- Then ---
    // If Build writes to stderr, the dry buffer fails the test automatically.
}
```

---

## Injecting metadata

Targets can read typed values from the Ring metadata store — a
`map[string]any` that survives the whole execution context. Use it to pass
configuration structs rather than environment strings:

```go
type DockerConfig struct {
    Registry string
    Tag      string
}

func DockerPush(ctx context.Context, rng *ring.Ring) error {
    raw := rng.MetaGet("docker")
    cfg, ok := raw.(DockerConfig)
    if !ok {
        return errors.New("docker: missing config in ring metadata")
    }
    ref := cfg.Registry + ":" + cfg.Tag
    fmt.Fprintf(rng.Stdout(), "pushing %s\n", ref)
    return runCmd(ctx, rng, "docker", "push", ref)
}
```

```go
func TestDockerPush_Config(t *testing.T) {
    // --- Given ---
    tst := ringtest.New(t, ring.WithMeta(map[string]any{
        "docker": DockerConfig{
            Registry: "registry.example.com/myapp",
            Tag:      "v2.0.0",
        },
    }))
    tst.WetStdout()

    // --- When ---
    err := DockerPush(t.Context(), tst.Ring())

    // --- Then ---
    assert.NoError(t, err)
    assert.Contains(t, tst.Stdout(), "registry.example.com/myapp:v2.0.0")
}
```

`MetaSet` and `MetaGet` work on any ring after construction too:

```go
rng := ring.New()
rng.MetaSet("config", MyConfig{...})
val := rng.MetaGet("config").(MyConfig)
```

---

## Injecting a filesystem

Targets that read configuration or template files can accept an `fs.FS`
through the Ring instead of calling `os.Open`. This means no disk access in
tests:

```go
func Generate(ctx context.Context, rng *ring.Ring) error {
    fsys, err := rng.FS()
    if err != nil {
        return fmt.Errorf("generate: %w", err)
    }
    tmpl, err := fs.ReadFile(fsys, "templates/header.tmpl")
    if err != nil {
        return err
    }
    fmt.Fprintf(rng.Stdout(), string(tmpl))
    return nil
}
```

```go
func TestGenerate(t *testing.T) {
    // --- Given ---
    fsys := fstest.MapFS{
        "templates/header.tmpl": {Data: []byte("# Generated\n")},
    }
    tst := ringtest.New(t, ring.WithFS(fsys))
    tst.WetStdout()

    // --- When ---
    err := Generate(t.Context(), tst.Ring())

    // --- Then ---
    assert.NoError(t, err)
    assert.Equal(t, "# Generated\n", tst.Stdout())
}
```

---

## Table-driven tests

Table-driven tests stay clean because each case creates its own Ring with
its own buffers:

```go
func TestGreet_Cases(t *testing.T) {
    cases := []struct {
        name    string
        args    []string
        env     []string
        wantOut string
    }{
        {
            name:    "arg wins over env",
            args:    []string{"Alice"},
            env:     []string{"GREETER_NAME=Bob"},
            wantOut: "Hello, Alice!\n",
        },
        {
            name:    "env used when no arg",
            env:     []string{"GREETER_NAME=Bob"},
            wantOut: "Hello, Bob!\n",
        },
        {
            name:    "default when nothing set",
            wantOut: "Hello, World!\n",
        },
    }

    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            // --- Given ---
            var out bytes.Buffer
            rng := ring.New(
                ring.WithArgs(tc.args),
                ring.WithEnv(tc.env),
            )
            rng.SetStdout(&out)

            // --- When ---
            err := Greet(t.Context(), rng)

            // --- Then ---
            assert.NoError(t, err)
            assert.Equal(t, tc.wantOut, out.String())
        })
    }
}
```

---

## Testing error paths

Verify that targets return correct errors and write nothing to the wrong
stream on failure:

```go
func TestDeploy_MissingConfig(t *testing.T) {
    // --- Given ---
    tst := ringtest.New(t)
    // No WetStdout() — success output must not appear.
    // No WetStderr() — error should come from the return value, not stderr.

    // --- When ---
    err := Deploy(t.Context(), tst.Ring())

    // --- Then ---
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "missing config")
    assert.Empty(t, tst.Stdout())
}
```

---

## Ring.Clone for shared base state

`ring.Clone()` copies environment, metadata, and clock into a new Ring with
**fresh I/O buffers**. Use it when all test cases share a base configuration:

```go
func TestDeploy(t *testing.T) {
    // --- Given ---
    base := ring.New(ring.WithEnv([]string{
        "DEPLOY_HOST=deploy.example.com",
        "DEPLOY_USER=ci",
    }))

    t.Run("staging", func(t *testing.T) {
        // --- Given ---
        var out bytes.Buffer
        rng := base.Clone()
        rng.SetStdout(&out)
        rng.SetArgs([]string{"--env", "staging"})

        // --- When ---
        err := Deploy(t.Context(), rng)

        // --- Then ---
        assert.NoError(t, err)
        assert.Contains(t, out.String(), "staging")
    })

    t.Run("production", func(t *testing.T) {
        // --- Given ---
        var out bytes.Buffer
        rng := base.Clone()
        rng.SetStdout(&out)
        rng.SetArgs([]string{"--env", "production"})

        // --- When ---
        err := Deploy(t.Context(), rng)

        // --- Then ---
        assert.NoError(t, err)
        assert.Contains(t, out.String(), "production")
    })
}
```

---

## Design guidelines for testable targets

Following these rules makes every target trivially testable:

1. **Always use `rng.Stdout()` and `rng.Stderr()`** — never `os.Stdout`,
   `fmt.Println`, or `log.Print`.

2. **Read environment through `rng.EnvGet`** — never `os.Getenv`.

3. **Read arguments from `rng.Args()`** — never `os.Args`.

4. **Pass configuration through Ring metadata** for complex structured values,
   rather than encoding them as environment strings.

5. **Accept `fs.FS` via the Ring** for any file-reading logic you want to test
   without touching disk.

6. **Keep targets thin**: delegate real work to ordinary Go functions that
   accept explicit parameters, then test those functions separately. The target
   itself becomes a thin adapter that reads from the Ring and passes values
   down — simple to test, easy to understand.

```go
// Thin target adapter — easy to test with Ring injection.
func Build(ctx context.Context, rng *ring.Ring) error {
    goos := rng.EnvGet("GOOS")
    output := rng.EnvGet("BUILD_OUTPUT")
    if output == "" {
        output = "dist/myapp"
    }
    return buildBinary(ctx, rng.Stdout(), rng.Stderr(), goos, output)
}

// Pure function with explicit parameters — easy to unit test directly.
func buildBinary(ctx context.Context, out, errOut io.Writer, goos, dst string) error {
    // ...
}
```
