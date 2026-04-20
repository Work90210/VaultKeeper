package federation

import "time"

// Bundle path constants for the VKE1 ZIP structure.
const (
	BundlePrefix = "vkx/"

	PathVersion          = BundlePrefix + "version.json"
	PathInstanceIdentity = BundlePrefix + "instance-identity.json"
	PathScope            = BundlePrefix + "scope.json"
	PathManifest         = BundlePrefix + "exchange-manifest.json"
	PathSignature        = BundlePrefix + "exchange-signature.json"
	PathMerkleRoot       = BundlePrefix + "merkle-root.json"
	PathMerkleProofs     = BundlePrefix + "merkle-proofs/"
	PathCustodyBridge    = BundlePrefix + "custody-bridge.json"
	PathTSAToken         = BundlePrefix + "tsa-token.json"
	PathDerivations      = BundlePrefix + "derivations/"

	EvidencePrefix = "evidence/"
	CustodyPrefix  = "custody/"
	PathCustodyChain = CustodyPrefix + "chain.json"
)

// BundleVersion is the metadata stored in version.json.
type BundleVersion struct {
	Format    string    `json:"format"`
	CreatedAt time.Time `json:"created_at"`
}

// InstanceIdentityDoc is the sender's identity as stored in the bundle.
type InstanceIdentityDoc struct {
	InstanceID   string `json:"instance_id"`
	PublicKey    string `json:"public_key"`     // base64 Ed25519
	Fingerprint  string `json:"fingerprint"`
	WellKnownURL string `json:"well_known_url,omitempty"`
}

// ExchangeSignatureDoc holds the Ed25519 signature over the manifest hash.
type ExchangeSignatureDoc struct {
	Signature string `json:"signature"` // base64
	Algorithm string `json:"algorithm"` // "ed25519"
}

// MerkleRootDoc holds the Merkle root and tree metadata.
type MerkleRootDoc struct {
	Root       string `json:"root"`        // hex
	LeafCount  int    `json:"leaf_count"`
	Algorithm  string `json:"algorithm"`   // "sha256-domain-separated"
}

// MerkleProofDoc holds a per-item inclusion proof.
type MerkleProofDoc struct {
	EvidenceID string      `json:"evidence_id"`
	LeafHash   string      `json:"leaf_hash"` // hex
	Steps      []ProofStep `json:"steps"`
}

// CustodyBridgeDoc holds the sender's bridge event details.
type CustodyBridgeDoc struct {
	Action              string `json:"action"`
	ExchangeID          string `json:"exchange_id"`
	ManifestHash        string `json:"manifest_hash"`
	RecipientInstanceID string `json:"recipient_instance_id,omitempty"`
	ScopeHash           string `json:"scope_hash"`
	MerkleRoot          string `json:"merkle_root"`
	ScopeCardinality    int    `json:"scope_cardinality"`
}

// EvidenceFileEntry describes one evidence file in the bundle.
type EvidenceFileEntry struct {
	EvidenceID string `json:"evidence_id"`
	Filename   string `json:"filename"`
	SHA256     string `json:"sha256"`
	SizeBytes  int64  `json:"size_bytes"`
}
