package vkverify

import (
	"archive/zip"
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"testing"
	"time"
)

// testBundle builds a valid VKE1 bundle ZIP for testing.
func testBundle(t *testing.T) (*bytes.Reader, ed25519.PublicKey) {
	t.Helper()

	pub, priv, _ := ed25519.GenerateKey(rand.Reader)

	caseID := "11111111-1111-1111-1111-111111111111"
	evID := "22222222-2222-2222-2222-222222222222"
	exchangeID := "33333333-3333-3333-3333-333333333333"
	now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)

	// Evidence content
	content := []byte("test evidence file content")
	contentHash := sha256.Sum256(content)
	contentHashHex := hex.EncodeToString(contentHash[:])

	// Scope
	scope := map[string]any{
		"scope_version": 1,
		"case_id":       caseID,
		"predicate":     map[string]any{"op": "eq", "field": "tags", "value": "test"},
		"snapshot": map[string]any{
			"as_of_custody_hash": "abc",
			"evaluated_at":       now.Format(time.RFC3339Nano),
		},
	}
	scopeHashBytes, _ := canonicalHash("VK:SCOPE:v1", scope)
	scopeHash := hex.EncodeToString(scopeHashBytes)

	// Descriptor
	descriptor := map[string]any{
		"evidence_id":    evID,
		"case_id":        caseID,
		"version":        1,
		"sha256":         contentHashHex,
		"classification": "public",
		"tags":           []string{"test"},
	}

	// Leaf + Merkle — single leaf tree: root = leaf hash itself
	leafHash, _ := canonicalHash("VK:MERKLE:LEAF:v1", descriptor)
	root := leafHash
	rootHex := hex.EncodeToString(root)

	fingerprint := computeFingerprint(pub)

	// Manifest (without hash)
	manifestBody := map[string]any{
		"protocol_version":        "VKE1",
		"exchange_id":             exchangeID,
		"sender_instance_id":      "test-instance",
		"sender_key_fingerprint":  fingerprint,
		"created_at":              now.Format(time.RFC3339Nano),
		"scope":                   scope,
		"scope_hash":              scopeHash,
		"scope_cardinality":       1,
		"merkle_root":             rootHex,
		"dependency_policy":       "none",
		"disclosed_evidence":      []any{descriptor},
		"sender_custody_head":     "custody-head",
		"sender_bridge_event_hash": "bridge-hash",
	}
	manifestHashBytes, _ := canonicalHash("VK:MANIFEST:v1", manifestBody)
	manifestHash := hex.EncodeToString(manifestHashBytes)

	// Full manifest with hash
	manifestBody["manifest_hash"] = manifestHash

	// Sign
	sig := ed25519.Sign(priv, manifestHashBytes)

	// Build ZIP
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	addJSON(t, zw, "vkx/version.json", map[string]any{
		"format":     "VKE1",
		"created_at": now.Format(time.RFC3339Nano),
	})
	addJSON(t, zw, "vkx/instance-identity.json", map[string]any{
		"instance_id": "test-instance",
		"public_key":  base64.StdEncoding.EncodeToString(pub),
		"fingerprint": fingerprint,
	})
	addJSON(t, zw, "vkx/scope.json", scope)
	addJSON(t, zw, "vkx/exchange-manifest.json", manifestBody)
	addJSON(t, zw, "vkx/exchange-signature.json", map[string]any{
		"signature": base64.StdEncoding.EncodeToString(sig),
		"algorithm": "ed25519",
	})
	addJSON(t, zw, "vkx/merkle-root.json", map[string]any{
		"root":       rootHex,
		"leaf_count": 1,
		"algorithm":  "sha256-domain-separated",
	})

	// Merkle proof (single leaf: empty proof, root = leaf)
	addJSON(t, zw, "vkx/merkle-proofs/"+evID+".json", map[string]any{
		"evidence_id": evID,
		"leaf_hash":   hex.EncodeToString(leafHash),
		"steps":       []map[string]any{},
	})

	// Custody bridge
	addJSON(t, zw, "vkx/custody-bridge.json", map[string]any{
		"action":           "disclosed_to_instance",
		"exchange_id":      exchangeID,
		"manifest_hash":    manifestHash,
		"scope_hash":       scopeHash,
		"merkle_root":      rootHex,
		"scope_cardinality": 1,
	})

	// Evidence
	addJSON(t, zw, "evidence/"+evID+"/descriptor.json", descriptor)
	addRaw(t, zw, "evidence/"+evID+"/content.bin", content)

	// Custody chain
	addRaw(t, zw, "custody/chain.json", []byte(`[]`))

	zw.Close()

	return bytes.NewReader(buf.Bytes()), pub
}

func computeFingerprint(pub ed25519.PublicKey) string {
	h := sha256.Sum256(pub)
	return "sha256:" + base64.StdEncoding.EncodeToString(h[:])
}

func addJSON(t *testing.T, zw *zip.Writer, name string, v any) {
	t.Helper()
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	fw, err := zw.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	fw.Write(data)
}

func addRaw(t *testing.T, zw *zip.Writer, name string, data []byte) {
	t.Helper()
	fw, err := zw.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	fw.Write(data)
}

func TestVerifyBundle_ValidBundle(t *testing.T) {
	reader, pub := testBundle(t)
	result, err := VerifyBundle(reader, int64(reader.Len()), pub)
	if err != nil {
		t.Fatal(err)
	}

	if !result.Valid {
		for _, s := range result.Steps {
			if !s.Passed {
				t.Errorf("step %s failed: %s", s.Step, s.Error)
			}
		}
		t.Fatal("expected valid bundle")
	}

	if result.BundleFormat != "VKE1" {
		t.Errorf("format = %q", result.BundleFormat)
	}
	if result.SenderInstanceID != "test-instance" {
		t.Errorf("sender = %q", result.SenderInstanceID)
	}
	if result.EvidenceCount != 1 {
		t.Errorf("evidence count = %d", result.EvidenceCount)
	}
}

func TestVerifyBundle_WrongPublicKey(t *testing.T) {
	reader, _ := testBundle(t)

	wrongPub, _, _ := ed25519.GenerateKey(rand.Reader)
	result, err := VerifyBundle(reader, int64(reader.Len()), wrongPub)
	if err != nil {
		t.Fatal(err)
	}

	if result.Valid {
		t.Error("should fail with wrong public key")
	}

	found := false
	for _, s := range result.Steps {
		if s.Step == StepVerifySignature && !s.Passed {
			found = true
		}
	}
	if !found {
		t.Error("expected signature step to fail")
	}
}

func TestVerifyBundle_InvalidZip(t *testing.T) {
	reader := bytes.NewReader([]byte("not a zip"))
	result, err := VerifyBundle(reader, int64(reader.Len()), nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Valid {
		t.Error("should fail for invalid zip")
	}
}

func TestVerifyBundle_AllStepsPresent(t *testing.T) {
	reader, pub := testBundle(t)
	result, _ := VerifyBundle(reader, int64(reader.Len()), pub)

	expectedSteps := []StepName{
		StepParseBundle,
		StepVerifySignature,
		StepVerifyManifest,
		StepVerifyScopeHash,
		StepVerifyTSA,
		StepRebuildMerkle,
		StepVerifyProofs,
		StepVerifyFiles,
		StepVerifyBridge,
		StepVerifyDerivations,
		StepVerifyDependencies,
	}

	if len(result.Steps) != len(expectedSteps) {
		t.Errorf("expected %d steps, got %d", len(expectedSteps), len(result.Steps))
		for _, s := range result.Steps {
			t.Logf("  %s: passed=%v", s.Step, s.Passed)
		}
	}
}
