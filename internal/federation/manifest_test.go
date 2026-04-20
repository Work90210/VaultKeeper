package federation

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/vaultkeeper/vaultkeeper/internal/migration"
)

func testManifest() ExchangeManifest {
	caseID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	return ExchangeManifest{
		ProtocolVersion:      ProtocolVersionVKE1,
		ExchangeID:           uuid.MustParse("55555555-5555-5555-5555-555555555555"),
		SenderInstanceID:     "test-instance",
		SenderKeyFingerprint: "sha256:testfp",
		CreatedAt:            time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC),
		Scope: ScopeDescriptor{
			ScopeVersion: 1,
			CaseID:       caseID,
			Predicate:    ScopePredicate{Op: "eq", Field: "tags", Value: json.RawMessage(`"test"`)},
			Snapshot:     SnapshotAnchor{AsOfCustodyHash: "abc", EvaluatedAt: time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)},
		},
		ScopeHash:        "scopehash",
		ScopeCardinality: 2,
		MerkleRoot:       "merkleroot",
		DependencyPolicy: DependencyPolicyNone,
		DisclosedEvidence: []EvidenceDescriptor{
			{
				EvidenceID:     uuid.MustParse("66666666-6666-6666-6666-666666666666"),
				CaseID:         caseID,
				Version:        1,
				SHA256:         "filehash",
				Classification: "public",
				Tags:           []string{"test"},
			},
		},
		SenderCustodyHead:     "custodyhead",
		SenderBridgeEventHash: "bridgehash",
	}
}

func TestComputeManifestHash_Deterministic(t *testing.T) {
	m := testManifest()
	h1, err := ComputeManifestHash(m)
	if err != nil {
		t.Fatal(err)
	}
	h2, _ := ComputeManifestHash(m)

	if h1 != h2 {
		t.Errorf("non-deterministic: %s vs %s", h1, h2)
	}
	if len(h1) != 64 {
		t.Errorf("unexpected hash length: %d", len(h1))
	}
}

func TestComputeManifestHash_ExcludesManifestHash(t *testing.T) {
	m := testManifest()
	h1, _ := ComputeManifestHash(m)

	m.ManifestHash = "this_should_be_ignored"
	h2, _ := ComputeManifestHash(m)

	if h1 != h2 {
		t.Error("ManifestHash field should not affect computed hash")
	}
}

func TestComputeManifestHash_FieldChangeDetected(t *testing.T) {
	m := testManifest()
	h1, _ := ComputeManifestHash(m)

	m.ScopeCardinality = 999
	h2, _ := ComputeManifestHash(m)

	if h1 == h2 {
		t.Error("changing a field should change the hash")
	}
}

func TestSigningRoundTrip(t *testing.T) {
	signer, err := migration.LoadOrGenerate()
	if err != nil {
		t.Fatal(err)
	}

	ms := NewManifestSigner(signer)

	m := testManifest()
	hash, err := ComputeManifestHash(m)
	if err != nil {
		t.Fatal(err)
	}

	sig, err := SignManifestHex(ms, hash)
	if err != nil {
		t.Fatal(err)
	}

	valid, err := VerifyManifestHex(ms, hash, sig)
	if err != nil {
		t.Fatal(err)
	}
	if !valid {
		t.Error("valid signature rejected")
	}
}

func TestSigningRejectsTamperedManifest(t *testing.T) {
	signer, err := migration.LoadOrGenerate()
	if err != nil {
		t.Fatal(err)
	}

	ms := NewManifestSigner(signer)

	m := testManifest()
	hash, _ := ComputeManifestHash(m)
	sig, _ := SignManifestHex(ms, hash)

	// Tamper with the manifest.
	m.ScopeCardinality = 999
	tamperedHash, _ := ComputeManifestHash(m)

	valid, _ := VerifyManifestHex(ms, tamperedHash, sig)
	if valid {
		t.Error("tampered manifest should not verify")
	}
}

func TestFingerprint(t *testing.T) {
	signer, err := migration.LoadOrGenerate()
	if err != nil {
		t.Fatal(err)
	}

	ms := NewManifestSigner(signer)
	fp := ms.Fingerprint()

	if fp[:7] != "sha256:" {
		t.Errorf("fingerprint should start with 'sha256:', got %q", fp[:7])
	}
	if len(fp) < 10 {
		t.Error("fingerprint too short")
	}
}

func TestBuildExchangeManifest(t *testing.T) {
	caseID := uuid.New()
	scope := ScopeDescriptor{
		ScopeVersion: 1,
		CaseID:       caseID,
		Predicate:    ScopePredicate{Op: "eq", Field: "tags", Value: json.RawMessage(`"test"`)},
		Snapshot:     SnapshotAnchor{AsOfCustodyHash: "abc", EvaluatedAt: time.Now().UTC()},
	}
	descriptors := []EvidenceDescriptor{
		{EvidenceID: uuid.New(), CaseID: caseID, Version: 1, SHA256: "h1", Classification: "public", Tags: []string{}},
		{EvidenceID: uuid.New(), CaseID: caseID, Version: 1, SHA256: "h2", Classification: "public", Tags: []string{}},
	}
	tree, err := BuildScopedMerkleTree(descriptors)
	if err != nil {
		t.Fatal(err)
	}

	m, err := BuildExchangeManifest(
		uuid.New(),
		"sender-instance",
		"sha256:fp",
		nil,
		scope,
		descriptors,
		tree.Root,
		DependencyPolicyNone,
		"custody-head",
		"bridge-hash",
	)
	if err != nil {
		t.Fatal(err)
	}

	if m.ProtocolVersion != ProtocolVersionVKE1 {
		t.Errorf("protocol version = %s", m.ProtocolVersion)
	}
	if m.ManifestHash == "" {
		t.Error("manifest hash not computed")
	}
	if m.ScopeCardinality != 2 {
		t.Errorf("scope cardinality = %d, want 2", m.ScopeCardinality)
	}

	// Verify the manifest hash is correct.
	recomputed, _ := ComputeManifestHash(m)
	if recomputed != m.ManifestHash {
		t.Error("manifest hash does not match recomputed value")
	}
}
