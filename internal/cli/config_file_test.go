// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package cli

import (
	"flag"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ctx42/ring/pkg/ring"
	"github.com/ctx42/ring/pkg/ring/ringtest"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/must"
	"github.com/ctx42/testkit/pkg/modkit"
	"github.com/ctx42/testkit/pkg/oskit"
	"github.com/ctx42/testkit/pkg/prjkit"
	"github.com/ctx42/xflag/pkg/xflag"

	"github.com/ctx42/gomake/internal/builtin/builtintest"
	gmt "github.com/ctx42/gomake/internal/cli/clitest"
	"github.com/ctx42/gomake/internal/mkf"
	"github.com/ctx42/gomake/internal/parser"
	"github.com/ctx42/gomake/pkg/gomake"
)

func Test_userConfigPath_tabular(t *testing.T) {
	tt := []struct {
		testN string

		env      []string
		wantPath string
		wantOK   bool
	}{
		{
			"xdg config home wins",
			[]string{"XDG_CONFIG_HOME=/x", "HOME=/home/u"},
			"/x/gomake/gomake.yaml",
			true,
		},
		{
			"falls back to home dot config",
			[]string{"HOME=/home/u"},
			"/home/u/.config/gomake/gomake.yaml",
			true,
		},
		{
			"empty xdg falls back to home",
			[]string{"XDG_CONFIG_HOME=", "HOME=/home/u"},
			"/home/u/.config/gomake/gomake.yaml",
			true,
		},
		{
			"neither set",
			[]string{},
			"",
			false,
		},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have, ok := userConfigPath(tc.env)

			// --- Then ---
			assert.Equal(t, tc.wantOK, ok)
			assert.Equal(t, tc.wantPath, have)
		})
	}
}

func Test_projectConfigPath(t *testing.T) {
	// --- Given ---
	srcDir := "/repo/project"

	// --- When ---
	have := projectConfigPath(srcDir)

	// --- Then ---
	assert.Equal(t, "/repo/project/gomake.yaml", have)
}

func Test_loadConfigFile(t *testing.T) {
	t.Run("missing file returns empty config", func(t *testing.T) {
		// --- Given ---
		pth := filepath.Join(t.TempDir(), configFileName)

		// --- When ---
		cfg, err := loadConfigFile(pth)

		// --- Then ---
		assert.NoError(t, err)
		assert.NotNil(t, cfg)
		assert.Nil(t, cfg.Version)
	})

	t.Run("reads and parses present file", func(t *testing.T) {
		// --- Given ---
		dir := t.TempDir()
		content := "version: 1\nsettings:\n  timeout: 30s\n"
		oskit.Write(t, content, dir, configFileName)
		pth := filepath.Join(dir, configFileName)

		// --- When ---
		cfg, err := loadConfigFile(pth)

		// --- Then ---
		assert.NoError(t, err)
		assert.NotNil(t, cfg.Version)
		assert.Equal(t, 1, *cfg.Version)
		assert.NotNil(t, cfg.Settings)
		assert.Equal(t, "30s", *cfg.Settings.Timeout)
	})

	t.Run("error - invalid content propagates", func(t *testing.T) {
		// --- Given ---
		dir := t.TempDir()
		oskit.Write(t, "version: 0\n", dir, configFileName)
		pth := filepath.Join(dir, configFileName)

		// --- When ---
		cfg, err := loadConfigFile(pth)

		// --- Then ---
		assert.ErrorIs(t, errCfgVersion, err)
		assert.Nil(t, cfg)
	})
}

func Test_parseConfigFile(t *testing.T) {
	t.Run("empty content returns empty config", func(t *testing.T) {
		// --- Given ---
		data := []byte("   \n\t\n")

		// --- When ---
		cfg, err := parseConfigFile(data)

		// --- Then ---
		assert.NoError(t, err)
		assert.Nil(t, cfg.Version)
		assert.Nil(t, cfg.Settings)
		assert.Equal(t, 0, len(cfg.Targets))
	})

	t.Run("minimal file with only version", func(t *testing.T) {
		// --- Given ---
		data := []byte("version: 1\n")

		// --- When ---
		cfg, err := parseConfigFile(data)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, 1, *cfg.Version)
	})

	t.Run("target tree is kept opaque", func(t *testing.T) {
		// --- Given ---
		data := []byte("" +
			"version: 1\n" +
			"targets:\n" +
			"  github.com/acme/tasks:\n" +
			"    deploy:\n" +
			"      region: eu\n" +
			"      nested:\n" +
			"        count: 3\n")

		// --- When ---
		cfg, err := parseConfigFile(data)

		// --- Then ---
		assert.NoError(t, err)
		_, ok := cfg.Targets["github.com/acme/tasks"].(map[string]any)
		assert.True(t, ok)
	})
}

func Test_parseConfigFile_error_tabular(t *testing.T) {
	tt := []struct {
		testN string

		data []byte
		err  error
	}{
		{"missing version", []byte("settings:\n  tmp: /t\n"), errCfgVersion},
		{"zero version", []byte("version: 0\n"), errCfgVersion},
		{"negative version", []byte("version: -1\n"), errCfgVersion},
		{"version too new", []byte("version: 2\n"), errCfgVersionNew},
		{
			"unknown top-level key",
			[]byte("version: 1\nsetting: {}\n"),
			errCfgParse,
		},
		{
			"unknown settings key",
			[]byte("version: 1\nsettings:\n  timout: 1s\n"),
			errCfgParse,
		},
		{"invalid yaml", []byte("version: 1\n\t{"), errCfgParse},
		{"version not integer", []byte("version: abc\n"), errCfgParse},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			cfg, err := parseConfigFile(tc.data)

			// --- Then ---
			assert.ErrorIs(t, tc.err, err)
			assert.Nil(t, cfg)
		})
	}
}

func Test_targetImp_tabular(t *testing.T) {
	tt := []struct {
		testN string

		tgt      *mkf.Target
		localImp string
		want     string
	}{
		{
			"import spec wins",
			&mkf.Target{ImpSpec: "github.com/me/proj"},
			"local.com/mod",
			"github.com/me/proj",
		},
		{
			"empty import spec uses localImp",
			&mkf.Target{},
			"local.com/mod",
			"local.com/mod",
		},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have := targetImp(tc.tgt, tc.localImp)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}

func Test_namePath_tabular(t *testing.T) {
	tt := []struct {
		testN string

		name string
		want []string
	}{
		{"bare target", "build", []string{"build"}},
		{"namespaced", "go:test", []string{"go", "test"}},
		{"nested namespace", "go:lint:install",
			[]string{"go", "lint", "install"}},
		{"leading colon dropped", ":print", []string{"print"}},
		{"kebab preserved", "go:test-v", []string{"go", "test-v"}},
		{"empty name", "", []string{}},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have := namePath(tc.name)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}

func Test_localImportPath(t *testing.T) {
	t.Run("module root", func(t *testing.T) {
		// --- Given ---
		dir := t.TempDir()
		oskit.Write(t, "module github.com/foo/bar\n", dir, "go.mod")

		// --- When ---
		have := localImportPath(dir)

		// --- Then ---
		assert.Equal(t, "github.com/foo/bar", have)
	})

	t.Run("sub package", func(t *testing.T) {
		// --- Given ---
		dir := t.TempDir()
		oskit.Write(t, "module github.com/foo/bar\n", dir, "go.mod")
		sub := oskit.MkdirAll(t, dir, "build", "mk")

		// --- When ---
		have := localImportPath(sub)

		// --- Then ---
		assert.Equal(t, "github.com/foo/bar/build/mk", have)
	})

	t.Run("not in a module", func(t *testing.T) {
		// --- Given ---
		dir := t.TempDir()

		// --- When ---
		have := localImportPath(dir)

		// --- Then ---
		assert.Equal(t, "", have)
	})
}

func Test_moduleImportPath(t *testing.T) {
	t.Run("reads module directive", func(t *testing.T) {
		// --- Given ---
		dir := t.TempDir()
		oskit.Write(t, "module github.com/foo/bar\n\ngo 1.21\n", dir, "go.mod")

		// --- When ---
		have := moduleImportPath(dir)

		// --- Then ---
		assert.Equal(t, "github.com/foo/bar", have)
	})

	t.Run("walks up from a sub directory", func(t *testing.T) {
		// --- Given ---
		dir := t.TempDir()
		oskit.Write(t, "module github.com/foo/bar\n", dir, "go.mod")
		sub := oskit.MkdirAll(t, dir, "cmd", "app")

		// --- When ---
		have := moduleImportPath(sub)

		// --- Then ---
		assert.Equal(t, "github.com/foo/bar", have)
	})

	t.Run("no module returns empty", func(t *testing.T) {
		// --- Given ---
		dir := t.TempDir()

		// --- When ---
		have := moduleImportPath(dir)

		// --- Then ---
		assert.Equal(t, "", have)
	})
}

func Test_mergeConfigs(t *testing.T) {
	t.Run("project settings win, user settings fill gaps", func(t *testing.T) {
		// --- Given ---
		user := &fileConfig{Settings: &fileSettings{
			Timeout: new("10s"),
			Tmp:     new("/u/tmp"),
		}}
		project := &fileConfig{Settings: &fileSettings{Timeout: new("30s")}}

		// --- When ---
		have := mergeConfigs(user, project)

		// --- Then ---
		assert.Equal(t, "30s", *have.timeout)
		assert.Equal(t, "/u/tmp", *have.tmp)
	})

	t.Run("target trees are not merged into settings", func(t *testing.T) {
		// --- Given ---
		user := &fileConfig{Targets: map[string]any{"ext.com/a": nil}}
		project := &fileConfig{Targets: map[string]any{"ext.com/b": nil}}

		// --- When ---
		have := mergeConfigs(user, project)

		// --- Then ---
		assert.Nil(t, have.timeout)
		assert.Nil(t, have.tmp)
	})
}

func Test_impInModule_tabular(t *testing.T) {
	tt := []struct {
		testN string

		imp     string
		modPath string
		want    bool
	}{
		{"module root package", "me.com/mod", "me.com/mod", true},
		{"sub package", "me.com/mod/sub", "me.com/mod", true},
		{"external package", "ext.com/a", "me.com/mod", false},
		{"prefix but not sub", "me.com/mod2", "me.com/mod", false},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have := impInModule(tc.imp, tc.modPath)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}

func Test_knownNodes(t *testing.T) {
	t.Run("prefixes of same-import targets", func(t *testing.T) {
		// --- Given ---
		tgts := []*mkf.Target{
			{ImpSpec: "ext.com/a", Name: "go:test"},
			{ImpSpec: "ext.com/a", Name: "go:lint:install"},
			{ImpSpec: "ext.com/b", Name: "deploy"},
		}

		// --- When ---
		have := knownNodes(tgts, "me.com/mod", "ext.com/a")

		// --- Then ---
		want := map[string]bool{
			"go":              true,
			"go:test":         true,
			"go:lint":         true,
			"go:lint:install": true,
		}
		assert.Equal(t, want, have)
	})

	t.Run("local target matched via empty import spec", func(t *testing.T) {
		// --- Given ---
		tgts := []*mkf.Target{
			{Name: "build"},
			{ImpSpec: "ext.com/a", Name: "go:test"},
		}

		// --- When ---
		have := knownNodes(tgts, "me.com/mod", "me.com/mod")

		// --- Then ---
		assert.Equal(t, map[string]bool{"build": true}, have)
	})
}

func Test_settingKeys(t *testing.T) {
	t.Run("strips known child keys", func(t *testing.T) {
		// --- Given ---
		known := map[string]bool{"go:lint": true, "go:build": true}
		node := map[string]any{
			"timeout": "5m",
			"lint":    map[string]any{"version": "v1"},
			"build":   map[string]any{"modules": nil},
		}

		// --- When ---
		have := settingKeys([]string{"go"}, node, known)

		// --- Then ---
		assert.Equal(t, map[string]any{"timeout": "5m"}, have)
	})

	t.Run("root prefix keeps unknown keys", func(t *testing.T) {
		// --- Given ---
		known := map[string]bool{"go": true}
		node := map[string]any{"go": map[string]any{}, "foo": "bar"}

		// --- When ---
		have := settingKeys(nil, node, known)

		// --- Then ---
		assert.Equal(t, map[string]any{"foo": "bar"}, have)
	})
}

func Test_resolveTargetBlock_tabular(t *testing.T) {
	// Tree mirrors the plan's gmgo example: a "go" ns_root carrying a shared
	// timeout, a "lint" sub-namespace, and a "build" target with its own block.
	root := map[string]any{
		"go": map[string]any{
			"timeout": "5m",
			"lint": map[string]any{
				"version": "v2.13.0",
				"file":    ".golangci.yml",
			},
			"build": map[string]any{
				"modules": map[string]any{"github.com/acme/app": "x"},
			},
		},
	}
	known := map[string]bool{
		"go":              true,
		"go:test":         true,
		"go:test-v":       true,
		"go:check":        true,
		"go:vet":          true,
		"go:doc":          true,
		"go:lint":         true,
		"go:lint:install": true,
		"go:lint:config":  true,
		"go:build":        true,
	}

	lint := map[string]any{"version": "v2.13.0", "file": ".golangci.yml"}
	timeout := map[string]any{"timeout": "5m"}
	build := map[string]any{
		"modules": map[string]any{"github.com/acme/app": "x"},
	}

	tt := []struct {
		testN string

		path   []string
		want   map[string]any
		wantOK bool
	}{
		{"lint install", []string{"go", "lint", "install"}, lint, true},
		{"lint config", []string{"go", "lint", "config"}, lint, true},
		{"lint itself", []string{"go", "lint"}, lint, true},
		{"test up to go", []string{"go", "test"}, timeout, true},
		{"test-v up to go", []string{"go", "test-v"}, timeout, true},
		{"check up to go", []string{"go", "check"}, timeout, true},
		{"vet up to go", []string{"go", "vet"}, timeout, true},
		{"doc up to go", []string{"go", "doc"}, timeout, true},
		{"build own block", []string{"go", "build"}, build, true},
		{"unknown target no block", []string{"missing"}, nil, false},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have, ok := resolveTargetBlock(root, tc.path, known)

			// --- Then ---
			assert.Equal(t, tc.wantOK, ok)
			assert.Equal(t, tc.want, have)
		})
	}
}

func Test_resolveTargetBlock(t *testing.T) {
	t.Run("scalar where a descent was expected yields no block",
		func(t *testing.T) {
			// --- Given ---
			root := map[string]any{"go": map[string]any{"lint": "oops"}}
			known := map[string]bool{
				"go":              true,
				"go:lint":         true,
				"go:lint:install": true,
			}
			path := []string{"go", "lint", "install"}

			// --- When ---
			have, ok := resolveTargetBlock(root, path, known)

			// --- Then ---
			assert.False(t, ok)
			assert.Nil(t, have)
		})
}

func Test_resolveDelivered(t *testing.T) {
	extTgts := []*mkf.Target{
		{ImpSpec: "ext.com/a", Name: "go:test"},
		{ImpSpec: "ext.com/a", Name: "go:lint"},
	}

	t.Run("project block wins over user block", func(t *testing.T) {
		// --- Given ---
		project := map[string]any{"ext.com/a": map[string]any{
			"go": map[string]any{"timeout": "5m"},
		}}
		user := map[string]any{"ext.com/a": map[string]any{
			"go": map[string]any{"timeout": "9m"},
		}}
		tgt := &mkf.Target{ImpSpec: "ext.com/a", Name: "go:test"}

		// --- When ---
		have, ok := resolveDelivered(
			tgt, "me.com/mod", "me.com/mod", user, project, extTgts,
		)

		// --- Then ---
		assert.True(t, ok)
		assert.Equal(t, map[string]any{"timeout": "5m"}, have)
	})

	t.Run("user fallback for an external import", func(t *testing.T) {
		// --- Given ---
		project := map[string]any{}
		user := map[string]any{"ext.com/a": map[string]any{
			"go": map[string]any{"timeout": "9m"},
		}}
		tgt := &mkf.Target{ImpSpec: "ext.com/a", Name: "go:test"}

		// --- When ---
		have, ok := resolveDelivered(
			tgt, "me.com/mod", "me.com/mod", user, project, extTgts,
		)

		// --- Then ---
		assert.True(t, ok)
		assert.Equal(t, map[string]any{"timeout": "9m"}, have)
	})

	t.Run("in-module target skips the user fallback", func(t *testing.T) {
		// --- Given ---
		localTgts := []*mkf.Target{{Name: "build"}}
		project := map[string]any{}
		user := map[string]any{"me.com/mod": map[string]any{
			"build": map[string]any{"key": "val"},
		}}
		tgt := &mkf.Target{Name: "build"}

		// --- When ---
		have, ok := resolveDelivered(
			tgt, "me.com/mod", "me.com/mod", user, project, localTgts,
		)

		// --- Then ---
		assert.False(t, ok)
		assert.Nil(t, have)
	})

	t.Run("no block anywhere", func(t *testing.T) {
		// --- Given ---
		tgt := &mkf.Target{ImpSpec: "ext.com/a", Name: "go:test"}

		// --- When ---
		have, ok := resolveDelivered(
			tgt, "me.com/mod", "me.com/mod", nil, nil, extTgts,
		)

		// --- Then ---
		assert.False(t, ok)
		assert.Nil(t, have)
	})
}

func Test_pickSetting_tabular(t *testing.T) {
	tt := []struct {
		testN string

		user    *string
		project *string
		want    *string
	}{
		{"project wins", new("u"), new("p"), new("p")},
		{"user fills gap", new("u"), nil, new("u")},
		{"both nil", nil, nil, nil},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have := pickSetting(tc.user, tc.project)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}

func Test_fileConfig_timeout(t *testing.T) {
	t.Run("nil config", func(t *testing.T) {
		// --- Given ---
		var cfg *fileConfig

		// --- Then ---
		assert.Nil(t, cfg.timeout())
	})

	t.Run("nil settings", func(t *testing.T) {
		assert.Nil(t, (&fileConfig{}).timeout())
	})

	t.Run("value", func(t *testing.T) {
		// --- Given ---
		cfg := &fileConfig{Settings: &fileSettings{Timeout: new("5s")}}

		// --- When ---
		have := cfg.timeout()

		// --- Then ---
		assert.Equal(t, "5s", *have)
	})
}

func Test_fileConfig_tmp(t *testing.T) {
	t.Run("nil config", func(t *testing.T) {
		// --- Given ---
		var cfg *fileConfig

		// --- Then ---
		assert.Nil(t, cfg.tmp())
	})

	t.Run("nil settings", func(t *testing.T) {
		assert.Nil(t, (&fileConfig{}).tmp())
	})

	t.Run("value", func(t *testing.T) {
		// --- Given ---
		cfg := &fileConfig{Settings: &fileSettings{Tmp: new("/t")}}

		// --- When ---
		have := cfg.tmp()

		// --- Then ---
		assert.Equal(t, "/t", *have)
	})
}

func Test_config_applyFileConfig(t *testing.T) {
	t.Run("applies settings and the project target tree", func(t *testing.T) {
		// --- Given ---
		dir := t.TempDir()
		content := "version: 1\n" +
			"settings:\n" +
			"  timeout: 45s\n" +
			"  tmp: /abs/from/settings\n" +
			"targets:\n" +
			"  ext.com/a:\n" +
			"    run:\n" +
			"      key: val\n"
		oskit.Write(t, content, dir, configFileName)
		cfg := &config{
			src: dir,
			fs:  xflag.NewFlagSet("t", flag.ContinueOnError),
		}

		// --- When ---
		err := cfg.applyFileConfig(nil)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, 45*time.Second, cfg.timeout)
		assert.Equal(t, "/abs/from/settings", cfg.tmp)
		_, ok := cfg.projectTargets["ext.com/a"].(map[string]any)
		assert.True(t, ok)
	})

	t.Run("environment tmp beats settings tmp", func(t *testing.T) {
		// --- Given ---
		dir := t.TempDir()
		content := "version: 1\nsettings:\n  tmp: /from/settings\n"
		oskit.Write(t, content, dir, configFileName)
		cfg := &config{
			src: dir,
			fs:  xflag.NewFlagSet("t", flag.ContinueOnError),
		}
		env := []string{envKeyTmpDir + "=/from/env"}

		// --- When ---
		err := cfg.applyFileConfig(env)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "/from/env", cfg.tmp)
	})

	t.Run("option timeout beats settings timeout", func(t *testing.T) {
		// --- Given ---
		dir := t.TempDir()
		oskit.Write(t, "version: 1\nsettings:\n  timeout: 45s\n",
			dir, configFileName)
		fs := xflag.NewFlagSet("t", flag.ContinueOnError)
		cfg := &config{src: dir, fs: fs}
		fs.DurationVar(&cfg.timeout, "timeout", 0, "")
		assert.NoError(t, fs.Parse([]string{"--timeout", "5s"}))

		// --- When ---
		err := cfg.applyFileConfig(nil)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, 5*time.Second, cfg.timeout)
	})

	t.Run("error - invalid settings timeout", func(t *testing.T) {
		// --- Given ---
		dir := t.TempDir()
		oskit.Write(t, "version: 1\nsettings:\n  timeout: nope\n",
			dir, configFileName)
		cfg := &config{
			src: dir,
			fs:  xflag.NewFlagSet("t", flag.ContinueOnError),
		}

		// --- When ---
		err := cfg.applyFileConfig(nil)

		// --- Then ---
		assert.ErrorIs(t, errCfgParse, err)
	})
}

func Test_deliverTargetConfig(t *testing.T) {
	t.Run("delivers nearest-level block to meta store", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()
		tgt := &mkf.Target{ImpSpec: "ext.com/a", Name: "run"}
		cfg := &config{projectTargets: map[string]any{
			"ext.com/a": map[string]any{
				"run": map[string]any{"region": "eu"},
			},
		}}

		// --- When ---
		err := deliverTargetConfig(rng, cfg, tgt, []*mkf.Target{tgt})

		// --- Then ---
		assert.NoError(t, err)
		meta, ok := rng.MetaLookup(gomake.ConfigMetaKey)
		assert.True(t, ok)
		assert.Equal(t, `{"region":"eu"}`, meta)
		_, hasEnv := rng.EnvLookup(targetConfigArg)
		assert.False(t, hasEnv)
	})

	t.Run("no configuration is a no-op", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()
		cfg := &config{}
		tgt := &mkf.Target{ImpSpec: "ext.com/a", Name: "run"}

		// --- When ---
		err := deliverTargetConfig(rng, cfg, tgt, []*mkf.Target{tgt})

		// --- Then ---
		assert.NoError(t, err)
		_, ok := rng.MetaLookup(gomake.ConfigMetaKey)
		assert.False(t, ok)
	})

	t.Run("target without a config entry is a no-op", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()
		tgt := &mkf.Target{ImpSpec: "ext.com/a", Name: "run"}
		cfg := &config{projectTargets: map[string]any{
			"ext.com/b": map[string]any{
				"other": map[string]any{"x": 1},
			},
		}}

		// --- When ---
		err := deliverTargetConfig(rng, cfg, tgt, []*mkf.Target{tgt})

		// --- Then ---
		assert.NoError(t, err)
		_, ok := rng.MetaLookup(gomake.ConfigMetaKey)
		assert.False(t, ok)
	})

	t.Run("nil target is a no-op", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()
		cfg := &config{projectTargets: map[string]any{
			"ext.com/a": map[string]any{
				"run": map[string]any{"region": "eu"},
			},
		}}

		// --- When ---
		err := deliverTargetConfig(rng, cfg, nil, nil)

		// --- Then ---
		assert.NoError(t, err)
		_, ok := rng.MetaLookup(gomake.ConfigMetaKey)
		assert.False(t, ok)
	})
}

func Test_invokedTarget(t *testing.T) {
	t.Run("resolves the named target", func(t *testing.T) {
		// --- Given ---
		build := &mkf.Target{Name: "build"}
		tgs := must.Value(parser.TargetsFromList(build))
		cfg := &config{target: "build"}

		// --- When ---
		have := invokedTarget(cfg, tgs)

		// --- Then ---
		assert.Same(t, build, have)
	})

	t.Run("resolves the default target when no name given", func(t *testing.T) {
		// --- Given ---
		def := &mkf.Target{Name: "deploy", Default: true}
		tgs := must.Value(parser.TargetsFromList(def))
		cfg := &config{target: ""}

		// --- When ---
		have := invokedTarget(cfg, tgs)

		// --- Then ---
		assert.Same(t, def, have)
	})

	t.Run("returns nil for an unknown target", func(t *testing.T) {
		// --- Given ---
		tgs := must.Value(parser.TargetsFromList(&mkf.Target{Name: "build"}))
		cfg := &config{target: "nope"}

		// --- When ---
		have := invokedTarget(cfg, tgs)

		// --- Then ---
		assert.Nil(t, have)
	})
}

func Test_runCheckConfig(t *testing.T) {
	// --- Given ---
	tst := ringtest.New(t).WetStderr()

	absPath := modkit.Path("testdata/projects/config_target/project")
	prj := gmt.NewProject(t)
	prj.GoModInit()
	prj.MakefilesFrom(absPath)
	prj.UseGomakeSrc(modkit.Root())
	prj.GoModTidy()
	prj.Close()

	yaml := "version: 1\n" +
		"targets:\n" +
		"  " + prjkit.GoModName + ":\n" +
		"    show:\n" +
		"      message: hi\n" +
		"  bogus.com/x:\n" +
		"    gone:\n" +
		"      k: v\n"
	oskit.Write(t, yaml, prj.Root(), configFileName)

	rng := tst.Ring("--src", prj.Root(), "--tmp", prj.TempDir())
	cfg, err := newConfig("1.2.3", rng)
	assert.NoError(t, err)

	// --- When ---
	err = runCheckConfig(rng, cfg, builtintest.NewTstProvider().Targets())

	// --- Then ---
	assert.NoError(t, err)
	out := tst.Stderr()
	assert.Contain(t, prjkit.GoModName+":", out)
	assert.Contain(t, "show", out)
	assert.Contain(t, "project import path has no target: bogus.com/x", out)
}

func Test_checkConfigReport(t *testing.T) {
	t.Run("no problems lists targets grouped by import", func(t *testing.T) {
		// --- Given ---
		tgts := []*mkf.Target{
			{ImpSpec: "ext.com/z", Name: "run"},
			{ImpSpec: "ext.com/a", Name: "go:build"},
		}
		user := &fileConfig{}
		project := &fileConfig{}

		// --- When ---
		have := checkConfigReport(tgts, "local.com/m", user, project)

		// --- Then ---
		assert.Contain(t, "no problems found", have)
		aIdx := strings.Index(have, "ext.com/a:")
		zIdx := strings.Index(have, "ext.com/z:")
		assert.True(t, aIdx > 0 && zIdx > aIdx)
		assert.Contain(t, "    go:build", have)
	})

	t.Run("reports problems then lists targets", func(t *testing.T) {
		// --- Given ---
		tgts := []*mkf.Target{{ImpSpec: "ext.com/a", Name: "build"}}
		user := &fileConfig{}
		project := &fileConfig{Targets: map[string]any{
			"ext.com/gone": map[string]any{"x": map[string]any{"k": "v"}},
		}}

		// --- When ---
		have := checkConfigReport(tgts, "local.com/m", user, project)

		// --- Then ---
		want := "project import path has no target: ext.com/gone"
		assert.Contain(t, "problems found", have)
		assert.Contain(t, want, have)
		assert.Contain(t, "ext.com/a:", have)
	})
}

func Test_checkConfigProblems(t *testing.T) {
	t.Run("no problems", func(t *testing.T) {
		// --- Given ---
		valid := map[string]map[string]bool{"ext.com/a": {"build": true}}
		user := &fileConfig{Settings: &fileSettings{Tmp: new("/abs")}}
		project := &fileConfig{Targets: map[string]any{
			"ext.com/a": map[string]any{"build": nil},
		}}

		// --- When ---
		have := checkConfigProblems(valid, user, project)

		// --- Then ---
		assert.Equal(t, 0, len(have))
	})

	t.Run("non-absolute tmp in both files", func(t *testing.T) {
		// --- Given ---
		valid := map[string]map[string]bool{}
		user := &fileConfig{Settings: &fileSettings{Tmp: new("rel/u")}}
		project := &fileConfig{Settings: &fileSettings{Tmp: new("rel/p")}}

		// --- When ---
		have := checkConfigProblems(valid, user, project)

		// --- Then ---
		assert.Equal(t, 2, len(have))
		assert.Contain(t, "user", have[0])
		assert.Contain(t, "project", have[1])
	})

	t.Run("unmatched project import path", func(t *testing.T) {
		// --- Given ---
		valid := map[string]map[string]bool{"ext.com/a": {"build": true}}
		user := &fileConfig{}
		project := &fileConfig{Targets: map[string]any{
			"ext.com/a":   map[string]any{"build": nil},
			"ext.com/old": map[string]any{"gone": nil},
		}}

		// --- When ---
		have := checkConfigProblems(valid, user, project)

		// --- Then ---
		assert.Equal(t, 1, len(have))
		assert.Contain(t, "ext.com/old", have[0])
	})

	t.Run("project import config is not a mapping", func(t *testing.T) {
		// --- Given ---
		valid := map[string]map[string]bool{}
		user := &fileConfig{}
		project := &fileConfig{Targets: map[string]any{
			"ext.com/a": "oops",
		}}

		// --- When ---
		have := checkConfigProblems(valid, user, project)

		// --- Then ---
		assert.Equal(t, 1, len(have))
		assert.Contain(t, "project config is not a mapping: ext.com/a", have[0])
	})
}
