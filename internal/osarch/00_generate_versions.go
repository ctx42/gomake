// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

//go:build ignore

// Command 00_generate_versions writes versions_gen.go: the GOOS and GOARCH
// values supported by Go, as a single version-independent list. GOOS/GOARCH
// values are only added in newer releases, so the list is populated from the
// latest Go reported by https://go.dev/VERSION (a safe superset).
//
// Set GOMAKE_GO_VERSION (e.g. "1.28") to pin the Go version used to populate
// the list instead of querying go.dev. The matching toolchain is downloaded on
// demand via GOTOOLCHAIN. Run it with: go generate ./internal/osarch
package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"go/format"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

// versionURL reports the latest published Go version as plain text; its first
// line is a version such as "go1.26.3".
const versionURL = "https://go.dev/VERSION?m=text"

// gomakeModule is the module path identifying the gomake repository root.
const gomakeModule = "github.com/ctx42/gomake"

func main() {
	msg, err := run()
	if msg != "" {
		_, _ = fmt.Fprint(os.Stderr, msg)
	}
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "generate versions:", err)
		os.Exit(1)
	}
}

func run() (string, error) {
	root, err := moduleRoot()
	if err != nil {
		return "", err
	}
	repoVer, err := goModVersion(filepath.Join(root, "go.mod"))
	if err != nil {
		return "", err
	}
	localVer := majorMinor(runtime.Version())
	if localVer == "" {
		format := "cannot parse toolchain version %q"
		return "", fmt.Errorf(format, runtime.Version())
	}

	targetVer, toolchain, err := target(localVer)
	if err != nil {
		return "", err
	}
	if verLess(targetVer, repoVer) {
		format := "target Go %s is older than gomake go.mod Go %s"
		return "", fmt.Errorf(format, targetVer, repoVer)
	}

	goos, goarch, err := distList(toolchain)
	if err != nil {
		return "", fmt.Errorf("Go %s (%s): %w", targetVer, toolchain, err)
	}
	if len(goos) == 0 || len(goarch) == 0 {
		format := "Go %s (%s): empty dist list"
		return "", fmt.Errorf(format, targetVer, toolchain)
	}
	format := "gomake: collected Go %s via %s\n"
	msg := fmt.Sprintf(format, targetVer, toolchain)

	src, err := render(targetVer, goos, goarch)
	if err != nil {
		return msg, err
	}
	dst := filepath.Join(root, "internal", "osarch", "versions_gen.go")
	if err = os.WriteFile(dst, src, 0o644); err != nil {
		return msg, fmt.Errorf("write %s: %w", dst, err)
	}
	return msg, nil
}

// target resolves the Go major.minor version to populate the list from and the
// GOTOOLCHAIN value to run `go tool dist list` under. It honors
// GOMAKE_GO_VERSION when set, otherwise queries go.dev for the latest release.
func target(localVer string) (ver, toolchain string, err error) {
	if env := strings.TrimSpace(os.Getenv("GOMAKE_GO_VERSION")); env != "" {
		ver = majorMinor(env)
		if ver == "" {
			return "", "", fmt.Errorf("invalid GOMAKE_GO_VERSION %q", env)
		}
		return ver, toolchainFor(ver, localVer, runtime.Version()), nil
	}

	full, mm, err := latestGo()
	if err != nil {
		return "", "", err
	}
	toolchain = full
	if mm == localVer {
		toolchain = runtime.Version()
	}
	return mm, toolchain, nil
}

// toolchainFor returns the GOTOOLCHAIN value for major.minor version mm: the
// running toolchain when mm is local (no download), else that minor's initial
// ".0" release.
func toolchainFor(mm, localVer, localFull string) string {
	if mm == localVer {
		return localFull
	}
	return "go" + mm + ".0"
}

// latestGo fetches the latest published Go version, returning the full version
// (e.g. "go1.26.3") and its major.minor (e.g. "1.26"). GOMAKE_VERSION_URL,
// when set, replaces the default endpoint.
func latestGo() (full, mm string, err error) {
	endpoint := versionURL
	override := strings.TrimSpace(os.Getenv("GOMAKE_VERSION_URL"))
	if override != "" {
		endpoint = override
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("fetch %s: %w", endpoint, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		format := "fetch %s: HTTP %d"
		return "", "", fmt.Errorf(format, endpoint, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("read %s: %w", endpoint, err)
	}
	full = strings.TrimSpace(strings.SplitN(string(body), "\n", 2)[0])
	mm = majorMinor(full)
	if mm == "" {
		return "", "", fmt.Errorf("cannot parse latest Go version %q", full)
	}
	return full, mm, nil
}

// distList runs `go tool dist list` under the given toolchain (downloaded via
// GOTOOLCHAIN when not the running one) and returns the sorted, unique GOOS and
// GOARCH values.
func distList(toolchain string) (goos, goarch []string, err error) {
	// GOTOOLCHAIN may download a toolchain, so allow several minutes, but
	// do not let a stuck go tool dist list run forever.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "tool", "dist", "list")
	cmd.Env = append(os.Environ(), "GOTOOLCHAIN="+toolchain)
	out, err := cmd.Output()
	if err != nil {
		if e, ok := errors.AsType[*exec.ExitError](err); ok {
			msg := strings.TrimSpace(string(e.Stderr))
			if msg == "" {
				return nil, nil, fmt.Errorf("go tool dist list: %w", e)
			}
			return nil, nil, fmt.Errorf("go tool dist list: %s: %w", msg, e)
		}
		return nil, nil, fmt.Errorf("go tool dist list: %w", err)
	}
	osSet := map[string]struct{}{}
	archSet := map[string]struct{}{}
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		pair := strings.SplitN(strings.TrimSpace(sc.Text()), "/", 2)
		if len(pair) != 2 {
			continue
		}
		osSet[pair[0]] = struct{}{}
		archSet[pair[1]] = struct{}{}
	}
	if err = sc.Err(); err != nil {
		return nil, nil, fmt.Errorf("scan dist list: %w", err)
	}
	return sortedKeys(osSet), sortedKeys(archSet), nil
}

func sortedKeys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// render returns gofmt-formatted Go source for versions_gen.go.
func render(ver string, goos, goarch []string) ([]byte, error) {
	var buf bytes.Buffer
	_, _ = buf.WriteString(
		"// Code generated by 00_generate_versions.go. DO NOT EDIT.\n\n",
	)
	_, _ = buf.WriteString("package osarch\n\n")
	_, _ = fmt.Fprintf(
		&buf, "// Generated from Go %s (go tool dist list).\n\n", ver,
	)
	_, _ = buf.WriteString("// goos lists every GOOS value supported by Go.\n")
	_, _ = fmt.Fprintf(&buf, "var goos = []string{%s}\n\n", quoteList(goos))
	_, _ = buf.WriteString(
		"// goarch lists every GOARCH value supported by Go.\n",
	)
	_, _ = fmt.Fprintf(&buf, "var goarch = []string{%s}\n", quoteList(goarch))

	src, err := format.Source(buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("format generated source: %w", err)
	}
	return src, nil
}

func quoteList(ss []string) string {
	parts := make([]string, len(ss))
	for i, s := range ss {
		parts[i] = strconv.Quote(s)
	}
	return strings.Join(parts, ", ")
}

// moduleRoot walks up from the working directory to the gomake module root.
func moduleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getwd: %w", err)
	}
	for {
		gomod := filepath.Join(dir, "go.mod")
		mod, err := modulePath(gomod)
		if err != nil {
			return "", fmt.Errorf("read %s: %w", gomod, err)
		}
		if mod == gomakeModule {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("gomake module root not found")
		}
		dir = parent
	}
}

// modulePath returns the module path declared in the go.mod at pth. A missing
// file returns "", nil. Any other read error is returned.
func modulePath(pth string) (string, error) {
	return directive(pth, "module ")
}

// goModVersion returns the go directive of the go.mod at pth as major.minor.
func goModVersion(pth string) (string, error) {
	line, err := directive(pth, "go ")
	if err != nil {
		return "", err
	}
	if mm := majorMinor(line); mm != "" {
		return mm, nil
	}
	return "", fmt.Errorf("no go directive in %s", pth)
}

// directive returns the trimmed remainder of the first line in the file at
// pth that starts with prefix. A missing file returns "", nil. Any other
// read or scan error is returned.
func directive(pth, prefix string) (string, error) {
	b, err := os.ReadFile(pth)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		rest, ok := strings.CutPrefix(line, strings.TrimSpace(prefix))
		if !ok || rest == "" || (rest[0] != ' ' && rest[0] != '\t') {
			continue
		}
		return directiveValue(rest), nil
	}
	if err = sc.Err(); err != nil {
		return "", err
	}
	return "", nil
}

// directiveValue drops a trailing // comment and surrounding quotes from a
// go.mod directive value.
func directiveValue(rest string) string {
	rest = strings.TrimSpace(rest)
	inQuote := false
	escaped := false
	for i := 0; i < len(rest); i++ {
		c := rest[i]
		if inQuote {
			if escaped {
				escaped = false
				continue
			}
			if c == '\\' {
				escaped = true
				continue
			}
			if c == '"' || c == '`' {
				inQuote = false
			}
			continue
		}
		if c == '"' || c == '`' {
			inQuote = true
			continue
		}
		if c == '/' && i+1 < len(rest) && rest[i+1] == '/' {
			rest = rest[:i]
			break
		}
	}
	return strings.Trim(strings.TrimSpace(rest), "\"`")
}

// majorMinor normalizes a Go version such as "go1.26.3", "1.26.3", "1.26",
// or a prerelease like "go1.26rc1" / "go1.26beta1" to "1.26". It returns an
// empty string when the input has no major.minor.
func majorMinor(v string) string {
	v = strings.TrimPrefix(v, "go")
	parts := strings.Split(v, ".")
	if len(parts) < 2 {
		return ""
	}
	if _, err := strconv.Atoi(parts[0]); err != nil {
		return ""
	}
	// Strip prerelease suffix from the minor (rc1, beta1, …).
	min := parts[1]
	i := 0
	for i < len(min) && min[i] >= '0' && min[i] <= '9' {
		i++
	}
	if i == 0 {
		return ""
	}
	min = min[:i]
	return parts[0] + "." + min
}

// verLess reports whether major.minor version a is older than b.
func verLess(a, b string) bool {
	aMaj, aMin := splitVer(a)
	bMaj, bMin := splitVer(b)
	if aMaj != bMaj {
		return aMaj < bMaj
	}
	return aMin < bMin
}

func splitVer(v string) (maj, min int) {
	parts := strings.Split(v, ".")
	if len(parts) > 0 {
		maj, _ = strconv.Atoi(parts[0])
	}
	if len(parts) > 1 {
		min, _ = strconv.Atoi(parts[1])
	}
	return maj, min
}
