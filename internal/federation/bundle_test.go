package federation

import (
	"archive/zip"
	"bytes"
	"encoding/hex"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
)

func testBundleInput(t *testing.T) PackBundleInput {
	t.Helper()

	caseID := uuid.New()
	evID := uuid.New()

	descriptor := EvidenceDescriptor{
		EvidenceID:     evID,
		CaseID:         caseID,
		Version:        1,
		SHA256:         "aabbccdd",
		Classification: "public",
		Tags:           []string{"test"},
	}

	tree, err := BuildScopedMerkleTree([]EvidenceDescriptor{descriptor})
	if err != nil {
		t.Fatal(err)
	}

	scope := ScopeDescriptor{
		ScopeVersion: 1,
		CaseID:       caseID,
		Predicate:    ScopePredicate{Op: "eq", Field: "tags", Value: json.RawMessage(`"test"`)},
		Snapshot:     SnapshotAnchor{AsOfCustodyHash: "abc", EvaluatedAt: time.Now().UTC()},
	}

	manifest, err := BuildExchangeManifest(
		uuid.New(), "test-instance", "sha256:fp", nil,
		scope, []EvidenceDescriptor{descriptor}, tree.Root,
		DependencyPolicyNone, "custody-head", "bridge-hash",
	)
	if err != nil {
		t.Fatal(err)
	}

	content := []byte("evidence file content here")

	return PackBundleInput{
		Manifest:  manifest,
		Signature: []byte("fake-sig-for-testing"),
		Identity: InstanceIdentityDoc{
			InstanceID:  "test-instance",
			PublicKey:   "base64pubkey",
			Fingerprint: "sha256:fp",
		},
		MerkleTree:  tree,
		Descriptors: []EvidenceDescriptor{descriptor},
		Evidence: []BundleEvidence{
			{
				Descriptor: descriptor,
				Filename:   "test.pdf",
				SizeBytes:  int64(len(content)),
				Content:    bytes.NewReader(content),
			},
		},
		CustodyEvents: json.RawMessage(`[{"action":"test"}]`),
	}
}

func TestPackUnpackRoundTrip(t *testing.T) {
	input := testBundleInput(t)

	reader, err := BundleToReaderAt(input)
	if err != nil {
		t.Fatalf("pack: %v", err)
	}

	bundle, err := UnpackBundle(reader, int64(reader.Len()))
	if err != nil {
		t.Fatalf("unpack: %v", err)
	}

	if bundle.Version.Format != ProtocolVersionVKE1 {
		t.Errorf("format = %q", bundle.Version.Format)
	}
	if bundle.Identity.InstanceID != "test-instance" {
		t.Errorf("instance_id = %q", bundle.Identity.InstanceID)
	}
	if bundle.Manifest.ManifestHash != input.Manifest.ManifestHash {
		t.Error("manifest hash mismatch")
	}
	if bundle.Signature.Algorithm != "ed25519" {
		t.Errorf("algorithm = %q", bundle.Signature.Algorithm)
	}
	if bundle.MerkleRoot.Root != hex.EncodeToString(input.MerkleTree.Root) {
		t.Error("merkle root mismatch")
	}

	evID := input.Descriptors[0].EvidenceID.String()
	ev, ok := bundle.Evidence[evID]
	if !ok {
		t.Fatalf("evidence %s not found in bundle", evID)
	}
	if string(ev.Content) != "evidence file content here" {
		t.Errorf("evidence content = %q", ev.Content)
	}
	if _, ok := bundle.MerkleProofs[evID]; !ok {
		t.Error("merkle proof not found for evidence")
	}
	if bundle.CustodyEvents == nil {
		t.Error("custody events missing")
	}
}

func TestUnpackBundle_MissingVersion(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	addZipJSON(t, zw, PathInstanceIdentity, InstanceIdentityDoc{InstanceID: "x"})
	zw.Close()

	reader := bytes.NewReader(buf.Bytes())
	_, err := UnpackBundle(reader, int64(reader.Len()))
	if err == nil {
		t.Error("expected error for missing version.json")
	}
}

func TestUnpackBundle_WrongFormat(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	addZipJSON(t, zw, PathVersion, BundleVersion{Format: "VKE99", CreatedAt: time.Now()})
	addZipJSON(t, zw, PathInstanceIdentity, InstanceIdentityDoc{InstanceID: "x"})
	addZipJSON(t, zw, PathManifest, ExchangeManifest{ManifestHash: "h"})
	addZipJSON(t, zw, PathSignature, ExchangeSignatureDoc{Algorithm: "ed25519"})
	addZipJSON(t, zw, PathMerkleRoot, MerkleRootDoc{Root: "r"})
	zw.Close()

	reader := bytes.NewReader(buf.Bytes())
	_, err := UnpackBundle(reader, int64(reader.Len()))
	if err == nil {
		t.Error("expected error for wrong format")
	}
}

func TestPackBundle_MultipleEvidence(t *testing.T) {
	caseID := uuid.New()
	descriptors := make([]EvidenceDescriptor, 3)
	evidenceItems := make([]BundleEvidence, 3)
	for i := range descriptors {
		descriptors[i] = EvidenceDescriptor{
			EvidenceID:     uuid.New(),
			CaseID:         caseID,
			Version:        1,
			SHA256:         "hash",
			Classification: "public",
			Tags:           []string{},
		}
		content := []byte("content-" + descriptors[i].EvidenceID.String())
		evidenceItems[i] = BundleEvidence{
			Descriptor: descriptors[i],
			Filename:   "file.bin",
			SizeBytes:  int64(len(content)),
			Content:    bytes.NewReader(content),
		}
	}

	tree, _ := BuildScopedMerkleTree(descriptors)
	scope := ScopeDescriptor{
		ScopeVersion: 1,
		CaseID:       caseID,
		Predicate:    ScopePredicate{Op: "eq", Field: "tags", Value: json.RawMessage(`"x"`)},
		Snapshot:     SnapshotAnchor{AsOfCustodyHash: "abc", EvaluatedAt: time.Now().UTC()},
	}
	manifest, _ := BuildExchangeManifest(
		uuid.New(), "inst", "sha256:fp", nil,
		scope, descriptors, tree.Root,
		DependencyPolicyNone, "ch", "bh",
	)

	input := PackBundleInput{
		Manifest:    manifest,
		Signature:   []byte("sig"),
		Identity:    InstanceIdentityDoc{InstanceID: "inst", PublicKey: "pk", Fingerprint: "fp"},
		MerkleTree:  tree,
		Descriptors: descriptors,
		Evidence:    evidenceItems,
	}

	reader, err := BundleToReaderAt(input)
	if err != nil {
		t.Fatal(err)
	}

	bundle, err := UnpackBundle(reader, int64(reader.Len()))
	if err != nil {
		t.Fatal(err)
	}

	if len(bundle.Evidence) != 3 {
		t.Errorf("expected 3 evidence items, got %d", len(bundle.Evidence))
	}
	if len(bundle.MerkleProofs) != 3 {
		t.Errorf("expected 3 merkle proofs, got %d", len(bundle.MerkleProofs))
	}
}

func addZipJSON(t *testing.T, zw *zip.Writer, name string, v any) {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	fw, err := zw.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	fw.Write(data)
}
