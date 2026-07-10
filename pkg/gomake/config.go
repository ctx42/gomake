// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gomake

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ctx42/ring/pkg/ring"
)

// ConfigMetaKey is the ring meta-store key under which gomake places the
// running target's configuration block as a JSON string. Read the block with
// [TargetConfig] rather than accessing the key directly.
const ConfigMetaKey = "github.com/ctx42/gomake/pkg/gomake.targetConfig"

// Configuration lookup errors.
var (
	// ErrMiss indicates a path that does not resolve to a value: an absent
	// key, an out-of-range or non-numeric array index, or an empty path.
	ErrMiss = errors.New("config path not found")

	// ErrType indicates a value that cannot be represented as the requested
	// type, including descending through a scalar leaf.
	ErrType = errors.New("config type mismatch")
)

// Config is the running target's configuration block, decoded from the JSON
// gomake stored in the ring meta store under [ConfigMetaKey]. It is an
// immutable snapshot; read values with [GetCfg] and [Config.Has]. The zero
// value is safe and behaves like an empty block.
type Config struct {
	data map[string]any
}

// TargetConfig decodes the running target's configuration block from the ring
// meta store and returns it. The block comes from the target's entry in a
// gomake.yaml file. gomake stores it as a string; a []byte is also accepted so
// a caller that embeds gomake and sets the meta value itself may use either.
// When the target has no configuration, which includes a meta value present
// but neither string nor []byte, it returns an empty [Config] and a nil error;
// it returns an error only when a present block is not valid JSON.
func TargetConfig(rng *ring.Ring) (*Config, error) {
	cfg := &Config{data: map[string]any{}}
	raw, ok := rng.MetaLookup(ConfigMetaKey)
	if !ok {
		return cfg, nil
	}
	var data []byte
	switch val := raw.(type) {
	case []byte:
		data = val

	case string:
		data = []byte(val)

	default:
		return cfg, nil
	}
	if err := json.Unmarshal(data, &cfg.data); err != nil {
		return nil, fmt.Errorf("gomake: target config: %w", err)
	}
	return cfg, nil
}

// Has reports whether path resolves to a value in the configuration block.
// Path segments are dot-separated and resolved against the current node's
// type: a map segment is a key, an array segment a zero-based index. A segment
// wrapped in single quotes is taken literally, so a map key that contains a
// dot is addressed as 'github.com/acme/app'; see [splitPath] for the grammar.
func (cfg *Config) Has(path string) bool {
	_, err := cfg.resolve(path)
	return err == nil
}

// resolve walks path through the configuration block and returns the value it
// names. It returns [ErrMiss] for an absent key, an out-of-range or
// non-numeric index, an empty path, or a malformed quoted segment, and
// [ErrType] when a segment descends through a scalar leaf.
func (cfg *Config) resolve(path string) (any, error) {
	segs, err := splitPath(path)
	if err != nil {
		return nil, err
	}

	var cur any = cfg.data
	for _, seg := range segs {
		switch node := cur.(type) {
		case map[string]any:
			val, ok := node[seg]
			if !ok {
				return nil, fmt.Errorf("%w: %q", ErrMiss, path)
			}
			cur = val

		case []any:
			idx, err := strconv.Atoi(seg)
			if err != nil || idx < 0 || idx >= len(node) {
				return nil, fmt.Errorf("%w: %q", ErrMiss, path)
			}
			cur = node[idx]

		default:
			return nil, fmt.Errorf("%w: %q", ErrType, path)
		}
	}
	return cur, nil
}

// splitPath cuts a dot-separated configuration path into its segments. A
// segment wrapped in single quotes is emitted verbatim, so the dots inside it
// are part of the key rather than separators: the path
// modules.'github.com/acme/app'.package names three segments. A single quote
// is significant only as the first character of a segment, and its closing
// quote must end that segment, being followed by a dot or the end of the path;
// elsewhere a quote is an ordinary character. splitPath returns [ErrMiss] for
// an empty path, an unterminated quote, or a quote not at a segment boundary.
func splitPath(path string) ([]string, error) {
	if path == "" {
		return nil, fmt.Errorf("%w: empty path", ErrMiss)
	}

	var segs []string
	for i := 0; i <= len(path); {
		if i < len(path) && path[i] == '\'' {
			end := strings.IndexByte(path[i+1:], '\'')
			if end < 0 {
				format := "%w: unterminated quote: %q"
				return nil, fmt.Errorf(format, ErrMiss, path)
			}
			end += i + 1
			if end != len(path)-1 && path[end+1] != '.' {
				format := "%w: quote not at segment boundary: %q"
				return nil, fmt.Errorf(format, ErrMiss, path)
			}
			segs = append(segs, path[i+1:end])
			i = end + 2 // Step past the closing quote and the dot, if any.
			continue
		}

		dot := strings.IndexByte(path[i:], '.')
		if dot < 0 {
			segs = append(segs, path[i:])
			break
		}
		segs = append(segs, path[i:i+dot])
		i += dot + 1
	}
	return segs, nil
}

// GetCfg resolves path against cfg and returns the value as T. The path is
// dot-separated; see [Config.Has] for the grammar. The value is converted to
// T through a JSON round-trip, so T may be any JSON-decodable type: a scalar,
// a slice, a map, or a struct with json tags. The conversion is strict: a
// number becomes an integer only when it has no fractional part, and a value
// of the wrong JSON kind is rejected.
//
// Two types are special: a [time.Duration] is taken from a JSON number as its
// nanosecond count, or from a string parsed with [time.ParseDuration]; and T of
// any yields the raw decoded value.
//
// GetCfg returns [ErrMiss] when path does not resolve and [ErrType] when the
// value cannot be represented as T.
func GetCfg[T any](cfg *Config, path string) (T, error) {
	var out T
	raw, err := cfg.resolve(path)
	if err != nil {
		return out, err
	}

	if dur, ok := any(&out).(*time.Duration); ok {
		// A string is parsed with time.ParseDuration; a number falls through to
		// the JSON round-trip below, which decodes it into the int64 nanosecond
		// count a Duration holds (the form encoding/json marshals it to).
		if text, ok := raw.(string); ok {
			val, err := time.ParseDuration(text)
			if err != nil {
				return out, fmt.Errorf("%w: %q: %w", ErrType, path, err)
			}
			*dur = val
			return out, nil
		}
	}

	if ptr, ok := any(&out).(*any); ok {
		*ptr = raw
		return out, nil
	}

	data, err := json.Marshal(raw)
	if err != nil {
		return out, fmt.Errorf("%w: %q: %w", ErrType, path, err)
	}
	if err = json.Unmarshal(data, &out); err != nil {
		return out, fmt.Errorf("%w: %q: %w", ErrType, path, err)
	}
	return out, nil
}

// GetCfgDefault returns the value at path from cfg decoded as T, or def when
// cfg omits path. It is a convenience over [GetCfg] for the common case of
// reading a single optional setting: an absent path yields def and a nil
// error, so a target needs no [ErrMiss] handling of its own. A value present
// but not representable as T is returned as an error. See [GetCfg] for the
// path grammar and the supported types.
func GetCfgDefault[T any](cfg *Config, path string, def T) (T, error) {
	val, err := GetCfg[T](cfg, path)
	if errors.Is(err, ErrMiss) {
		return def, nil
	}
	if err != nil {
		return def, err
	}
	return val, nil
}
