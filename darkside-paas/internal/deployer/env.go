package deployer

import (
	"encoding/json"
	"sort"
)

// snapshotJSON returns a deterministic JSON representation of an env map.
// Values are NOT redacted — full visibility by design.
func snapshotJSON(env map[string]string) string {
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make(map[string]string, len(env))
	for _, k := range keys {
		out[k] = env[k]
	}
	b, _ := json.MarshalIndent(out, "", "  ")
	return string(b)
}
