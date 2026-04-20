package federation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
)

// CanonicalJSON produces RFC 8785 canonical JSON: sorted keys, no
// whitespace, no trailing newline. Go's encoding/json already sorts
// map keys (since Go 1.12) and struct fields follow tag order. For
// struct types the caller must ensure fields are declared in the
// desired canonical order via json tags.
//
// Values are first marshalled to JSON, then unmarshalled into an
// interface{} tree so that map key ordering is applied recursively
// (struct field order from tags is NOT guaranteed to survive a
// marshal→unmarshal round-trip through map[string]any, but RFC 8785
// requires sorted keys regardless of source order).
func CanonicalJSON(v any) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("canonical json marshal: %w", err)
	}

	var tree any
	if err := json.Unmarshal(raw, &tree); err != nil {
		return nil, fmt.Errorf("canonical json unmarshal: %w", err)
	}

	canonical := sortKeys(tree)

	out, err := json.Marshal(canonical)
	if err != nil {
		return nil, fmt.Errorf("canonical json re-marshal: %w", err)
	}
	return out, nil
}

// CanonicalHash computes domain-separated SHA-256 over canonical JSON:
//
//	SHA-256([]byte(domain) || CanonicalJSON(v))
func CanonicalHash(domain string, v any) ([]byte, error) {
	canonical, err := CanonicalJSON(v)
	if err != nil {
		return nil, fmt.Errorf("canonical hash: %w", err)
	}

	h := sha256.New()
	h.Write([]byte(domain))
	h.Write(canonical)
	sum := h.Sum(nil)
	return sum, nil
}

// HexHash returns hex-encoded CanonicalHash.
func HexHash(domain string, v any) (string, error) {
	hash, err := CanonicalHash(domain, v)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(hash), nil
}

// sortKeys recursively walks a JSON value tree and returns a new tree
// with all object keys sorted lexicographically. Arrays preserve order.
func sortKeys(v any) any {
	switch val := v.(type) {
	case map[string]any:
		sorted := make(map[string]any, len(val))
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			sorted[k] = sortKeys(val[k])
		}
		return sorted
	case []any:
		result := make([]any, len(val))
		for i, item := range val {
			result[i] = sortKeys(item)
		}
		return result
	default:
		return val
	}
}
