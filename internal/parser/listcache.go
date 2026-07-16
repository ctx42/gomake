// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package parser

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ctx42/ring/pkg/ring"

	"github.com/ctx42/gomake/pkg/gomake"
)

// listCacheDir returns the directory holding cached `go list` results, creating
// it when needed.
func listCacheDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "gomake", "list")
	if err = os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

// listCacheKey returns the cache key for resolving import spec from the project
// at dir under the build tag, GOOS, and GOARCH held by rng. It hashes the
// project's go.mod and go.sum so any dependency change invalidates the key. It
// reports false when the project module cannot be located, in which case the
// `go list` result must not be cached.
func listCacheKey(rng *ring.Ring, dir, spec string) (string, bool) {
	root, err := gomake.Root(dir)
	if err != nil {
		return "", false
	}

	h := sha256.New()
	for _, name := range []string{"go.mod", "go.sum", "go.work", "go.work.sum"} {
		data, rErr := os.ReadFile(filepath.Join(root, name))
		if rErr == nil {
			_, _ = fmt.Fprintf(h, "%s\n", name)
			_, _ = h.Write(data)
		}
	}
	if gowork := strings.TrimSpace(rng.EnvGet("GOWORK")); gowork != "" {
		_, _ = fmt.Fprintf(h, "GOWORK:%s\n", gowork)
	}
	format := "spec:%s\ntag:%s\ngoos:%s\ngoarch:%s\ngo:%s\n"
	_, _ = fmt.Fprintf(
		h,
		format,
		spec,
		GetBuildTag(rng),
		rng.EnvGet("GOOS"),
		rng.EnvGet("GOARCH"),
		runtime.Version(),
	)
	return hex.EncodeToString(h.Sum(nil)), true
}

// cacheableModule reports whether a module's `go list` result is safe to cache
// across invocations. Only version-pinned modules that are not redirected by a
// replace directive have immutable module-cache content; the main module, local
// packages, and replaced modules are not content-pinned.
func cacheableModule(m module) bool {
	return m.Version != "" && m.Replace == nil
}

// loadListCache returns the cached `go list` output for key and true when a
// cache entry exists.
func loadListCache(key string) ([]byte, bool) {
	dir, err := listCacheDir()
	if err != nil {
		return nil, false
	}
	data, err := os.ReadFile(filepath.Join(dir, key))
	if err != nil {
		return nil, false
	}
	return data, true
}

// storeListCache writes the `go list` output for key to the cache. It is
// advisory: errors are ignored. The write is rename-atomic so concurrent
// readers never see a partial entry.
func storeListCache(key string, data []byte) {
	dir, err := listCacheDir()
	if err != nil {
		return
	}
	dst := filepath.Join(dir, key)
	tmp := dst + ".tmp"
	if err = os.WriteFile(tmp, data, 0600); err != nil {
		return
	}
	if err = os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
	}
}
