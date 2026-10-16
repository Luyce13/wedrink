package utils

import (
	"encoding/json"
	"fmt"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// ToCleanMap normalizes any state object (Go struct, bson.D, bson.M, JSON string, or serialized Key-Value pair slice)
// into a standard map[string]any suitable for JSON persistence, audit comparison, and diff computation.
func ToCleanMap(v any) map[string]any {
	if v == nil {
		return nil
	}

	// 1. Direct map[string]any
	if m, ok := v.(map[string]any); ok {
		return m
	}

	// 2. MongoDB BSON primitives
	if d, ok := v.(bson.D); ok {
		res := make(map[string]any, len(d))
		for _, e := range d {
			res[e.Key] = e.Value
		}
		return res
	}
	if d, ok := v.(*bson.D); ok && d != nil {
		res := make(map[string]any, len(*d))
		for _, e := range *d {
			res[e.Key] = e.Value
		}
		return res
	}
	if m, ok := v.(bson.M); ok {
		return map[string]any(m)
	}

	// 3. String containing JSON
	if str, ok := v.(string); ok {
		str = strings.TrimSpace(str)
		if strings.HasPrefix(str, "{") {
			var m map[string]any
			if err := json.Unmarshal([]byte(str), &m); err == nil {
				return m
			}
		} else if strings.HasPrefix(str, "[") {
			var slice []any
			if err := json.Unmarshal([]byte(str), &slice); err == nil {
				if m := parseKVSlice(slice); m != nil {
					return m
				}
			}
		}
	}

	// 4. Slice of Key/Value pairs (e.g. from JSON-unmarshaled bson.D)
	if slice, ok := v.([]any); ok {
		if m := parseKVSlice(slice); m != nil {
			return m
		}
	}

	// 5. General object serialization fallback
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}

	// First try unmarshaling as map
	var m map[string]any
	if err := json.Unmarshal(b, &m); err == nil && m != nil {
		return m
	}

	// If json.Marshal produced an array (such as when encoding bson.D)
	var slice []any
	if err := json.Unmarshal(b, &slice); err == nil {
		if res := parseKVSlice(slice); res != nil {
			return res
		}
	}

	return nil
}

// parseKVSlice converts an array of [{"Key": "k", "Value": v}] or [{"key": "k", "value": v}] into a map.
func parseKVSlice(slice []any) map[string]any {
	if len(slice) == 0 {
		return nil
	}
	res := make(map[string]any, len(slice))
	isKVPairs := true
	for _, item := range slice {
		if kv, ok := item.(map[string]any); ok {
			if k, hasKey := kv["Key"]; hasKey {
				res[fmt.Sprint(k)] = kv["Value"]
				continue
			}
			if k, hasKey := kv["key"]; hasKey {
				res[fmt.Sprint(k)] = kv["value"]
				continue
			}
		}
		isKVPairs = false
		break
	}
	if isKVPairs && len(res) > 0 {
		return res
	}
	return nil
}
