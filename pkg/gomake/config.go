// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gomake

import (
	"encoding/json"
	"fmt"

	"github.com/ctx42/ring/pkg/ring"
)

// ConfigMetaKey is the ring meta-store key under which gomake places the
// running target's configuration block as a JSON string. Read the block with
// [TargetConfig] rather than accessing the key directly.
const ConfigMetaKey = "github.com/ctx42/gomake/pkg/gomake.targetConfig"

// TargetConfig decodes the running target's configuration block into v. The
// block originates from the target's entry in a gomake.yaml file and is the
// JSON value gomake stored in the ring meta store under [ConfigMetaKey]. It
// leaves v unchanged and returns nil when the target has no configuration,
// which includes the case where the meta value is present but not a string.
func TargetConfig(rng *ring.Ring, v any) error {
	raw, ok := rng.MetaLookup(ConfigMetaKey)
	if !ok {
		return nil
	}
	text, ok := raw.(string)
	if !ok {
		return nil
	}
	if err := json.Unmarshal([]byte(text), v); err != nil {
		return fmt.Errorf("gomake: target config: %w", err)
	}
	return nil
}
