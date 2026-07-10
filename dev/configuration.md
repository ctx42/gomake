# Target configuration — internals

Maintainer-facing notes on how a target's `gomake.yaml` block travels from disk
to the running target. The user-facing reference lives in
`docs/content/docs/configuration.md`; this document covers the code that
implements it.

## The contract

A target reads its configuration through a single ring meta-store key holding a
JSON string:

- **Key:** `gomake.ConfigMetaKey` (`pkg/gomake/config.go`), whose value is
  `github.com/ctx42/gomake/pkg/gomake.targetConfig`.
- **Value:** the target's resolved settings block, `json.Marshal`ed.
- **Read:** `gomake.TargetConfig(rng)` returns a `*gomake.Config` — it
  `MetaLookup`s the key and `json.Unmarshal`s the block once into a
  `map[string]any` (`pkg/gomake/config.go`). A missing key or a non-string
  value yields an empty `Config` and a nil error; only a malformed block is an
  error.

The environment is deliberately never used to carry configuration — it stays
free for a target's own override logic.

## Producer side

All of the machinery lives in `internal/cli/config_file.go`. Nothing here is
exported; the target only ever sees the meta key above.

### Load

`config.applyFileConfig` (`config_file.go`) loads both levels and stores their
target trees on the `config`:

| Field                   | Source                                            |
|-------------------------|---------------------------------------------------|
| `cfg.userTargets`       | `$XDG_CONFIG_HOME/gomake/gomake.yaml` `targets:`  |
| `cfg.projectTargets`    | `<src>/gomake.yaml` `targets:`                     |

`loadConfigFile` → `parseConfigFile` parse `version` and `settings` strictly
(`yaml.Strict()`, unknown keys rejected) but keep `targets:` as an opaque
`map[string]any` tree. Only `settings` is merged (`mergeConfigs`, project wins);
the two target trees stay separate and are resolved per invocation.

### Resolve

`deliverTargetConfig(rng, cfg, tgt, tgts)` is called once per run — from the
built-in fast path (`main.go`, `runWithoutCompile`) and from the compile path
(`main.go`, before `gmk.Execute`). It delegates to `resolveDelivered`:

1. `imp` = the target's import path (`tgt.ImpSpec`, or `localImportPath(src)`
   for a target defined in the project's own makefile package).
2. `path` = `namePath(tgt.Name)` — the kebab name split on `:`, e.g.
   `"go:lint"` → `{"go","lint"}`.
3. `known` = `knownNodes(...)` — every colon-joined name-path prefix a
   discovered target under `imp` contributes. This lets the resolver tell a
   child-node key from a setting key.
4. Look up the **project** tree at `imp`; if present, `resolveTargetBlock`.
5. Otherwise, unless `imp` lies inside the current module, fall back to the
   **user** tree.

`resolveTargetBlock` walks the tree along `path` as far as both allow, then
walks back **up** to the first node carrying at least one setting key. That is
the nearest-level block. `settingKeys` returns a shallow copy of that node with
child-node keys (those in `known`) stripped, so a delivered block never carries
a nested target's configuration.

The selected block is `json.Marshal`ed and `rng.MetaSet` under
`gomake.ConfigMetaKey`. For an **in-process** target (library use / built-in
fast path) that is the whole delivery — same ring, target reads it directly.

## Subprocess bridge

A compiled makefile runs as a separate process and does not share the parent's
meta store, so the JSON is ferried across the boundary as an internal argument
(`internal/cli/config_file.go`, `targetConfigArg` = `--gomake-config`):

```
cli.go (parent)                gen_main.go (generated child main)
  MetaLookup(ConfigMetaKey)      scan os.Args for --gomake-config=<json>
      │                              │  strip it from args (never reaches target)
      ▼                              ▼
  prepend --gomake-config=<json>  MetaSet(ConfigMetaKey, <json>)  ── child ring
  to the subprocess args
```

- `cli.go` (`Execute`) reads the meta key and prepends `--gomake-config=<json>`
  to the subprocess argv.
- `gen_main.go`'s template (`mainTpl`) generates the child `main`, which strips
  the argument from `os.Args` and re-stores its value under `ConfigMetaKey` in
  the child ring. The argument never reaches the target as positional input.

The key string and argument name are injected into the template from
`gomake.ConfigMetaKey` and `targetConfigArg`, so the two processes agree by
construction.

## Consumer side

The target — in-process or subprocess — reads the block identically. It
constructs a `*gomake.Config` once, then pulls typed values by dotted path with
the generic `gomake.GetCfg[T]`:

```go
func Show(_ context.Context, rng *ring.Ring) error {
    cfg, err := gomake.TargetConfig(rng)
    if err != nil {
        return err
    }
    msg, err := gomake.GetCfg[string](cfg, "message")
    if err != nil && !errors.Is(err, gomake.ErrMiss) {
        return err
    }
    // msg is "" when the block has no "message" key.
    ...
}
```

`GetCfg[T]` resolves the path against the block, then converts the value to
`T` through a JSON round-trip, so `T` may be a scalar, slice, map, or a
json-tagged struct; `time.Duration` is parsed from a string, and `GetCfg[any]`
returns the raw value. It returns `ErrMiss` when the path is absent and
`ErrType` on a type mismatch. Because methods cannot be generic in Go,
`GetCfg` is a free function taking the `*Config` as its first argument, not a
method. `Config.Has(path)` tests presence without an `errors.Is` dance.

See `testdata/projects/config_target/project/makefile.go`.

## `--check-config`

`runCheckConfig` → `checkConfigReport` (`config_file.go`) re-loads both files,
lists every discovered target grouped by import path (`checkConfigReport`), and
reports the soft problems a normal run tolerates (`checkConfigProblems`):
non-absolute `settings.tmp`, a project import key whose value is not a mapping,
and a project import key matching no discovered target. It never judges setting
keys — those are opaque to gomake.

## Related, but separate

`targets.yaml` (`internal/cli/imports_config.go`, `TargetsFile`) is unrelated to
`gomake.yaml`. It declares external target imports at install time, and its
per-import `config` field is delivered through a different meta key
(`ImportEntry.MetaKey`, set by `applyExternalTargetMeta` in `main.go`).
