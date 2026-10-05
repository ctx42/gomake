// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gomake

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/ctx42/ring/pkg/ring"
)

// Config is the running target's configuration block, decoded from the JSON
// gomake stored in the ring meta store under [ConfigMetaKey]. It is an
// immutable snapshot; read values with [GetCfg] and [Config.Has]. The zero
// value and a nil *Config are safe and behave like an empty block.
type Config struct {
	data map[string]any
}

// TargetConfig decodes the running target's configuration block from the ring
// meta store and returns it. The block comes from the target's entry in a
// gomake.yaml file. gomake stores it as a string; a []byte or json.RawMessage
// is also accepted so a caller that embeds gomake and sets the meta value
// itself may use either. When the target has no configuration, which includes
// a meta value of any other type, it returns an empty [Config] and a nil
// error; it returns [ErrConfig] only when a present block is not a single valid
// JSON object.
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

	case json.RawMessage:
		data = val

	case string:
		data = []byte(val)

	default:
		return cfg, nil
	}
	// UseNumber keeps integers exact beyond float64 mantissa range.
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&cfg.data); err != nil {
		return nil, fmt.Errorf("gomake: %w: %w", ErrConfig, err)
	}
	// Decoder.More is false when the next byte is ] or }, so a second
	// decode is what rejects trailing values. Only io.EOF means the block
	// ended at the first value.
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, fmt.Errorf("gomake: %w: trailing data", ErrConfig)
		}
		return nil, fmt.Errorf("gomake: %w: %w", ErrConfig, err)
	}
	if cfg.data == nil {
		// A JSON null root leaves data nil; treat it as an empty object.
		cfg.data = map[string]any{}
	}
	return cfg, nil
}

// Has reports whether path resolves to a value in the configuration block.
// Path segments are dot-separated and resolved against the current node's
// type: a map segment is a key, an array segment a zero-based index. A segment
// wrapped in single quotes is taken literally, so a map key that contains a
// dot is addressed as 'github.com/acme/app'. A quote is significant only as
// the first character of a segment, and its closing quote must end the
// segment.
func (cfg *Config) Has(path string) bool {
	_, err := cfg.resolve(path)
	return err == nil
}

// resolve walks path through the configuration block and returns the value it
// names. It returns [ErrMiss] for an absent key or an out-of-range or
// non-numeric index, [ErrPath] for a malformed path, and [ErrType] when a
// segment descends through a scalar leaf.
func (cfg *Config) resolve(path string) (any, error) {
	segs, err := splitPath(path)
	if err != nil {
		return nil, err
	}

	var data map[string]any // A nil Config behaves like an empty block.
	if cfg != nil {
		data = cfg.data
	}
	var cur any = data
	for _, seg := range segs {
		switch node := cur.(type) {
		case map[string]any:
			val, ok := node[seg]
			if !ok {
				return nil, fmt.Errorf("%w: %q", ErrMiss, path)
			}
			cur = val

		case []any:
			idx, ok := arrayIndex(seg)
			if !ok || idx >= len(node) {
				return nil, fmt.Errorf("%w: %q", ErrMiss, path)
			}
			cur = node[idx]

		default:
			return nil, fmt.Errorf("%w: %q", ErrType, path)
		}
	}
	return cur, nil
}

// arrayIndex parses seg as a zero-based array index written in decimal digits
// without a sign or a leading zero, so "1" and "0" are indexes but "+1", "-0"
// and "01" are not.
func arrayIndex(seg string) (int, bool) {
	if seg == "" || (len(seg) > 1 && seg[0] == '0') {
		return 0, false
	}
	for _, r := range seg {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	idx, err := strconv.Atoi(seg)
	if err != nil { // Overflow.
		return 0, false
	}
	return idx, true
}

// splitPath cuts a dot-separated configuration path into its segments. A
// segment wrapped in single quotes is emitted verbatim, so the dots inside it
// are part of the key rather than separators: the path
// modules.'github.com/acme/app'.package names three segments. A single quote
// is significant only as the first character of a segment, and its closing
// quote must end that segment, being followed by a dot or the end of the path;
// elsewhere a quote is an ordinary character. splitPath returns [ErrPath] for
// an empty path, an unterminated quote, or a quote not at a segment boundary.
func splitPath(path string) ([]string, error) {
	if path == "" {
		return nil, fmt.Errorf("%w: empty path", ErrPath)
	}

	var segs []string
	for i := 0; i <= len(path); {
		if i < len(path) && path[i] == '\'' {
			end := strings.IndexByte(path[i+1:], '\'')
			if end < 0 {
				format := "%w: unterminated quote: %q"
				return nil, fmt.Errorf(format, ErrPath, path)
			}
			end += i + 1
			if end != len(path)-1 && path[end+1] != '.' {
				format := "%w: quote not at segment boundary: %q"
				return nil, fmt.Errorf(format, ErrPath, path)
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
// number becomes an integer only when written without a fraction or an
// exponent (so 1e3 is rejected), and a value of the wrong JSON kind is
// rejected.
//
// Two types are special: a [time.Duration] is taken from a JSON number as its
// nanosecond count, or from a string parsed with [time.ParseDuration]; and T of
// any yields the raw decoded value.
//
// GetCfg returns [ErrMiss] when path does not resolve, [ErrPath] when it is
// malformed, and [ErrType] when the value cannot be represented as T.
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
			val, perr := time.ParseDuration(text)
			if perr != nil {
				return out, fmt.Errorf("%w: %q: %w", ErrType, path, perr)
			}
			*dur = val
			return out, nil
		}
	}

	if ptr, ok := any(&out).(*any); ok {
		// Deep-copy maps and slices so callers cannot mutate the snapshot.
		*ptr = cloneCfgValue(raw)
		return out, nil
	}

	// JSON null is present but not a typed value; reject for concrete T so
	// GetCfgDefault does not treat it as a zero value.
	if raw == nil {
		return out, fmt.Errorf("%w: %q: null", ErrType, path)
	}

	data, err := json.Marshal(raw)
	if err != nil {
		return out, fmt.Errorf("%w: %q: %w", ErrType, path, err)
	}
	// UseNumber so nested integers stay exact (same as TargetConfig).
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err = dec.Decode(&out); err != nil {
		return out, fmt.Errorf("%w: %q: %w", ErrType, path, err)
	}
	return out, nil
}

// GetCfgDefault returns the value at path from cfg decoded as T, or def when
// cfg omits path. It is a convenience over [GetCfg] for the common case of
// reading a single optional setting: an absent path yields def and a nil
// error, so a target needs no [ErrMiss] handling of its own. A malformed path
// ([ErrPath]) or a value present but not representable as T is returned as an
// error. See [GetCfg] for the
// path grammar and the supported types.
func GetCfgDefault[T any](cfg *Config, path string, def T) (T, error) {
	val, err := GetCfg[T](cfg, path)
	if errors.Is(err, ErrMiss) {
		return def, nil
	}
	if err != nil {
		var zero T
		return zero, err
	}
	return val, nil
}

// cloneCfgValue returns a deep copy of v for map and slice values produced by
// JSON decoding, so [GetCfg] with T=any cannot mutate the [Config] snapshot.
// Scalars and other types are returned as-is.
func cloneCfgValue(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, val := range x {
			out[k] = cloneCfgValue(val)
		}
		return out

	case []any:
		out := make([]any, len(x))
		for i, val := range x {
			out[i] = cloneCfgValue(val)
		}
		return out

	default:
		return v
	}
}
