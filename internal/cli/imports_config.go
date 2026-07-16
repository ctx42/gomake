// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
)

// TargetsFile is the YAML config file name for external target imports. It
// lives at the gomake source root and is committed to the repo.
const TargetsFile = "targets.yaml"

// fetchTimeout bounds an HTTP fetch of an external targets file.
const fetchTimeout = 10 * time.Second

// External targets config errors.
var (
	// errDupImportPath indicates a duplicate import path in config.
	errDupImportPath = errors.New("duplicate import path")

	// errInvConfig indicates invalid YAML or schema in the config file.
	errInvConfig = errors.New("invalid external targets config")
)

// ImportEntry is one import from [TargetsFile].
type ImportEntry struct {
	Path      string
	Namespace string
	Config    json.RawMessage
}

// MetaKey returns the ring meta-key for the entry. When Namespace is set, it's
// used as the key; otherwise, the last path segment of the import path
// (without any version suffix) is returned.
func (ent ImportEntry) MetaKey() string {
	if ent.Namespace != "" {
		return ent.Namespace
	}
	p := ent.Path
	if i := strings.Index(p, "@"); i >= 0 {
		p = p[:i]
	}
	base := path.Base(p)
	// A major-version path suffix ("/v2", "/v3", ...) is not the package name;
	// it lives in the preceding segment.
	if isMajorVersion(base) {
		base = path.Base(path.Dir(p))
	}
	return base
}

// isMajorVersion reports whether seg is a module major-version path segment
// such as "v2" or "v3".
func isMajorVersion(seg string) bool {
	if len(seg) < 2 || seg[0] != 'v' {
		return false
	}
	for _, r := range seg[1:] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// ImportsConfig is the external targets import list.
type ImportsConfig struct {
	imports []ImportEntry
	raw     []byte
}

// Imports returns a copy of the parsed import entries.
func (cfg *ImportsConfig) Imports() []ImportEntry {
	return append([]ImportEntry(nil), cfg.imports...)
}

// Raw returns a copy of the raw YAML bytes that were read to produce this
// config. It is nil when the config was not loaded from a file or URL.
func (cfg *ImportsConfig) Raw() []byte {
	if cfg.raw == nil {
		return nil
	}
	return append([]byte(nil), cfg.raw...)
}

// Paths returns the import path string for each entry.
func (cfg *ImportsConfig) Paths() []string {
	specs := make([]string, 0, len(cfg.imports))
	for _, ent := range cfg.imports {
		specs = append(specs, ent.Path)
	}
	return specs
}

// importLines returns one announcement line per import in the config, each of
// the form "adding external target <path>". The result is empty when the
// config has no imports.
func (cfg *ImportsConfig) importLines() []string {
	lines := make([]string, 0, len(cfg.imports))
	for _, ent := range cfg.imports {
		lines = append(lines, "adding external target "+ent.Path)
	}
	return lines
}

// LoadExternalTargets loads a targets.yaml from pathOrURL. If pathOrURL begins
// with "http://" or "https://" the file is fetched over HTTP; otherwise it is
// read from the local filesystem. A missing local file returns an empty config
// without error. The context cancels in-flight HTTP fetches; local reads ignore
// it except for a pre-check of ctx.Err().
func LoadExternalTargets(ctx context.Context, tgs string) (*ImportsConfig, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	isURL := strings.HasPrefix(tgs, "http://") ||
		strings.HasPrefix(tgs, "https://")
	if isURL {
		return fetchExternalTargets(ctx, tgs)
	}
	cfg, err := readExternalTargets(tgs)
	if errors.Is(err, os.ErrNotExist) {
		return &ImportsConfig{}, nil
	}
	return cfg, err
}

// fetchExternalTargets performs an HTTP GET for url, parses the response body
// as targets.yaml, and returns the config with Raw set to the response body.
// The GET is bounded by the shorter of [fetchTimeout] and ctx.
func fetchExternalTargets(
	ctx context.Context,
	url string,
) (*ImportsConfig, error) {

	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d fetching %s", resp.StatusCode, url)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	cfg, err := parseExternalTargets(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", url, err)
	}
	cfg.raw = data
	return cfg, nil
}

// readExternalTargets reads the file at pth, parses it as targets.yaml, and
// returns the config with Raw set to the file content.
func readExternalTargets(pth string) (*ImportsConfig, error) {
	data, err := os.ReadFile(pth)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", pth, err)
	}
	cfg, err := parseExternalTargets(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", pth, err)
	}
	cfg.raw = data
	return cfg, nil
}

// parseExternalTargets decodes the YAML content of a targets.yaml file and
// returns the import config. It rejects unknown fields and invalid YAML with
// [errInvConfig] and duplicate import paths with [errDupImportPath].
func parseExternalTargets(data []byte) (*ImportsConfig, error) {
	imports, err := decodeTargetsYAML(data)
	if err != nil {
		return nil, err
	}
	if err = validateNoDupPaths(imports); err != nil {
		return nil, err
	}
	return &ImportsConfig{imports: imports}, nil
}

// decodeTargetsYAML decodes the YAML content of targets.yaml and returns the
// import entries. Each entry must have a non-empty "import" field. Unknown
// fields and invalid YAML are rejected with [errInvConfig].
func decodeTargetsYAML(data []byte) ([]ImportEntry, error) {
	var raw struct {
		Imports []struct {
			Path      string `yaml:"import"`
			Namespace string `yaml:"namespace,omitempty"`
			Config    any    `yaml:"config,omitempty"`
		} `yaml:"imports"`
	}
	dec := yaml.NewDecoder(bytes.NewReader(data), yaml.Strict())
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("%w: %w", errInvConfig, err)
	}
	entries := make([]ImportEntry, 0, len(raw.Imports))
	for i, item := range raw.Imports {
		if item.Path == "" {
			format := "%w: imports[%d]: import is empty"
			return nil, fmt.Errorf(format, errInvConfig, i)
		}
		ent := ImportEntry{Path: item.Path, Namespace: item.Namespace}
		if item.Config != nil {
			jsonBytes, err := json.Marshal(item.Config)
			if err != nil {
				format := "%w: imports[%d]: config: %w"
				return nil, fmt.Errorf(format, errInvConfig, i, err)
			}
			ent.Config = jsonBytes
		}
		entries = append(entries, ent)
	}
	return entries, nil
}

// validateNoDupPaths returns [errDupImportPath] when any path string appears
// more than once in imports.
func validateNoDupPaths(imports []ImportEntry) error {
	seen := make(map[string]struct{}, len(imports))
	for _, ent := range imports {
		if _, dup := seen[ent.Path]; dup {
			return fmt.Errorf("%w: %s", errDupImportPath, ent.Path)
		}
		seen[ent.Path] = struct{}{}
	}
	return nil
}
