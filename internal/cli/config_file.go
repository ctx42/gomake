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
// a project-level gomake.yaml are loaded; the settings section is merged
// (project wins) while target configuration is kept as a nested tree keyed by
// import path and then by the target's kebab invocation-path names. The invoked
// target's nearest-level block is resolved (project first, user as a fallback)
// and delivered to it through the ring meta store; "--check-config" lists the
// discovered targets grouped by import path and flags soft structural problems.

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
// gomake-owned surface (version and settings) is parsed strictly; the target
// configuration is kept as an opaque nested tree that gomake navigates but does
// not validate.
type fileConfig struct {
	// Version is the configuration schema version.
	Version *int `yaml:"version"`

	// Settings is gomake's own configuration.
	Settings *fileSettings `yaml:"settings"`

	// Targets maps an import path to a nested tree of namespace and target
	// nodes; each node's non-child keys are that node's opaque setting block.
	Targets map[string]any `yaml:"targets"`
}

// mergedConfig is the effective gomake.yaml settings after merging the
// user-level and project-level files. Target configuration is not merged; it is
// resolved per invocation by [resolveDelivered].
type mergedConfig struct {
	// timeout is the effective settings.timeout, or nil when unset.
	timeout *string

	// tmp is the effective settings.tmp, or nil when unset.
	tmp *string
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

// targetImp returns tgt's import path. A target defined in the project's own
// makefile package carries an empty [mkf.Target.ImpSpec]; localImp supplies
// that package's import path for such targets.
func targetImp(tgt *mkf.Target, localImp string) string {
	if tgt.ImpSpec != "" {
		return tgt.ImpSpec
	}
	return localImp
}

// namePath splits a target's CLI name into its node-path components, dropping
// empty segments. It mirrors the pieces the parser joins to build the name, so
// "go:lint:install" becomes {"go", "lint", "install"} and a bare "build"
// becomes {"build"}.
func namePath(name string) []string {
	raw := strings.Split(name, ":")
	path := make([]string, 0, len(raw))
	for _, seg := range raw {
		if seg != "" {
			path = append(path, seg)
		}
	}
	return path
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

// mergeConfigs merges the settings sections of the user-level and
// project-level configurations. The project-level file wins and the user-level
// file fills the gaps. Target configuration is not merged; each target's block
// is resolved per invocation by [resolveDelivered].
func mergeConfigs(user, project *fileConfig) *mergedConfig {
	return &mergedConfig{
		timeout: pickSetting(user.timeout(), project.timeout()),
		tmp:     pickSetting(user.tmp(), project.tmp()),
	}
}

// impInModule reports whether the import path imp is the module modPath or a
// package within it.
func impInModule(imp, modPath string) bool {
	return imp == modPath || strings.HasPrefix(imp, modPath+"/")
}

// knownNodes returns the set of node-path keys, colon-joined, contributed by
// every target in tgts whose import path is imp. Each target contributes every
// non-empty prefix of its name path, recording both namespace nodes and leaf
// target nodes; the set lets the resolver tell a child-node key from a setting
// key.
func knownNodes(tgts []*mkf.Target, localImp, imp string) map[string]bool {
	known := make(map[string]bool)
	for _, tgt := range tgts {
		if targetImp(tgt, localImp) != imp {
			continue
		}
		path := namePath(tgt.Name)
		for i := 1; i <= len(path); i++ {
			known[strings.Join(path[:i], ":")] = true
		}
	}
	return known
}

// settingKeys returns a shallow copy of node keeping only its setting keys —
// those whose colon-joined path (prefix plus the key) is not a known child
// node. Child namespace and target keys are dropped so a delivered block never
// carries a nested target's configuration.
func settingKeys(
	prefix []string,
	node map[string]any,
	known map[string]bool,
) map[string]any {

	base := strings.Join(prefix, ":")
	out := make(map[string]any, len(node))
	for key, val := range node {
		full := key
		if base != "" {
			full = base + ":" + key
		}
		if known[full] {
			continue
		}
		out[key] = val
	}
	return out
}

// resolveTargetBlock walks the nested target tree rooted at root along the
// invoked target's node path and returns its nearest-level settings block. It
// descends as far as both root and path allow, then walks back up to the first
// node carrying at least one setting key, returning that node's setting keys
// with child-node keys stripped. The boolean is false when no node on the path
// carries settings. known reports which colon-joined node paths name a child
// namespace or target.
func resolveTargetBlock(
	root map[string]any,
	path []string,
	known map[string]bool,
) (map[string]any, bool) {

	type level struct {
		prefix []string
		node   map[string]any
	}

	chain := []level{{nil, root}}
	node := root
	for _, comp := range path {
		child, ok := node[comp].(map[string]any)
		if !ok {
			break
		}
		prev := chain[len(chain)-1].prefix
		prefix := append(append([]string{}, prev...), comp)
		chain = append(chain, level{prefix, child})
		node = child
	}

	for i := len(chain) - 1; i >= 0; i-- {
		blk := settingKeys(chain[i].prefix, chain[i].node, known)
		if len(blk) > 0 {
			return blk, true
		}
	}
	return nil, false
}

// resolveDelivered resolves the configuration block delivered to tgt. It looks
// up tgt's import path in the project-level tree first and returns its
// nearest-level block when present; otherwise, unless tgt's import path lies
// within modPath, it falls back to the user-level tree. tgts supplies the
// discovered targets used to distinguish child-node keys from settings. The
// boolean is false when neither tree carries a block for tgt.
func resolveDelivered(
	tgt *mkf.Target,
	localImp, modPath string,
	userTgts, projectTgts map[string]any,
	tgts []*mkf.Target,
) (map[string]any, bool) {

	imp := targetImp(tgt, localImp)
	path := namePath(tgt.Name)
	known := knownNodes(tgts, localImp, imp)

	if root, ok := projectTgts[imp].(map[string]any); ok {
		if blk, ok := resolveTargetBlock(root, path, known); ok {
			return blk, true
		}
	}
	if modPath != "" && impInModule(imp, modPath) {
		return nil, false
	}
	if root, ok := userTgts[imp].(map[string]any); ok {
		if blk, ok := resolveTargetBlock(root, path, known); ok {
			return blk, true
		}
	}
	return nil, false
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

	cfg.userTargets = user.Targets
	cfg.projectTargets = project.Targets

	merged := mergeConfigs(user, project)

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

// deliverTargetConfig places tgt's nearest-level configuration block, if any,
// into the ring meta-store under [gomake.ConfigMetaKey] so the running target
// can read it with [gomake.TargetConfig]. tgts supplies the discovered targets
// used to distinguish child-node keys from settings. It is a no-op when tgt is
// nil or no configuration file carries a block for it. For an in-process target
// this is the whole delivery; for a target compiled into a makefile subprocess,
// [goMake.Execute] ferries the same value across the process boundary via
// [targetConfigArg].
func deliverTargetConfig(
	rng *ring.Ring,
	cfg *config,
	tgt *mkf.Target,
	tgts []*mkf.Target,
) error {

	if tgt == nil ||
		(len(cfg.userTargets) == 0 && len(cfg.projectTargets) == 0) {
		return nil
	}

	blk, ok := resolveDelivered(
		tgt,
		localImportPath(cfg.src),
		moduleImportPath(cfg.src),
		cfg.userTargets,
		cfg.projectTargets,
		tgts,
	)
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

// checkConfigReport builds the "--check-config" report. It lists every
// discovered target grouped by import path (local targets, whose
// [mkf.Target.ImpSpec] is empty, resolve their import path from localImp),
// naming each target by its kebab node path, and reports the soft problems
// gomake tolerates during a normal run (see [checkConfigProblems]).
func checkConfigReport(
	tgts []*mkf.Target,
	localImp string,
	user, project *fileConfig,
) string {

	imps := make(map[string]map[string]bool)
	for _, tgt := range tgts {
		imp := targetImp(tgt, localImp)
		if imps[imp] == nil {
			imps[imp] = make(map[string]bool)
		}
		imps[imp][strings.Join(namePath(tgt.Name), ":")] = true
	}

	problems := checkConfigProblems(imps, user, project)

	var b strings.Builder
	if len(problems) == 0 {
		b.WriteString("gomake.yaml: no problems found\n")
	} else {
		b.WriteString("gomake.yaml: problems found\n")
		for _, prob := range problems {
			b.WriteString("  - ")
			b.WriteString(prob)
			b.WriteString("\n")
		}
	}

	b.WriteString("\ntargets:\n")
	if len(imps) == 0 {
		b.WriteString("  (no targets)\n")
	}
	paths := make([]string, 0, len(imps))
	for imp := range imps {
		paths = append(paths, imp)
	}
	sort.Strings(paths)
	for _, imp := range paths {
		b.WriteString("  ")
		b.WriteString(imp)
		b.WriteString(":\n")
		names := make([]string, 0, len(imps[imp]))
		for name := range imps[imp] {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			b.WriteString("    ")
			b.WriteString(name)
			b.WriteString("\n")
		}
	}
	return b.String()
}

// checkConfigProblems returns the soft configuration problems, in report order:
// non-absolute settings.tmp values (user then project), sorted project-level
// import keys whose value is not a mapping, then sorted project-level import
// keys matching no discovered target. validImps holds the discovered import
// paths keyed to their target names.
func checkConfigProblems(
	validImps map[string]map[string]bool,
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

	var malformed, unmatched []string
	for imp, blk := range project.Targets {
		if _, ok := blk.(map[string]any); !ok {
			malformed = append(malformed, imp)
			continue
		}
		if validImps[imp] == nil {
			unmatched = append(unmatched, imp)
		}
	}
	sort.Strings(malformed)
	sort.Strings(unmatched)
	for _, imp := range malformed {
		problems = append(problems, "project config is not a mapping: "+imp)
	}
	for _, imp := range unmatched {
		problems = append(problems, "project import path has no target: "+imp)
	}
	return problems
}
