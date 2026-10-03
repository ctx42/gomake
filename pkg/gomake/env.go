// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gomake

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"
)

// Public gomake environment contract keys.
const (
	// VersionEnvKey is the environment variable carrying the gomake version
	// string. The runtime sets it on the process environment (and ring meta)
	// so a target can read the version of the gomake tool that invoked it.
	VersionEnvKey = "GOMAKE_VERSION"

	// ProjectDirEnvKey is the environment variable carrying the project
	// directory (the --src path). The runtime sets it on the process
	// environment (and ring meta) so a target can locate the project root.
	ProjectDirEnvKey = "GOMAKE_PROJECT_DIR"
)

// EnvSplit parses [os.Environ] results and returns it as a key value map.
func EnvSplit(env []string) map[string]string {
	m, _ := EnvSplitOrdered(env)
	return m
}

// EnvSplitOrdered parses [os.Environ] results and returns it as a key value
// map and a slice with the order of first-seen keys. When a key appears more
// than once, the map keeps the last value and the order list keeps a single
// entry at the key's first occurrence.
func EnvSplitOrdered(env []string) (map[string]string, []string) {
	k := make([]string, 0, 10)
	m := make(map[string]string, 10)
	for _, s := range env {
		if s == "" {
			continue
		}
		parts := strings.SplitN(s, "=", 2)
		if len(parts) != 2 || parts[0] == "" {
			continue
		}
		key := parts[0]
		if _, seen := m[key]; !seen {
			k = append(k, key)
		}
		m[key] = parts[1]
	}
	return m, k
}

// EnvJoin joins an environment variable map into a slice of "key=value"
// strings, matching the return type of [os.Environ]. Keys are sorted so the
// result does not depend on map iteration order.
func EnvJoin(env map[string]string) []string {
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		out = append(out, key+"="+env[key])
	}
	return out
}

// Expander returns a function used to return values from the parsed
// environment. If the key does not exist, the returned function returns an
// empty string. The returned function signature matches the [os.Expand]
// mapping argument.
func Expander(env []string) func(string) string {
	m := EnvSplit(env)
	return func(key string) string {
		return m[key]
	}
}

// PrettyPrintEnv pretty prints environment variables to the writer, skipping
// entries with empty values. It returns the first write error encountered.
func PrettyPrintEnv(env []string, buf io.Writer) error {
	envMap, keys := EnvSplitOrdered(env)
	tw := tabwriter.NewWriter(buf, 0, 8, 4, ' ', 0)
	for _, name := range keys {
		if envMap[name] == "" {
			continue
		}
		_, err := fmt.Fprintf(tw, "%s\t%s\n", name, envMap[name])
		if err != nil {
			return err
		}
	}
	return tw.Flush()
}
