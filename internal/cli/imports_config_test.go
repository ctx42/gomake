// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package cli

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/ctx42/ring/pkg/ring"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testkit/pkg/httpkit"
	"github.com/ctx42/testkit/pkg/oskit"
)

func Test_ImportEntry_MetaKey_tabular(t *testing.T) {
	tt := []struct {
		testN string

		entry ImportEntry
		want  string
	}{
		{
			"namespace takes priority",
			ImportEntry{Path: "a.com/pkg/gmbump", Namespace: "mygmbump"},
			"mygmbump",
		},
		{
			"last path segment",
			ImportEntry{Path: "a.com/pkg/gmbump"},
			"gmbump",
		},
		{
			"version suffix stripped",
			ImportEntry{Path: "a.com/pkg/gmbump@v1.2.3"},
			"gmbump",
		},
		{
			"major version path suffix ignored",
			ImportEntry{Path: "a.com/pkg/gmbump/v2"},
			"gmbump",
		},
		{
			"major version path suffix with query ignored",
			ImportEntry{Path: "a.com/pkg/gmbump/v2@v2.1.0"},
			"gmbump",
		},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have := tc.entry.MetaKey()

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}

func Test_isMajorVersion_tabular(t *testing.T) {
	tt := []struct {
		testN string

		seg  string
		want bool
	}{
		{"empty string", "", false},
		{"only v", "v", false},
		{"not v prefix", "abc", false},
		{"single digit", "v1", true},
		{"multi digit", "v12", true},
		{"non-digit in suffix", "v1a", false},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have := isMajorVersion(tc.seg)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}

func Test_ImportsConfig_Imports(t *testing.T) {
	t.Run("nil slice returns empty slice", func(t *testing.T) {
		// --- Given ---
		cfg := &ImportsConfig{}

		// --- When ---
		have := cfg.Imports()

		// --- Then ---
		assert.Equal(t, 0, len(have))
	})

	t.Run("returns copy of entries", func(t *testing.T) {
		// --- Given ---
		cfg := &ImportsConfig{
			imports: []ImportEntry{{Path: "a.com/x"}, {Path: "b.com/y"}},
		}

		// --- When ---
		have := cfg.Imports()

		// --- Then ---
		assert.Equal(t, cfg.imports, have)
	})
}

func Test_ImportsConfig_Raw(t *testing.T) {
	t.Run("nil raw returns nil", func(t *testing.T) {
		// --- Given ---
		cfg := &ImportsConfig{}

		// --- When ---
		have := cfg.Raw()

		// --- Then ---
		assert.Nil(t, have)
	})

	t.Run("returns copy of raw bytes", func(t *testing.T) {
		// --- Given ---
		cfg := &ImportsConfig{raw: []byte("imports: []\n")}

		// --- When ---
		have := cfg.Raw()

		// --- Then ---
		assert.Equal(t, cfg.raw, have)
	})
}

func Test_ImportsConfig_Paths_tabular(t *testing.T) {
	tt := []struct {
		testN string

		imports []ImportEntry
		want    []string
	}{
		{
			"with entries",
			[]ImportEntry{
				{Path: "a.com/x"},
				{Path: "b.com/y", Namespace: "ns"},
			},
			[]string{"a.com/x", "b.com/y"},
		},
		{
			"empty",
			nil,
			[]string{},
		},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- Given ---
			cfg := &ImportsConfig{imports: tc.imports}

			// --- When ---
			have := cfg.Paths()

			// --- Then ---
			if len(tc.want) == 0 {
				assert.Equal(t, 0, len(have))
				return
			}
			assert.Equal(t, tc.want, have)
		})
	}
}

func Test_ImportsConfig_importLines(t *testing.T) {
	t.Run("no imports yields no lines", func(t *testing.T) {
		// --- Given ---
		cfg := &ImportsConfig{}

		// --- When ---
		have := cfg.importLines()

		// --- Then ---
		assert.Equal(t, 0, len(have))
	})

	t.Run("one line per import", func(t *testing.T) {
		// --- Given ---
		cfg := &ImportsConfig{imports: []ImportEntry{
			{Path: "example.com/foo"},
			{Path: "example.com/bar"},
		}}

		// --- When ---
		have := cfg.importLines()

		// --- Then ---
		want := []string{
			"adding external target example.com/foo",
			"adding external target example.com/bar",
		}
		assert.Equal(t, want, have)
	})
}

func Test_LoadExternalTargets_tabular(t *testing.T) {
	tt := []struct {
		testN string

		// content is the file content; empty string means the file is absent.
		content string
		want    []ImportEntry
		err     error
	}{
		{
			"absent file",
			"",
			[]ImportEntry{},
			nil,
		},
		{
			"empty imports array",
			"imports: []\n",
			[]ImportEntry{},
			nil,
		},
		{
			"object with namespace",
			`imports:
  - import: a.com/pkg
    namespace: ns
  - import: b.com/x
`,
			[]ImportEntry{
				{Path: "a.com/pkg", Namespace: "ns"},
				{Path: "b.com/x"},
			},
			nil,
		},
		{
			"version suffix in path",
			"imports:\n  - import: a.com/pkg@v1.2.3\n",
			[]ImportEntry{{Path: "a.com/pkg@v1.2.3"}},
			nil,
		},
		{
			"duplicate path",
			`imports:
  - import: dup.com/x
  - import: dup.com/x
`,
			nil,
			errDupImportPath,
		},
		{
			"unknown top-level field",
			"imports: []\nextra: 1\n",
			nil,
			errInvConfig,
		},
		{
			"unknown import object field",
			`imports:
  - import: a.com
    extra: 1
`,
			nil,
			errInvConfig,
		},
		{
			"empty path string",
			"imports:\n  - \"\"\n",
			nil,
			errInvConfig,
		},
		{
			"empty path object",
			"imports:\n  - import: \"\"\n",
			nil,
			errInvConfig,
		},
		{
			"invalid yaml",
			"{",
			nil,
			errInvConfig,
		},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- Given ---
			dir := t.TempDir()
			if tc.content != "" {
				oskit.Write(t, tc.content, dir, TargetsFile)
			}

			ctx := t.Context()

			rng := ring.New()

			join := filepath.Join(dir, TargetsFile)

			// --- When ---
			have, err := LoadExternalTargets(
				ctx,
				rng,
				join,
			)

			// --- Then ---
			if tc.err != nil {
				assert.ErrorIs(t, tc.err, err)
				return
			}
			assert.NoError(t, err)
			if len(tc.want) == 0 {
				assert.Equal(t, 0, len(have.imports))
				return
			}
			assert.Equal(t, tc.want, have.imports)
		})
	}
}

func Test_LoadExternalTargets(t *testing.T) {
	t.Run("fetches valid YAML from HTTP URL", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()

		body := "imports:\n  - import: a.com/x\n"

		fn := func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(body))
		}

		srv := httpkit.HandleFunc(t, "/", fn).Start(ctx)

		rng := ring.New()

		// --- When ---
		have, err := LoadExternalTargets(
			ctx,
			rng,
			srv.URL,
		)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, []string{"a.com/x"}, have.Paths())
		assert.Equal(t, []byte(body), have.Raw())
	})

	t.Run("expands a tilde from the ring", func(t *testing.T) {
		// --- Given ---
		home := t.TempDir()

		body := "imports:\n  - import: a.com/x\n"
		oskit.Write(t, body, home, "targets.yaml")

		rng := ring.New()
		rng.EnvSet("HOME", home)
		rng.EnvSet("USERPROFILE", home)
		rng.EnvSet("home", home)

		context := t.Context()

		path := "~/targets.yaml"

		// --- When ---
		have, err := LoadExternalTargets(context, rng, path)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, []string{"a.com/x"}, have.Paths())
	})

	t.Run("error - home unset", func(t *testing.T) {
		// --- Given ---
		rng := ring.New()
		rng.EnvSet("HOME", "")
		rng.EnvSet("USERPROFILE", "")
		rng.EnvSet("home", "")

		context := t.Context()

		path := "~/targets.yaml"

		// --- When ---
		_, err := LoadExternalTargets(context, rng, path)

		// --- Then ---
		assert.ErrorContain(t, "is not defined", err)
	})
}

func Test_fetchExternalTargets(t *testing.T) {
	t.Run("returns parsed config and raw bytes on 200", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()

		body := "imports:\n  - import: a.com/x\n"

		fn := func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(body))
		}

		srv := httpkit.HandleFunc(t, "/", fn).Start(ctx)

		// --- When ---
		have, err := fetchExternalTargets(ctx, srv.URL)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, []string{"a.com/x"}, have.Paths())
		assert.Equal(t, []byte(body), have.Raw())
	})

	t.Run("returns error on non-200 status", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()

		fn := func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}

		srv := httpkit.HandleFunc(t, "/", fn).Start(ctx)

		// --- When ---
		_, err := fetchExternalTargets(ctx, srv.URL)

		// --- Then ---
		assert.ErrorContain(t, "HTTP 404", err)
	})

	t.Run("returns error on unreachable address", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()

		path := "http://localhost:0/x"

		// --- When ---
		_, err := fetchExternalTargets(ctx, path)

		// --- Then ---
		assert.ErrorContain(t, "connection refused", err)
	})

	t.Run("returns error for malformed URL", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()

		path := "http://\x00invalid"

		// --- When ---
		_, err := fetchExternalTargets(ctx, path)

		// --- Then ---
		assert.ErrorContain(t, "invalid control character", err)
	})

	t.Run("returns error for invalid YAML body", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()

		fn := func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("{bad yaml"))
		}

		srv := httpkit.HandleFunc(t, "/", fn).Start(ctx)

		// --- When ---
		_, err := fetchExternalTargets(ctx, srv.URL)

		// --- Then ---
		assert.ErrorIs(t, errInvConfig, err)
	})
}

func Test_readExternalTargets_tabular(t *testing.T) {
	tt := []struct {
		testN string

		// content is the file content; empty string means the file is absent.
		content string
		want    []string
		err     error
	}{
		{
			"reads valid file and sets raw",
			"imports:\n  - import: a.com/x\n",
			[]string{"a.com/x"},
			nil,
		},
		{
			"missing file returns ErrNotExist",
			"",
			nil,
			os.ErrNotExist,
		},
		{
			"invalid YAML returns errInvConfig",
			"{",
			nil,
			errInvConfig,
		},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- Given ---
			dir := t.TempDir()

			pth := filepath.Join(dir, TargetsFile)
			if tc.content != "" {
				oskit.Write(t, tc.content, pth)
			}

			// --- When ---
			have, err := readExternalTargets(pth)

			// --- Then ---
			if tc.err != nil {
				assert.ErrorIs(t, tc.err, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.want, have.Paths())
			assert.Equal(t, []byte(tc.content), have.Raw())
		})
	}
}

func Test_parseExternalTargets_tabular(t *testing.T) {
	tt := []struct {
		testN string

		data string
		want []ImportEntry
		err  error
	}{
		{
			"objects with namespace",
			"imports:\n  - import: a.com/pkg\n    namespace: ns\n",
			[]ImportEntry{{Path: "a.com/pkg", Namespace: "ns"}},
			nil,
		},
		{
			"version suffix in path",
			"imports:\n  - import: a.com/pkg@v1.2.3\n",
			[]ImportEntry{{Path: "a.com/pkg@v1.2.3"}},
			nil,
		},
		{
			"duplicate path",
			`imports:
  - import: dup.com/x
  - import: dup.com/x
`,
			nil,
			errDupImportPath,
		},
		{
			"unknown top-level field",
			"imports: []\nextra: 1\n",
			nil,
			errInvConfig,
		},
		{
			"unknown import object field",
			"imports:\n  - import: a.com\n    extra: 1\n",
			nil,
			errInvConfig,
		},
		{
			"empty path string",
			"imports:\n  - \"\"\n",
			nil,
			errInvConfig,
		},
		{
			"empty path object",
			"imports:\n  - import: \"\"\n",
			nil,
			errInvConfig,
		},
		{
			"invalid yaml",
			"{",
			nil,
			errInvConfig,
		},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- Given ---
			val := []byte(tc.data)

			// --- When ---
			have, err := parseExternalTargets(val)

			// --- Then ---
			if tc.err != nil {
				assert.ErrorIs(t, tc.err, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.want, have.imports)
		})
	}
}

func Test_decodeTargetsYAML_tabular(t *testing.T) {
	tt := []struct {
		testN string

		data string
		want []ImportEntry
		err  error
	}{
		{
			"objects with namespace",
			"imports:\n  - import: a.com/pkg\n    namespace: ns\n",
			[]ImportEntry{{Path: "a.com/pkg", Namespace: "ns"}},
			nil,
		},
		{
			"version suffix in path",
			"imports:\n  - import: a.com/pkg@v1.2.3\n",
			[]ImportEntry{{Path: "a.com/pkg@v1.2.3"}},
			nil,
		},
		{
			"unknown top-level field",
			"imports: []\nextra: 1\n",
			nil,
			errInvConfig,
		},
		{
			"unknown import object field",
			"imports:\n  - import: a.com\n    extra: 1\n",
			nil,
			errInvConfig,
		},
		{
			"empty path string",
			"imports:\n  - \"\"\n",
			nil,
			errInvConfig,
		},
		{
			"empty path object",
			"imports:\n  - import: \"\"\n",
			nil,
			errInvConfig,
		},
		{
			"invalid yaml",
			"{",
			nil,
			errInvConfig,
		},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- Given ---
			val := []byte(tc.data)

			// --- When ---
			have, err := decodeTargetsYAML(val)

			// --- Then ---
			if tc.err != nil {
				assert.ErrorIs(t, tc.err, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.want, have)
		})
	}
}

func Test_decodeTargetsYAML_config(t *testing.T) {
	t.Run("object with config", func(t *testing.T) {
		// --- Given ---
		data := `imports:
  - import: a.com/pkg
    config:
      key: val
`

		val := []byte(data)

		// --- When ---
		have, err := decodeTargetsYAML(val)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, 1, len(have))
		assert.Equal(t, "a.com/pkg", have[0].Path)
		assert.Equal(t, json.RawMessage(`{"key":"val"}`), have[0].Config)
	})

	t.Run("object without config", func(t *testing.T) {
		// --- Given ---
		data := "imports:\n  - import: a.com/pkg\n"

		val := []byte(data)

		// --- When ---
		have, err := decodeTargetsYAML(val)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, 1, len(have))
		assert.Nil(t, have[0].Config)
	})
}

func Test_validateNoDupPaths_tabular(t *testing.T) {
	tt := []struct {
		testN string

		imports []ImportEntry
		err     error
	}{
		{"empty slice", nil, nil},
		{
			"unique paths",
			[]ImportEntry{{Path: "a.com/x"}, {Path: "b.com/y"}},
			nil,
		},
		{
			"duplicate path",
			[]ImportEntry{{Path: "a.com/x"}, {Path: "a.com/x"}},
			errDupImportPath,
		},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			err := validateNoDupPaths(tc.imports)

			// --- Then ---
			if tc.err != nil {
				assert.ErrorIs(t, tc.err, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}
