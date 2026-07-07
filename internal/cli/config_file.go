// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ctx42/ring/pkg/ring"
	"github.com/goccy/go-yaml"

	"github.com/ctx42/gomake/internal/mkf"
	"github.com/ctx42/gomake/internal/parser"
	"github.com/ctx42/gomake/pkg/gomake"
)

// This file implements gomake's two-level YAML configuration. A user-level and
// a project-level gomake.yaml are loaded, merged (project wins), and split into
// gomake's own settings and per-target configuration blocks. The invoked
// target's block is delivered to it through the ring meta store;
// "--check-config" lints the files and lists every target's canonical key.

// configFileName is the name of gomake's YAML configuration file. Both the
// user-level and the project-level configurations use this same name.
const configFileName = "gomake.yaml"

// configSchemaVersion is the highest configuration schema version this binary
// understands.
const configSchemaVersion = 1

// Environment keys used to resolve the user-level configuration file location.
const (
	// envKeyXDGConfigHome is the XDG base-directory variable for user config.
	envKeyXDGConfigHome = "XDG_CONFIG_HOME"

	// envKeyHome is the current user's home directory variable.
	envKeyHome = "HOME"
)

// targetConfigArg is the internal command-line argument that carries the
// invoked target's JSON configuration block to a compiled makefile subprocess.
// The generated main strips it and loads its value into the ring meta store
// under [gomake.ConfigMetaKey]; it never reaches the target as an argument. The
// environment is deliberately not used: a target's configuration originates
// only from the ring meta store, leaving the environment for the target's own
// optional override logic.
const targetConfigArg = "--gomake-config"

// Configuration file errors.
var (
	// errCfgParse indicates invalid YAML or an unknown key on the
	// gomake-owned surface of a gomake.yaml file.
	errCfgParse = errors.New("invalid gomake.yaml")

	// errCfgVersion indicates a missing or non-positive schema version.
	errCfgVersion = errors.New(
		"gomake.yaml: version must be a positive integer",
	)

	// errCfgVersionNew indicates a schema version newer than this binary
	// supports.
	errCfgVersionNew = errors.New("gomake.yaml: schema version not supported")
)

// fileSettings is the "settings" section of a gomake.yaml file. It configures
// gomake itself. A nil field means the key was absent.
type fileSettings struct {
	// Timeout is the default target execution timeout as a Go duration string.
	Timeout *string `yaml:"timeout"`

	// Tmp is the default temporary directory.
	Tmp *string `yaml:"tmp"`
}

// fileConfig is the parsed content of a single gomake.yaml file. The
// gomake-owned surface (version and settings) is parsed strictly; each target
// block is kept as an opaque value that gomake does not validate.
type fileConfig struct {
	// Version is the configuration schema version.
	Version *int `yaml:"version"`

	// Settings is gomake's own configuration.
	Settings *fileSettings `yaml:"settings"`

	// Targets maps a canonical target key to its opaque configuration block.
	Targets map[string]any `yaml:"targets"`
}

// mergedConfig is the effective gomake.yaml configuration after merging the
// user-level and project-level files.
type mergedConfig struct {
	// timeout is the effective settings.timeout, or nil when unset.
	timeout *string

	// tmp is the effective settings.tmp, or nil when unset.
	tmp *string

	// targets maps a canonical target key to its opaque configuration block.
	targets map[string]any
}

// userConfigPath returns the path to the user-level gomake.yaml. It honors
// XDG_CONFIG_HOME and falls back to $HOME/.config. The boolean is false when
// neither variable is set, in which case there is no user-level file.
func userConfigPath(env []string) (string, bool) {
	base, ok := gomake.LookupEnv(env, envKeyXDGConfigHome)
	if ok && base != "" {
		return filepath.Join(base, binName, configFileName), true
	}
	if home, ok := gomake.LookupEnv(env, envKeyHome); ok && home != "" {
		return filepath.Join(home, ".config", binName, configFileName), true
	}
	return "", false
}

// projectConfigPath returns the path to the project-level gomake.yaml located
// in the source-scan directory srcDir.
func projectConfigPath(srcDir string) string {
	return filepath.Join(srcDir, configFileName)
}

// loadConfigFile reads and parses the gomake.yaml file at pth. A missing file
// yields an empty configuration and no error. Any other read error, invalid
// YAML, an unknown gomake-owned key, or an unsupported schema version is
// reported as an error.
func loadConfigFile(pth string) (*fileConfig, error) {
	data, err := os.ReadFile(pth)
	if errors.Is(err, os.ErrNotExist) {
		return &fileConfig{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errCfgParse, err)
	}
	return parseConfigFile(data)
}

// parseConfigFile decodes the content of a gomake.yaml file. Empty content
// yields an empty configuration. It rejects unknown keys on the gomake-owned
// surface, a missing or non-positive version, and a version newer than
// [configSchemaVersion].
func parseConfigFile(data []byte) (*fileConfig, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return &fileConfig{}, nil
	}

	var cfg fileConfig
	dec := yaml.NewDecoder(bytes.NewReader(data), yaml.Strict())
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("%w: %w", errCfgParse, err)
	}

	if cfg.Version == nil || *cfg.Version < 1 {
		return nil, errCfgVersion
	}
	if *cfg.Version > configSchemaVersion {
		return nil, fmt.Errorf("%w: %d", errCfgVersionNew, *cfg.Version)
	}
	return &cfg, nil
}

// canonicalKey returns the canonical configuration key for tgt, of the form
// "<import-path>#<name>". The name is the target's Go identifier as declared in
// its defining package: a plain function contributes its function name, and a
// method contributes "<Receiver>.<FuncName>". The key is derived from the
// target's origin and is independent of any local import alias or namespace
// rename applied where the target is used.
//
// A target defined in the project's own makefile package carries an empty
// [mkf.Target.ImpSpec]; localImp supplies that package's import path for such
// targets.
func canonicalKey(tgt *mkf.Target, localImp string) string {
	imp := tgt.ImpSpec
	if imp == "" {
		imp = localImp
	}
	name := tgt.FuncName
	if tgt.Receiver != "" {
		name = tgt.Receiver + "." + tgt.FuncName
	}
	return imp + "#" + name
}

// localImportPath returns the import path of the makefile package in the
// source-scan directory srcDir. It returns an empty string when srcDir is not
// inside a Go module.
func localImportPath(srcDir string) string {
	modPath := moduleImportPath(srcDir)
	if modPath == "" {
		return ""
	}
	root, err := gomake.Root(srcDir)
	if err != nil {
		return modPath
	}
	rel, err := filepath.Rel(root, srcDir)
	if err != nil || rel == "." || rel == "" {
		return modPath
	}
	return modPath + "/" + filepath.ToSlash(rel)
}

// moduleImportPath returns the module path declared in the go.mod file found by
// walking up from dir. It returns an empty string when dir is not inside a Go
// module or the module directive cannot be read.
func moduleImportPath(dir string) string {
	modFile, err := gomake.Root(dir, "go.mod")
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(modFile)
	if err != nil {
		return ""
	}

	scn := bufio.NewScanner(bytes.NewReader(data))
	for scn.Scan() {
		line := strings.TrimSpace(scn.Text())
		rest, ok := strings.CutPrefix(line, "module")
		if !ok || rest == "" || (rest[0] != ' ' && rest[0] != '\t') {
			continue
		}
		rest = strings.TrimSpace(rest)
		if i := strings.Index(rest, "//"); i >= 0 {
			rest = strings.TrimSpace(rest[:i])
		}
		if rest = strings.Trim(rest, "\"`"); rest != "" {
			return rest
		}
	}
	return ""
}

// mergeConfigs merges the user-level and project-level configurations. For both
// settings and target blocks the project-level file wins; a target block is
// replaced as a whole rather than deep-merged. When modPath is non-empty
// (gomake runs inside a module), user-level target entries whose key resolves
// to a target within that module are discarded.
func mergeConfigs(user, project *fileConfig, modPath string) *mergedConfig {
	m := &mergedConfig{targets: make(map[string]any)}

	for key, blk := range user.Targets {
		if modPath != "" && keyInModule(key, modPath) {
			continue
		}
		m.targets[key] = blk
	}
	maps.Copy(m.targets, project.Targets)

	m.timeout = pickSetting(user.timeout(), project.timeout())
	m.tmp = pickSetting(user.tmp(), project.tmp())
	return m
}

// keyInModule reports whether the target key's import path is the module
// modPath or a package within it.
func keyInModule(key, modPath string) bool {
	imp := key
	if before, _, ok := strings.Cut(key, "#"); ok {
		imp = before
	}
	return imp == modPath || strings.HasPrefix(imp, modPath+"/")
}

// pickSetting returns project when it is non-nil, otherwise user.
func pickSetting(user, project *string) *string {
	if project != nil {
		return project
	}
	return user
}

// timeout returns the settings.timeout value, or nil when it is unset.
func (cfg *fileConfig) timeout() *string {
	if cfg == nil || cfg.Settings == nil {
		return nil
	}
	return cfg.Settings.Timeout
}

// tmp returns the settings.tmp value, or nil when it is unset.
func (cfg *fileConfig) tmp() *string {
	if cfg == nil || cfg.Settings == nil {
		return nil
	}
	return cfg.Settings.Tmp
}

// applyFileConfig loads the user-level and project-level gomake.yaml files,
// merges them, stores the per-target configuration on cfg, and applies the
// settings section. The timeout and temporary directory are set only when a
// higher-precedence source (a command-line option, or the environment for the
// temporary directory) has not already provided a value.
func (cfg *config) applyFileConfig(env []string) error {
	user := &fileConfig{}
	if pth, ok := userConfigPath(env); ok {
		var err error
		if user, err = loadConfigFile(pth); err != nil {
			return err
		}
	}

	project, err := loadConfigFile(projectConfigPath(cfg.src))
	if err != nil {
		return err
	}

	merged := mergeConfigs(user, project, moduleImportPath(cfg.src))
	cfg.targetCfg = merged.targets

	set := make(map[string]bool)
	cfg.fs.Visit(func(f *flag.Flag) { set[f.Name] = true })

	if !set["timeout"] && merged.timeout != nil {
		dur, err := time.ParseDuration(*merged.timeout)
		if err != nil {
			return fmt.Errorf("%w: settings.timeout: %w", errCfgParse, err)
		}
		cfg.timeout = dur
	}

	if cfg.tmp == "" {
		envMap := gomake.EnvSplit(env)
		switch {
		case envMap[envKeyTmpDir] != "":
			cfg.tmp = envMap[envKeyTmpDir]
		case merged.tmp != nil && *merged.tmp != "":
			cfg.tmp = *merged.tmp
		default:
			cfg.tmp = filepath.Join(os.TempDir(), binName)
		}
	}
	return nil
}

// deliverTargetConfig places tgt's configuration block, if any, into the ring
// meta-store under [gomake.ConfigMetaKey] so the running target can read it
// with [gomake.TargetConfig]. It is a no-op when tgt is nil or unconfigured.
// For an in-process target this is the whole delivery; for a target compiled
// into a makefile subprocess, [goMake.Execute] ferries the same value across
// the process boundary via [targetConfigArg].
func deliverTargetConfig(rng *ring.Ring, cfg *config, tgt *mkf.Target) error {
	if tgt == nil || len(cfg.targetCfg) == 0 {
		return nil
	}
	blk, ok := cfg.targetCfg[canonicalKey(tgt, localImportPath(cfg.src))]
	if !ok {
		return nil
	}

	data, err := json.Marshal(blk)
	if err != nil {
		return fmt.Errorf("%w: %w", errCfgParse, err)
	}
	rng.MetaSet(gomake.ConfigMetaKey, string(data))
	return nil
}

// invokedTarget resolves the target the user asked to run from tgs. When no
// target name was given it returns the default target, if any.
func invokedTarget(cfg *config, tgs *parser.Targets) *mkf.Target {
	if cfg.target != "" {
		return tgs.Get(cfg.target)
	}
	for _, tgt := range tgs.List() {
		if tgt.Default {
			return tgt
		}
	}
	return nil
}

// runCheckConfig implements the "--check-config" option. It discovers all
// targets, reloads the user-level and project-level gomake.yaml files, and
// prints the report from [checkConfigReport] to standard error.
func runCheckConfig(rng *ring.Ring, cfg *config, stock []*mkf.Target) error {
	tgts, err := allTargets(rng, cfg, stock)
	if err != nil {
		return err
	}

	user := &fileConfig{}
	if pth, ok := userConfigPath(rng.EnvAll()); ok {
		if user, err = loadConfigFile(pth); err != nil {
			return err
		}
	}
	project, err := loadConfigFile(projectConfigPath(cfg.src))
	if err != nil {
		return err
	}

	report := checkConfigReport(tgts, localImportPath(cfg.src), user, project)
	_, _ = fmt.Fprint(rng.Stderr(), report)
	return nil
}

// checkConfigReport builds the "--check-config" report. It lists the canonical
// key of every target in tgts (local targets, whose [mkf.Target.ImpSpec] is
// empty, resolve their import path from localImp) and reports the soft problems
// gomake tolerates during a normal run: a project-level target key matching no
// discovered target, and a non-absolute settings.tmp in either file.
func checkConfigReport(
	tgts []*mkf.Target,
	localImp string,
	user, project *fileConfig,
) string {

	valid := make(map[string]bool, len(tgts))
	keys := make([]string, 0, len(tgts))
	for _, tgt := range tgts {
		key := canonicalKey(tgt, localImp)
		if !valid[key] {
			valid[key] = true
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)

	problems := checkConfigProblems(valid, user, project)

	var b strings.Builder
	if len(problems) == 0 {
		b.WriteString("gomake.yaml: no problems found\n")
	} else {
		b.WriteString("gomake.yaml: problems found\n")
		for _, prob := range problems {
			b.WriteString("  - " + prob + "\n")
		}
	}

	b.WriteString("\nconfig keys:\n")
	if len(keys) == 0 {
		b.WriteString("  (no targets)\n")
	}
	for _, key := range keys {
		b.WriteString("  " + key + "\n")
	}
	return b.String()
}

// checkConfigProblems returns the soft configuration problems, in report order:
// non-absolute settings.tmp values (user then project) followed by sorted
// project-level target keys absent from valid.
func checkConfigProblems(
	valid map[string]bool,
	user, project *fileConfig,
) []string {

	var problems []string
	for _, ent := range []struct {
		name string
		cfg  *fileConfig
	}{{"user", user}, {"project", project}} {
		tmp := ent.cfg.tmp()
		if tmp != nil && *tmp != "" && !filepath.IsAbs(*tmp) {
			format := "%s settings.tmp is not an absolute path: %s"
			problems = append(problems, fmt.Sprintf(format, ent.name, *tmp))
		}
	}

	unmatched := make([]string, 0)
	for key := range project.Targets {
		if !valid[key] {
			unmatched = append(unmatched, key)
		}
	}
	sort.Strings(unmatched)
	for _, key := range unmatched {
		problems = append(problems, "project target key has no target: "+key)
	}
	return problems
}
