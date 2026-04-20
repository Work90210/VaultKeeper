package federation

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"strings"
)

// BundleEvidence pairs an evidence descriptor with its file content
// reader for pack operations.
type BundleEvidence struct {
	Descriptor EvidenceDescriptor
	Filename   string
	SizeBytes  int64
	Content    io.Reader
}

// BundleCustodyEvents holds the relevant custody chain events for
// the disclosed evidence items.
type BundleCustodyEvents struct {
	Events json.RawMessage `json:"events"` // raw custody event array
}

// PackBundleInput holds all data needed to pack a VKE1 bundle.
type PackBundleInput struct {
	Manifest        ExchangeManifest
	Signature       []byte
	Identity        InstanceIdentityDoc
	MerkleTree      *MerkleTree
	Descriptors     []EvidenceDescriptor
	Evidence        []BundleEvidence
	CustodyEvents   json.RawMessage // raw JSON array of custody events
	TSAToken        json.RawMessage // raw TSA token JSON (optional)
	DerivationDocs  map[string]json.RawMessage // evidence_id → derivation record JSON
}

// PackBundle writes a VKE1 ZIP bundle to w from the given input.
// Evidence files are streamed — never buffered entirely in memory.
func PackBundle(w io.Writer, input PackBundleInput) error {
	zw := zip.NewWriter(w)
	defer zw.Close()

	// version.json
	if err := writeJSON(zw, PathVersion, BundleVersion{
		Format:    ProtocolVersionVKE1,
		CreatedAt: input.Manifest.CreatedAt,
	}); err != nil {
		return fmt.Errorf("write version.json: %w", err)
	}

	// instance-identity.json
	if err := writeJSON(zw, PathInstanceIdentity, input.Identity); err != nil {
		return fmt.Errorf("write instance-identity.json: %w", err)
	}

	// scope.json
	if err := writeJSON(zw, PathScope, input.Manifest.Scope); err != nil {
		return fmt.Errorf("write scope.json: %w", err)
	}

	// exchange-manifest.json
	if err := writeJSON(zw, PathManifest, input.Manifest); err != nil {
		return fmt.Errorf("write exchange-manifest.json: %w", err)
	}

	// exchange-signature.json
	if err := writeJSON(zw, PathSignature, ExchangeSignatureDoc{
		Signature: base64.StdEncoding.EncodeToString(input.Signature),
		Algorithm: "ed25519",
	}); err != nil {
		return fmt.Errorf("write exchange-signature.json: %w", err)
	}

	// merkle-root.json
	if err := writeJSON(zw, PathMerkleRoot, MerkleRootDoc{
		Root:      hex.EncodeToString(input.MerkleTree.Root),
		LeafCount: len(input.MerkleTree.Leaves),
		Algorithm: "sha256-domain-separated",
	}); err != nil {
		return fmt.Errorf("write merkle-root.json: %w", err)
	}

	// merkle-proofs/{evidence_id}.json
	sorted := SortDescriptors(input.Descriptors)
	for i, d := range sorted {
		proof, err := input.MerkleTree.Proof(i)
		if err != nil {
			return fmt.Errorf("generate proof for %s: %w", d.EvidenceID, err)
		}
		leafHash, err := LeafHash(d)
		if err != nil {
			return fmt.Errorf("leaf hash for %s: %w", d.EvidenceID, err)
		}
		proofDoc := MerkleProofDoc{
			EvidenceID: d.EvidenceID.String(),
			LeafHash:   hex.EncodeToString(leafHash),
			Steps:      proof,
		}
		proofPath := PathMerkleProofs + d.EvidenceID.String() + ".json"
		if err := writeJSON(zw, proofPath, proofDoc); err != nil {
			return fmt.Errorf("write proof for %s: %w", d.EvidenceID, err)
		}
	}

	// custody-bridge.json
	bridgeDoc := CustodyBridgeDoc{
		Action:       ActionDisclosedToInstance,
		ExchangeID:   input.Manifest.ExchangeID.String(),
		ManifestHash: input.Manifest.ManifestHash,
		ScopeHash:    input.Manifest.ScopeHash,
		MerkleRoot:   input.Manifest.MerkleRoot,
		ScopeCardinality: input.Manifest.ScopeCardinality,
	}
	if input.Manifest.RecipientInstanceID != nil {
		bridgeDoc.RecipientInstanceID = *input.Manifest.RecipientInstanceID
	}
	if err := writeJSON(zw, PathCustodyBridge, bridgeDoc); err != nil {
		return fmt.Errorf("write custody-bridge.json: %w", err)
	}

	// tsa-token.json (optional)
	if len(input.TSAToken) > 0 {
		if err := writeRaw(zw, PathTSAToken, input.TSAToken); err != nil {
			return fmt.Errorf("write tsa-token.json: %w", err)
		}
	}

	// derivations/{evidence_id}.json (optional)
	for evidenceID, doc := range input.DerivationDocs {
		derivPath := PathDerivations + evidenceID + ".json"
		if err := writeRaw(zw, derivPath, doc); err != nil {
			return fmt.Errorf("write derivation for %s: %w", evidenceID, err)
		}
	}

	// evidence/{evidence_id}/descriptor.json + content.bin
	for _, ev := range input.Evidence {
		evDir := EvidencePrefix + ev.Descriptor.EvidenceID.String() + "/"

		if err := writeJSON(zw, evDir+"descriptor.json", ev.Descriptor); err != nil {
			return fmt.Errorf("write descriptor for %s: %w", ev.Descriptor.EvidenceID, err)
		}

		fw, err := zw.Create(evDir + "content.bin")
		if err != nil {
			return fmt.Errorf("create content entry for %s: %w", ev.Descriptor.EvidenceID, err)
		}
		if _, err := io.Copy(fw, ev.Content); err != nil {
			return fmt.Errorf("write content for %s: %w", ev.Descriptor.EvidenceID, err)
		}
	}

	// custody/chain.json
	if len(input.CustodyEvents) > 0 {
		if err := writeRaw(zw, PathCustodyChain, input.CustodyEvents); err != nil {
			return fmt.Errorf("write custody chain: %w", err)
		}
	}

	return nil
}

// UnpackedBundle holds the parsed contents of a VKE1 bundle.
type UnpackedBundle struct {
	Version       BundleVersion
	Identity      InstanceIdentityDoc
	Scope         ScopeDescriptor
	Manifest      ExchangeManifest
	Signature     ExchangeSignatureDoc
	MerkleRoot    MerkleRootDoc
	MerkleProofs  map[string]MerkleProofDoc // evidence_id → proof
	CustodyBridge CustodyBridgeDoc
	TSAToken      json.RawMessage
	Derivations   map[string]json.RawMessage // evidence_id → derivation
	Evidence      map[string]UnpackedEvidence // evidence_id → descriptor + content
	CustodyEvents json.RawMessage
}

// UnpackedEvidence holds a parsed evidence descriptor and its raw
// content bytes from the bundle.
type UnpackedEvidence struct {
	Descriptor EvidenceDescriptor
	Content    []byte
}

// UnpackBundle reads and validates a VKE1 ZIP bundle from r.
// The entire bundle is read into memory for validation — for large
// bundles, use streaming verification instead.
func UnpackBundle(r io.ReaderAt, size int64) (*UnpackedBundle, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, fmt.Errorf("open zip: %w", err)
	}

	bundle := &UnpackedBundle{
		MerkleProofs: make(map[string]MerkleProofDoc),
		Derivations:  make(map[string]json.RawMessage),
		Evidence:     make(map[string]UnpackedEvidence),
	}

	for _, f := range zr.File {
		// Sanitize: reject path traversal in zip entries.
		cleanName := path.Clean(f.Name)
		if strings.HasPrefix(cleanName, "/") || strings.HasPrefix(cleanName, "..") || strings.Contains(cleanName, "/../") {
			return nil, fmt.Errorf("path traversal detected in zip entry: %q", f.Name)
		}
		f.Name = cleanName

		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("open %s: %w", f.Name, err)
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", f.Name, err)
		}

		switch {
		case f.Name == PathVersion:
			if err := json.Unmarshal(data, &bundle.Version); err != nil {
				return nil, fmt.Errorf("parse version.json: %w", err)
			}
		case f.Name == PathInstanceIdentity:
			if err := json.Unmarshal(data, &bundle.Identity); err != nil {
				return nil, fmt.Errorf("parse instance-identity.json: %w", err)
			}
		case f.Name == PathScope:
			if err := json.Unmarshal(data, &bundle.Scope); err != nil {
				return nil, fmt.Errorf("parse scope.json: %w", err)
			}
		case f.Name == PathManifest:
			if err := json.Unmarshal(data, &bundle.Manifest); err != nil {
				return nil, fmt.Errorf("parse exchange-manifest.json: %w", err)
			}
		case f.Name == PathSignature:
			if err := json.Unmarshal(data, &bundle.Signature); err != nil {
				return nil, fmt.Errorf("parse exchange-signature.json: %w", err)
			}
		case f.Name == PathMerkleRoot:
			if err := json.Unmarshal(data, &bundle.MerkleRoot); err != nil {
				return nil, fmt.Errorf("parse merkle-root.json: %w", err)
			}
		case f.Name == PathCustodyBridge:
			if err := json.Unmarshal(data, &bundle.CustodyBridge); err != nil {
				return nil, fmt.Errorf("parse custody-bridge.json: %w", err)
			}
		case f.Name == PathTSAToken:
			bundle.TSAToken = data
		case f.Name == PathCustodyChain:
			bundle.CustodyEvents = data
		case strings.HasPrefix(f.Name, PathMerkleProofs):
			var proof MerkleProofDoc
			if err := json.Unmarshal(data, &proof); err != nil {
				return nil, fmt.Errorf("parse %s: %w", f.Name, err)
			}
			bundle.MerkleProofs[proof.EvidenceID] = proof
		case strings.HasPrefix(f.Name, PathDerivations):
			evidenceID := strings.TrimSuffix(path.Base(f.Name), ".json")
			bundle.Derivations[evidenceID] = data
		case strings.HasPrefix(f.Name, EvidencePrefix) && strings.HasSuffix(f.Name, "/descriptor.json"):
			evidenceID := extractEvidenceID(f.Name)
			ev := bundle.Evidence[evidenceID]
			if err := json.Unmarshal(data, &ev.Descriptor); err != nil {
				return nil, fmt.Errorf("parse %s: %w", f.Name, err)
			}
			bundle.Evidence[evidenceID] = ev
		case strings.HasPrefix(f.Name, EvidencePrefix) && strings.HasSuffix(f.Name, "/content.bin"):
			evidenceID := extractEvidenceID(f.Name)
			ev := bundle.Evidence[evidenceID]
			ev.Content = data
			bundle.Evidence[evidenceID] = ev
		}
	}

	// Validate required files are present.
	if bundle.Version.Format == "" {
		return nil, fmt.Errorf("missing %s", PathVersion)
	}
	if bundle.Version.Format != ProtocolVersionVKE1 {
		return nil, fmt.Errorf("unsupported bundle format: %q", bundle.Version.Format)
	}
	if bundle.Identity.InstanceID == "" {
		return nil, fmt.Errorf("missing %s", PathInstanceIdentity)
	}
	if bundle.Manifest.ManifestHash == "" {
		return nil, fmt.Errorf("missing %s", PathManifest)
	}
	if bundle.Signature.Algorithm == "" {
		return nil, fmt.Errorf("missing %s", PathSignature)
	}
	if bundle.MerkleRoot.Root == "" {
		return nil, fmt.Errorf("missing %s", PathMerkleRoot)
	}

	return bundle, nil
}

// extractEvidenceID extracts the evidence UUID from a path like
// "evidence/{uuid}/descriptor.json".
func extractEvidenceID(filePath string) string {
	parts := strings.Split(filePath, "/")
	if len(parts) >= 2 {
		return parts[1]
	}
	return ""
}

func writeJSON(zw *zip.Writer, name string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fw, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = fw.Write(data)
	return err
}

func writeRaw(zw *zip.Writer, name string, data []byte) error {
	fw, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = fw.Write(data)
	return err
}

// BundleToReaderAt creates an io.ReaderAt from a packed bundle for
// testing purposes. Packs the bundle into a buffer and returns a
// bytes.Reader.
func BundleToReaderAt(input PackBundleInput) (*bytes.Reader, error) {
	var buf bytes.Buffer
	if err := PackBundle(&buf, input); err != nil {
		return nil, err
	}
	return bytes.NewReader(buf.Bytes()), nil
}
