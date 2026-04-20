// Package vkverify provides offline verification of VKE1 evidence
// exchange bundles. It has zero database or network dependencies and
// is designed to be importable by the standalone vkverify binary.
//
// Verification order (fail-fast):
//  1. Parse bundle structure, validate VKE1 version
//  2. Verify manifest signature (Ed25519)
//  3. Verify manifest hash (recompute from manifest fields)
//  4. Verify scope hash (recompute from scope descriptor)
//  5. Rebuild Merkle tree from all evidence descriptors, compare root
//  6. Verify each item's Merkle inclusion proof
//  7. Stream-verify each evidence file's SHA-256 against descriptor
//  8. Verify custody bridge event hash consistency
//  9. Verify derivation commitments for redacted items
package vkverify

import (
	"archive/zip"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
	"time"
)

// StepName identifies a verification step.
type StepName string

const (
	StepParseBundle     StepName = "parse_bundle"
	StepVerifySignature StepName = "verify_signature"
	StepVerifyManifest  StepName = "verify_manifest_hash"
	StepVerifyScopeHash StepName = "verify_scope_hash"
	StepRebuildMerkle   StepName = "rebuild_merkle_tree"
	StepVerifyProofs    StepName = "verify_merkle_proofs"
	StepVerifyFiles     StepName = "verify_file_hashes"
	StepVerifyTSA       StepName = "verify_tsa_token"
	StepVerifyBridge    StepName = "verify_custody_bridge"
	StepVerifyDerivations StepName = "verify_derivations"
	StepVerifyDependencies StepName = "verify_dependencies"
)

// StepResult records the outcome of one verification step.
type StepResult struct {
	Step    StepName `json:"step"`
	Passed  bool     `json:"passed"`
	Detail  string   `json:"detail,omitempty"`
	Error   string   `json:"error,omitempty"`
}

// VerificationResult is the complete verification report.
type VerificationResult struct {
	Valid              bool         `json:"valid"`
	Steps              []StepResult `json:"steps"`
	BundleFormat       string       `json:"bundle_format"`
	SenderInstanceID   string       `json:"sender_instance_id"`
	SenderFingerprint  string       `json:"sender_fingerprint"`
	ExchangeID         string       `json:"exchange_id"`
	EvidenceCount      int          `json:"evidence_count"`
	VerifiedAt         time.Time    `json:"verified_at"`
}

// VerifyBundle performs offline verification of a VKE1 bundle.
// The publicKey parameter is the sender's Ed25519 public key (32 bytes).
// Verification is fail-fast: steps execute in order and stop on first failure.
func VerifyBundle(r io.ReaderAt, size int64, publicKey ed25519.PublicKey) (*VerificationResult, error) {
	result := &VerificationResult{
		Valid:      true,
		VerifiedAt: time.Now().UTC(),
	}

	// Step 1: Parse bundle
	bundle, err := parseBundle(r, size)
	if err != nil {
		result.fail(StepParseBundle, "", err.Error())
		return result, nil
	}
	result.pass(StepParseBundle, fmt.Sprintf("VKE1 bundle with %d evidence items", len(bundle.evidence)))
	result.BundleFormat = bundle.version.Format
	result.SenderInstanceID = bundle.identity.InstanceID
	result.SenderFingerprint = bundle.identity.Fingerprint
	result.ExchangeID = bundle.manifest.ExchangeID
	result.EvidenceCount = len(bundle.evidence)

	// Step 2: Verify signature
	sigBytes, err := base64.StdEncoding.DecodeString(bundle.signature.Signature)
	if err != nil {
		result.fail(StepVerifySignature, "", fmt.Sprintf("decode signature: %v", err))
		return result, nil
	}
	manifestHashBytes, err := hex.DecodeString(bundle.manifest.ManifestHash)
	if err != nil {
		result.fail(StepVerifySignature, "", fmt.Sprintf("decode manifest hash: %v", err))
		return result, nil
	}
	if !ed25519.Verify(publicKey, manifestHashBytes, sigBytes) {
		result.fail(StepVerifySignature, "", "Ed25519 signature verification failed")
		return result, nil
	}
	result.pass(StepVerifySignature, "Ed25519 signature valid")

	// Step 3: Verify manifest hash
	recomputedHash, err := computeManifestHash(bundle.manifest)
	if err != nil {
		result.fail(StepVerifyManifest, "", fmt.Sprintf("recompute manifest hash: %v", err))
		return result, nil
	}
	if !constantTimeHexCompare(recomputedHash, bundle.manifest.ManifestHash) {
		result.fail(StepVerifyManifest, "", fmt.Sprintf("manifest hash mismatch: computed %s, declared %s", recomputedHash, bundle.manifest.ManifestHash))
		return result, nil
	}
	result.pass(StepVerifyManifest, "manifest hash verified")

	// Step 4: Verify scope hash
	recomputedScopeHash, err := computeScopeHash(bundle.manifest.Scope)
	if err != nil {
		result.fail(StepVerifyScopeHash, "", fmt.Sprintf("recompute scope hash: %v", err))
		return result, nil
	}
	if !constantTimeHexCompare(recomputedScopeHash, bundle.manifest.ScopeHash) {
		result.fail(StepVerifyScopeHash, "", fmt.Sprintf("scope hash mismatch: computed %s, declared %s", recomputedScopeHash, bundle.manifest.ScopeHash))
		return result, nil
	}
	result.pass(StepVerifyScopeHash, "scope hash verified")

	// Step 5: Verify TSA token (structural check only)
	if bundle.tsaToken != nil {
		var tsaDoc struct {
			ManifestHash string `json:"manifest_hash"`
		}
		if err := json.Unmarshal(bundle.tsaToken, &tsaDoc); err != nil {
			result.fail(StepVerifyTSA, "", fmt.Sprintf("parse TSA token: %v", err))
			return result, nil
		}
		if tsaDoc.ManifestHash == "" {
			result.fail(StepVerifyTSA, "", "TSA token missing manifest_hash field")
			return result, nil
		}
		if !constantTimeHexCompare(tsaDoc.ManifestHash, bundle.manifest.ManifestHash) {
			result.fail(StepVerifyTSA, "", fmt.Sprintf("TSA token manifest_hash mismatch: token has %s, manifest has %s", tsaDoc.ManifestHash, bundle.manifest.ManifestHash))
			return result, nil
		}
		result.pass(StepVerifyTSA, "TSA token references correct manifest hash (RFC 3161 cryptographic verification requires external CA roots)")
	} else {
		result.pass(StepVerifyTSA, "no TSA token present")
	}

	// Step 6: Rebuild Merkle tree
	descriptors := sortDescriptors(bundle.manifest.DisclosedEvidence)
	leaves := make([][]byte, len(descriptors))
	for i, d := range descriptors {
		leaf, err := computeLeafHash(d)
		if err != nil {
			result.fail(StepRebuildMerkle, "", fmt.Sprintf("leaf hash for %s: %v", d.EvidenceID, err))
			return result, nil
		}
		leaves[i] = leaf
	}
	tree, err := buildMerkleTree(leaves)
	if err != nil {
		result.fail(StepRebuildMerkle, "", fmt.Sprintf("build tree: %v", err))
		return result, nil
	}
	declaredRoot, err := hex.DecodeString(bundle.manifest.MerkleRoot)
	if err != nil {
		result.fail(StepRebuildMerkle, "", fmt.Sprintf("decode declared root: %v", err))
		return result, nil
	}
	if subtle.ConstantTimeCompare(tree.root, declaredRoot) != 1 {
		result.fail(StepRebuildMerkle, "", fmt.Sprintf("merkle root mismatch: computed %s, declared %s",
			hex.EncodeToString(tree.root), bundle.manifest.MerkleRoot))
		return result, nil
	}
	result.pass(StepRebuildMerkle, fmt.Sprintf("merkle root verified over %d leaves", len(leaves)))

	// Step 6: Verify Merkle inclusion proofs
	for i, d := range descriptors {
		proofDoc, ok := bundle.merkleProofs[d.EvidenceID]
		if !ok {
			result.fail(StepVerifyProofs, "", fmt.Sprintf("missing proof for %s", d.EvidenceID))
			return result, nil
		}
		if !verifyProof(leaves[i], proofDoc.Steps, tree.root) {
			result.fail(StepVerifyProofs, "", fmt.Sprintf("proof verification failed for %s", d.EvidenceID))
			return result, nil
		}
	}
	result.pass(StepVerifyProofs, fmt.Sprintf("all %d inclusion proofs verified", len(descriptors)))

	// Step 7: Verify file SHA-256 hashes
	for _, d := range descriptors {
		ev, ok := bundle.evidence[d.EvidenceID]
		if !ok {
			result.fail(StepVerifyFiles, "", fmt.Sprintf("missing evidence file for %s", d.EvidenceID))
			return result, nil
		}
		h := sha256.Sum256(ev.content)
		fileHash := hex.EncodeToString(h[:])
		if !constantTimeHexCompare(fileHash, d.SHA256) {
			result.fail(StepVerifyFiles, "", fmt.Sprintf("file hash mismatch for %s: computed %s, declared %s", d.EvidenceID, fileHash, d.SHA256))
			return result, nil
		}
	}
	result.pass(StepVerifyFiles, fmt.Sprintf("all %d file hashes verified", len(descriptors)))

	// Step 8: Verify custody bridge consistency
	if bundle.custodyBridge.ManifestHash != bundle.manifest.ManifestHash {
		result.fail(StepVerifyBridge, "", "custody bridge manifest_hash does not match manifest")
		return result, nil
	}
	if bundle.custodyBridge.ScopeHash != bundle.manifest.ScopeHash {
		result.fail(StepVerifyBridge, "", "custody bridge scope_hash does not match manifest")
		return result, nil
	}
	if bundle.custodyBridge.MerkleRoot != bundle.manifest.MerkleRoot {
		result.fail(StepVerifyBridge, "", "custody bridge merkle_root does not match manifest")
		return result, nil
	}
	result.pass(StepVerifyBridge, "custody bridge consistent with manifest")

	// Step 9: Verify derivation commitments
	derivationCount := 0
	for evID, derivRaw := range bundle.derivations {
		var deriv derivationRecord
		if err := json.Unmarshal(derivRaw, &deriv); err != nil {
			result.fail(StepVerifyDerivations, "", fmt.Sprintf("parse derivation for %s: %v", evID, err))
			return result, nil
		}
		// Verify the commitment hash matches the canonical JSON of the record.
		recomputed, err := canonicalHash("VK:DERIVATION:v1", deriv)
		if err != nil {
			result.fail(StepVerifyDerivations, "", fmt.Sprintf("recompute derivation commitment for %s: %v", evID, err))
			return result, nil
		}
		if !constantTimeHexCompare(hex.EncodeToString(recomputed), deriv.DerivationCommitment) {
			result.fail(StepVerifyDerivations, "", fmt.Sprintf("derivation commitment mismatch for %s", evID))
			return result, nil
		}
		derivationCount++
	}
	if derivationCount > 0 {
		result.pass(StepVerifyDerivations, fmt.Sprintf("%d derivation commitments verified", derivationCount))
	} else {
		result.pass(StepVerifyDerivations, "no derivations to verify")
	}

	// Step 12: Verify dependency policy compliance
	policy := bundle.manifest.DependencyPolicy
	if policy == "none" || policy == "" {
		result.pass(StepVerifyDependencies, "dependency policy is \"none\", skipped")
	} else if policy == "direct_parent" || policy == "full_ancestry" {
		disclosedIDs := make(map[string]struct{}, len(bundle.manifest.DisclosedEvidence))
		for _, d := range bundle.manifest.DisclosedEvidence {
			disclosedIDs[d.EvidenceID] = struct{}{}
		}
		for _, d := range bundle.manifest.DisclosedEvidence {
			if d.ParentID == nil {
				continue
			}
			if _, ok := disclosedIDs[*d.ParentID]; !ok {
				result.fail(StepVerifyDependencies, "",
					fmt.Sprintf("dependency policy %q requires parent %s of evidence %s to be in disclosed_evidence, but it is missing",
						policy, *d.ParentID, d.EvidenceID))
				return result, nil
			}
		}
		result.pass(StepVerifyDependencies, fmt.Sprintf("all parent references satisfied under %q policy", policy))
	} else {
		result.pass(StepVerifyDependencies, fmt.Sprintf("unknown dependency policy %q, skipped", policy))
	}

	return result, nil
}

func (r *VerificationResult) pass(step StepName, detail string) {
	r.Steps = append(r.Steps, StepResult{Step: step, Passed: true, Detail: detail})
}

func (r *VerificationResult) fail(step StepName, detail, errMsg string) {
	r.Valid = false
	r.Steps = append(r.Steps, StepResult{Step: step, Passed: false, Detail: detail, Error: errMsg})
}

// --- internal types (mirror federation package, zero import) ---

type bundleContents struct {
	version       bundleVersion
	identity      instanceIdentityDoc
	manifest      exchangeManifest
	signature     exchangeSignatureDoc
	merkleRoot    merkleRootDoc
	merkleProofs  map[string]merkleProofDoc
	custodyBridge custodyBridgeDoc
	tsaToken      json.RawMessage // vkx/tsa-token.json (optional)
	derivations   map[string]json.RawMessage
	evidence      map[string]evidenceFile
}

type bundleVersion struct {
	Format    string    `json:"format"`
	CreatedAt time.Time `json:"created_at"`
}

type instanceIdentityDoc struct {
	InstanceID  string `json:"instance_id"`
	PublicKey   string `json:"public_key"`
	Fingerprint string `json:"fingerprint"`
}

type exchangeSignatureDoc struct {
	Signature string `json:"signature"`
	Algorithm string `json:"algorithm"`
}

type merkleRootDoc struct {
	Root      string `json:"root"`
	LeafCount int    `json:"leaf_count"`
}

type custodyBridgeDoc struct {
	ManifestHash string `json:"manifest_hash"`
	ScopeHash    string `json:"scope_hash"`
	MerkleRoot   string `json:"merkle_root"`
}

type merkleProofDoc struct {
	EvidenceID string      `json:"evidence_id"`
	LeafHash   string      `json:"leaf_hash"`
	Steps      []proofStep `json:"steps"`
}

type proofStep struct {
	SiblingHash []byte `json:"sibling_hash"`
	Position    string `json:"position"`
}

type evidenceFile struct {
	descriptor evidenceDescriptor
	content    []byte
}

type exchangeManifest struct {
	ProtocolVersion       string               `json:"protocol_version"`
	ExchangeID            string               `json:"exchange_id"`
	SenderInstanceID      string               `json:"sender_instance_id"`
	SenderKeyFingerprint  string               `json:"sender_key_fingerprint"`
	RecipientInstanceID   *string              `json:"recipient_instance_id,omitempty"`
	CreatedAt             time.Time            `json:"created_at"`
	Scope                 json.RawMessage      `json:"scope"`
	ScopeHash             string               `json:"scope_hash"`
	ScopeCardinality      int                  `json:"scope_cardinality"`
	MerkleRoot            string               `json:"merkle_root"`
	DependencyPolicy      string               `json:"dependency_policy"`
	DisclosedEvidence     []evidenceDescriptor `json:"disclosed_evidence"`
	SenderCustodyHead     string               `json:"sender_custody_head"`
	SenderBridgeEventHash string               `json:"sender_bridge_event_hash"`
	ManifestHash          string               `json:"manifest_hash"`
}

type evidenceDescriptor struct {
	EvidenceID           string     `json:"evidence_id"`
	CaseID               string     `json:"case_id"`
	Version              int        `json:"version"`
	SHA256               string     `json:"sha256"`
	Classification       string     `json:"classification"`
	Tags                 []string   `json:"tags"`
	SourceDate           *time.Time `json:"source_date,omitempty"`
	ParentID             *string    `json:"parent_id,omitempty"`
	DerivationCommitment *string    `json:"derivation_commitment,omitempty"`
	TSATokenHash         *string    `json:"tsa_token_hash,omitempty"`
}

type derivationRecord struct {
	Type                 string          `json:"type"`
	ParentEvidenceID     string          `json:"parent_evidence_id"`
	ChildEvidenceID      string          `json:"child_evidence_id"`
	ChildSHA256          string          `json:"child_sha256"`
	ParentHashCommitment *string         `json:"parent_hash_commitment,omitempty"`
	RedactionMethod      string          `json:"redaction_method"`
	RedactionPurpose     string          `json:"redaction_purpose"`
	ParametersCommitment string          `json:"parameters_commitment"`
	DerivationCommitment string          `json:"derivation_commitment"`
	CreatedAt            time.Time       `json:"created_at"`
	SignedByInstance      string         `json:"signed_by_instance"`
}

// --- bundle path constants ---
const (
	pathPrefix          = "vkx/"
	pathVersion         = pathPrefix + "version.json"
	pathInstanceIdentity = pathPrefix + "instance-identity.json"
	pathManifest        = pathPrefix + "exchange-manifest.json"
	pathSignature       = pathPrefix + "exchange-signature.json"
	pathMerkleRoot      = pathPrefix + "merkle-root.json"
	pathMerkleProofs    = pathPrefix + "merkle-proofs/"
	pathCustodyBridge   = pathPrefix + "custody-bridge.json"
	pathTSAToken        = pathPrefix + "tsa-token.json"
	pathDerivations     = pathPrefix + "derivations/"
	evidencePrefix      = "evidence/"
)

func parseBundle(r io.ReaderAt, size int64) (*bundleContents, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, fmt.Errorf("open zip: %w", err)
	}

	b := &bundleContents{
		merkleProofs: make(map[string]merkleProofDoc),
		derivations:  make(map[string]json.RawMessage),
		evidence:     make(map[string]evidenceFile),
	}

	for _, f := range zr.File {
		// Sanitize path: clean and reject traversal attempts.
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
		case f.Name == pathVersion:
			if err := json.Unmarshal(data, &b.version); err != nil {
				return nil, fmt.Errorf("parse version: %w", err)
			}
		case f.Name == pathInstanceIdentity:
			if err := json.Unmarshal(data, &b.identity); err != nil {
				return nil, fmt.Errorf("parse identity: %w", err)
			}
		case f.Name == pathManifest:
			if err := json.Unmarshal(data, &b.manifest); err != nil {
				return nil, fmt.Errorf("parse manifest: %w", err)
			}
		case f.Name == pathSignature:
			if err := json.Unmarshal(data, &b.signature); err != nil {
				return nil, fmt.Errorf("parse signature: %w", err)
			}
		case f.Name == pathMerkleRoot:
			if err := json.Unmarshal(data, &b.merkleRoot); err != nil {
				return nil, fmt.Errorf("parse merkle root: %w", err)
			}
		case f.Name == pathTSAToken:
			b.tsaToken = data
		case f.Name == pathCustodyBridge:
			if err := json.Unmarshal(data, &b.custodyBridge); err != nil {
				return nil, fmt.Errorf("parse custody bridge: %w", err)
			}
		case strings.HasPrefix(f.Name, pathMerkleProofs):
			var proof merkleProofDoc
			if err := json.Unmarshal(data, &proof); err != nil {
				return nil, fmt.Errorf("parse %s: %w", f.Name, err)
			}
			b.merkleProofs[proof.EvidenceID] = proof
		case strings.HasPrefix(f.Name, pathDerivations):
			evID := strings.TrimSuffix(path.Base(f.Name), ".json")
			b.derivations[evID] = data
		case strings.HasPrefix(f.Name, evidencePrefix) && strings.HasSuffix(f.Name, "/descriptor.json"):
			evID := extractEvID(f.Name)
			ef := b.evidence[evID]
			if err := json.Unmarshal(data, &ef.descriptor); err != nil {
				return nil, fmt.Errorf("parse %s: %w", f.Name, err)
			}
			b.evidence[evID] = ef
		case strings.HasPrefix(f.Name, evidencePrefix) && strings.HasSuffix(f.Name, "/content.bin"):
			evID := extractEvID(f.Name)
			ef := b.evidence[evID]
			ef.content = data
			b.evidence[evID] = ef
		}
	}

	if b.version.Format != "VKE1" {
		return nil, fmt.Errorf("unsupported or missing bundle format: %q", b.version.Format)
	}

	return b, nil
}

func extractEvID(filePath string) string {
	parts := strings.Split(filePath, "/")
	if len(parts) >= 2 {
		return parts[1]
	}
	return ""
}

// --- crypto primitives (self-contained, no imports from internal/) ---

func canonicalJSON(v any) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var tree any
	if err := json.Unmarshal(raw, &tree); err != nil {
		return nil, err
	}
	return json.Marshal(sortKeysRecursive(tree))
}

func sortKeysRecursive(v any) any {
	switch val := v.(type) {
	case map[string]any:
		sorted := make(map[string]any, len(val))
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			sorted[k] = sortKeysRecursive(val[k])
		}
		return sorted
	case []any:
		result := make([]any, len(val))
		for i, item := range val {
			result[i] = sortKeysRecursive(item)
		}
		return result
	default:
		return val
	}
}

func canonicalHash(domain string, v any) ([]byte, error) {
	canonical, err := canonicalJSON(v)
	if err != nil {
		return nil, err
	}
	h := sha256.New()
	h.Write([]byte(domain))
	h.Write(canonical)
	return h.Sum(nil), nil
}

func computeLeafHash(d evidenceDescriptor) ([]byte, error) {
	return canonicalHash("VK:MERKLE:LEAF:v1", d)
}

func computeManifestHash(m exchangeManifest) (string, error) {
	forHash := struct {
		ProtocolVersion       string               `json:"protocol_version"`
		ExchangeID            string               `json:"exchange_id"`
		SenderInstanceID      string               `json:"sender_instance_id"`
		SenderKeyFingerprint  string               `json:"sender_key_fingerprint"`
		RecipientInstanceID   *string              `json:"recipient_instance_id,omitempty"`
		CreatedAt             time.Time            `json:"created_at"`
		Scope                 json.RawMessage      `json:"scope"`
		ScopeHash             string               `json:"scope_hash"`
		ScopeCardinality      int                  `json:"scope_cardinality"`
		MerkleRoot            string               `json:"merkle_root"`
		DependencyPolicy      string               `json:"dependency_policy"`
		DisclosedEvidence     []evidenceDescriptor `json:"disclosed_evidence"`
		SenderCustodyHead     string               `json:"sender_custody_head"`
		SenderBridgeEventHash string               `json:"sender_bridge_event_hash"`
	}{
		ProtocolVersion:       m.ProtocolVersion,
		ExchangeID:            m.ExchangeID,
		SenderInstanceID:      m.SenderInstanceID,
		SenderKeyFingerprint:  m.SenderKeyFingerprint,
		RecipientInstanceID:   m.RecipientInstanceID,
		CreatedAt:             m.CreatedAt,
		Scope:                 m.Scope,
		ScopeHash:             m.ScopeHash,
		ScopeCardinality:      m.ScopeCardinality,
		MerkleRoot:            m.MerkleRoot,
		DependencyPolicy:      m.DependencyPolicy,
		DisclosedEvidence:     m.DisclosedEvidence,
		SenderCustodyHead:     m.SenderCustodyHead,
		SenderBridgeEventHash: m.SenderBridgeEventHash,
	}
	hash, err := canonicalHash("VK:MANIFEST:v1", forHash)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(hash), nil
}

func computeScopeHash(scopeRaw json.RawMessage) (string, error) {
	var scope any
	if err := json.Unmarshal(scopeRaw, &scope); err != nil {
		return "", err
	}
	hash, err := canonicalHash("VK:SCOPE:v1", scope)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(hash), nil
}

func sortDescriptors(descs []evidenceDescriptor) []evidenceDescriptor {
	sorted := make([]evidenceDescriptor, len(descs))
	copy(sorted, descs)
	sort.Slice(sorted, func(i, j int) bool {
		ki := fmt.Sprintf("%s:%d", sorted[i].EvidenceID, sorted[i].Version)
		kj := fmt.Sprintf("%s:%d", sorted[j].EvidenceID, sorted[j].Version)
		return ki < kj
	})
	return sorted
}

// --- Merkle tree (self-contained) ---

type merkleTree struct {
	root       []byte
	leaves     [][]byte
	layers     [][]byte
	layerSizes []int
}

func buildMerkleTree(leaves [][]byte) (*merkleTree, error) {
	if len(leaves) == 0 {
		return nil, fmt.Errorf("cannot build merkle tree from zero leaves")
	}

	current := make([][]byte, len(leaves))
	copy(current, leaves)

	allLayers := [][]byte{}
	layerSizes := []int{}

	allLayers = append(allLayers, current...)
	layerSizes = append(layerSizes, len(current))

	for len(current) > 1 {
		next := make([][]byte, 0, (len(current)+1)/2)
		for i := 0; i < len(current); i += 2 {
			left := current[i]
			right := left
			if i+1 < len(current) {
				right = current[i+1]
			}
			next = append(next, merkleNodeHash(left, right))
		}
		allLayers = append(allLayers, next...)
		layerSizes = append(layerSizes, len(next))
		current = next
	}

	return &merkleTree{
		root:       current[0],
		leaves:     leaves,
		layers:     allLayers,
		layerSizes: layerSizes,
	}, nil
}

func merkleNodeHash(left, right []byte) []byte {
	h := sha256.New()
	h.Write([]byte("VK:MERKLE:NODE:v1"))
	h.Write(left)
	h.Write(right)
	return h.Sum(nil)
}

func verifyProof(leaf []byte, proof []proofStep, expectedRoot []byte) bool {
	current := leaf
	for _, step := range proof {
		if step.Position == "left" {
			current = merkleNodeHash(step.SiblingHash, current)
		} else {
			current = merkleNodeHash(current, step.SiblingHash)
		}
	}
	return subtle.ConstantTimeCompare(current, expectedRoot) == 1
}

// constantTimeHexCompare decodes two hex strings and compares them
// in constant time to prevent timing oracle attacks.
func constantTimeHexCompare(a, b string) bool {
	aBytes, err := hex.DecodeString(a)
	if err != nil {
		return false
	}
	bBytes, err := hex.DecodeString(b)
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(aBytes, bBytes) == 1
}
