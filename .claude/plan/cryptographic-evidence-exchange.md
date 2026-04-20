# Cryptographic Cross-Border Evidence Exchange with Selective Disclosure

**Phase:** 3 — Federation & Sovereignty
**Supersedes:** Sprint 18 federation section (retains per-case encryption as prerequisite)
**Goal:** A protocol where VaultKeeper Instance A can cryptographically hand an evidence subset to VaultKeeper Instance B with proof that the subset is authentic, complete relative to a declared scope, and unmodified — enabling chains like Bellingcat → Lighthouse → ICC list counsel → national prosecutor without trusting a central cloud.

---

## Reading Guide

**Sections 1-8** (Technical Solution) are the **protocol specification** — what VKE1 is and how it works cryptographically. **Steps 1-14** are the **implementation plan** — what to build and in what order. **Risks, Threat Model, and v2 Considerations** are **reference material** for security review and future planning.

---

## Task Type

- [x] Backend (Go) — protocol engine, crypto, verification
- [x] Frontend (Next.js) — peer management, scope builder, verification UI
- [x] Specification — VKE1 protocol spec, custody report format

---

## Design Principles

1. **Sovereignty by construction** — No central authority. Peer-to-peer trust. Each instance controls its own keys, data, and policy.
2. **Offline-first verification** — Any party can verify an exchange bundle without contacting the sender's instance. A judge in a disconnected courtroom can validate provenance.
3. **Legal explainability** — Every cryptographic operation must be describable in a legal brief. No exotic primitives. SHA-256, Ed25519, Merkle trees, RFC 3161 — all have legal precedent.
4. **Selective disclosure with completeness** — Sender proves "I gave you everything matching scope X" without revealing anything outside scope X.
5. **Non-repudiation** — Sender signs, receiver receipts. Neither can deny the exchange.
6. **Permissionless verification** — Nobody can prevent you from verifying a VKE1 bundle. Nobody can force you to trust someone else's verification. You trust only the cryptographic primitives and the binary you chose to run. Everything else is verified, not trusted. This is the Bitcoin Core model applied to evidence: the math is open, the tools are open, the process is open.

---

## Protocol Overview: VKE1

### Core Concept

A **VKE1 exchange** is a signed, self-contained bundle containing:
- A **scope declaration** — what the sender claims to have shared
- A **Merkle commitment** — cryptographic proof that the disclosed items are exactly the items matching the scope
- **Evidence files + metadata** — the actual content
- **Merkle inclusion proofs** — per-item proofs binding each item to the root
- **Custody bridge** — cryptographic link from sender's chain to receiver's chain
- **Instance identity** — sender's Ed25519 public key + fingerprint
- **RFC 3161 timestamp** — external time notarization on the exchange

### Trust Model

**Baseline: Manual fingerprint verification + key pinning**
- Institutions exchange Ed25519 public key fingerprints out-of-band (phone, legal letter, secure messenger, in-person ceremony)
- First contact: `.well-known/vaultkeeper-instance.json` discovery (TOFU optional, not default)
- Pinned keys persist in `peer_instances` table
- Key rotation: new key valid only if signed by old key OR manually re-verified
- Optional overlay: X.509 certificate chain for institutions that want PKI

**Trust states:** `untrusted` → `tofu_pending` → `manual_pinned` → `pki_verified` | `revoked`

### Fundamental Limitation: The Lying Source Problem

**The completeness proof guarantees "every item matching scope S at evaluation time is included." It does NOT guarantee the sender's database is truthful.** A malicious sender can delete evidence items *before* running the exchange — the scope evaluates correctly against the tampered database, the Merkle root is valid, and the receiver has no way to detect the omission.

The defense against this is layered:
1. **The sender's custody chain records all deletions.** Every `destroyed` or `deleted` event is logged with actor, timestamp, and legal authority citation. The exchange is anchored to the custody chain head (`as_of_custody_hash`) at evaluation time.
2. **If the sender deleted items, those deletions appear in the chain** — and the chain's integrity is independently verifiable (hash chain, RFC 3161 timestamps).
3. **A sender who wants to hide deletions must also tamper with their custody chain** — which requires breaking the hash chain, which is detectable by any party who has ever verified the chain at an earlier state.
4. **This ultimately depends on the custody chain's integrity**, which is the subject of the [formal verification plan](formal-verification-custody-engine.md). The federation protocol's completeness guarantee is only as strong as the underlying custody chain's tamper resistance.

**Concrete scenario:**

> Organization A has 50 evidence items tagged `ukraine-2024`. Before creating an exchange for Organization B, an insider at A deletes 3 items that implicate a politically connected individual. A then runs the exchange with scope `tags contains "ukraine-2024"`. The scope correctly evaluates to 47 items. The Merkle root is valid over those 47 items. B receives a cryptographically perfect bundle of 47 items and has no way to know 3 were deleted.
>
> **Detection path:** A's custody chain contains three `EVIDENCE_DESTROYED` events with timestamps prior to the exchange's `as_of_custody_hash`. If B (or any auditor) can obtain a copy of A's custody chain — either from A directly, or from any party who previously verified it — the deletions are visible. If A *also* tampered with the custody chain to remove the deletion events, the chain's hash integrity breaks, which is detectable by anyone who verified the chain at an earlier state (e.g., a previous exchange partner, or a TSA timestamp anchoring an earlier chain head).

**For legal proceedings:** The correct framing is "VKE1 proves the exchange is faithful to the sender's recorded state. If the sender's recorded state is itself fraudulent, that constitutes evidence tampering — a criminal offense in every jurisdiction where VaultKeeper operates. The protocol creates the audit trail that makes such tampering prosecutable."

This limitation MUST be documented prominently in the VKE1 specification under a "Security Considerations" section, **with the concrete scenario above**, so that a non-technical reader (judge, lawyer) can understand exactly what is and isn't protected. It is the question opposing counsel will ask.

---

## Technical Solution

### 1. Canonical Scope DSL

A strict, versioned, deterministic predicate language for declaring "what I'm sharing." No freeform SQL. No arbitrary expressions.

```go
type ScopeDescriptor struct {
    ScopeVersion int            `json:"scope_version"` // 1
    CaseID       uuid.UUID      `json:"case_id"`
    Predicate    ScopePredicate `json:"predicate"`
    Snapshot     SnapshotAnchor `json:"snapshot"`
}

type ScopePredicate struct {
    Op       string           `json:"op"`    // "and", "eq", "contains", "in", "gte", "lt"
    Field    string           `json:"field"` // v1: tags, source_date, classification, evidence_number, version
    Value    json.RawMessage  `json:"value,omitempty"`
    Children []ScopePredicate `json:"children,omitempty"` // for "and"
}

type SnapshotAnchor struct {
    AsOfCustodyHash string    `json:"as_of_custody_hash"` // head of custody chain at evaluation time
    EvaluatedAt     time.Time `json:"evaluated_at"`
}
```

**v1 supported fields:** `tags`, `source_date`, `classification`, `evidence_number`, `version`, `parent_id`, `retention_until`
**v1 supported operators:** `eq`, `contains`, `in`, `gte`, `lt`, `and`
**No `or` in v1** — eliminates normalization ambiguity. Express disjunctions as separate exchanges.

**Canonical form rules:**
- Children sorted lexicographically by `(field, op, canonical_json(value))`
- Timestamps in RFC 3339 UTC
- Tags lowercased, sorted
- All JSON uses RFC 8785 canonical form (sorted keys, no whitespace)

**Scope hash:** `SHA-256("VK:SCOPE:v1" || canonical_json(scope_descriptor))`

### 2. Evidence Descriptors

Leaf nodes in the Merkle tree. One per evidence item in the evaluated scope.

```go
type EvidenceDescriptor struct {
    EvidenceID           uuid.UUID  `json:"evidence_id"`
    CaseID               uuid.UUID  `json:"case_id"`
    Version              int        `json:"version"`
    SHA256               string     `json:"sha256"`
    Classification       string     `json:"classification"`
    Tags                 []string   `json:"tags"`      // sorted, lowercased
    SourceDate           *time.Time `json:"source_date,omitempty"`
    ParentID             *uuid.UUID `json:"parent_id,omitempty"`
    DerivationCommitment *string    `json:"derivation_commitment,omitempty"`
    TSATokenHash         *string    `json:"tsa_token_hash,omitempty"` // SHA-256 of TSA token bytes
}
```

**Leaf hash:** `SHA-256("VK:MERKLE:LEAF:v1" || canonical_json(descriptor))`
**Sort key:** `evidence_id + ":" + version` (lexicographic)

### 3. Scoped Merkle Tree

Per-disclosure tree over the **entire scope-matching set** (not just the disclosed subset — the disclosed subset IS the scope-matching set).

```
func BuildScopedMerkleTree(descriptors []EvidenceDescriptor) *MerkleTree:
    sort descriptors by (evidence_id, version)
    leaves = [LeafHash(d) for d in descriptors]
    return BuildBinaryMerkleTree(leaves)

Node hash: SHA-256("VK:MERKLE:NODE:v1" || left || right)
Odd leaf: duplicated as right sibling
```

**Completeness proof:** The scope hash + Merkle root + scope cardinality together prove: "exactly N items matched scope S, and here they are." If sender omits an item, the Merkle root won't match what an auditor computes from the declared scope against the sender's data.

**Per-item inclusion proofs:** Each disclosed item carries a Merkle proof (array of `{sibling_hash, position}` steps) from its leaf to the root. Receiver can verify each item belongs to the committed set.

### 4. Exchange Manifest

The signed object. This is what the sender's Ed25519 key attests to.

```go
type ExchangeManifest struct {
    ProtocolVersion      string               `json:"protocol_version"` // "VKE1"
    ExchangeID           uuid.UUID            `json:"exchange_id"`
    SenderInstanceID     string               `json:"sender_instance_id"`
    SenderKeyFingerprint string               `json:"sender_key_fingerprint"`
    RecipientInstanceID  *string              `json:"recipient_instance_id,omitempty"` // see Recipient Binding Semantics below
    CreatedAt            time.Time            `json:"created_at"`
    Scope                ScopeDescriptor      `json:"scope"`
    ScopeHash            string               `json:"scope_hash"`
    ScopeCardinality     int                  `json:"scope_cardinality"`
    MerkleRoot           string               `json:"merkle_root"`
    DependencyPolicy     string               `json:"dependency_policy"` // none, direct_parent, full_ancestry
    DisclosedEvidence    []EvidenceDescriptor `json:"disclosed_evidence"`
    SenderCustodyHead    string               `json:"sender_custody_head"`
    SenderBridgeEventHash string              `json:"sender_bridge_event_hash"`
    ManifestHash         string               `json:"manifest_hash"` // computed last
}
```

**Manifest hash:** `SHA-256("VK:MANIFEST:v1" || canonical_json(manifest_without_manifest_hash))`
**Signature:** `Ed25519.Sign(instance_private_key, manifest_hash_bytes)`

**Recipient Binding Semantics:**
- `recipient_instance_id` is **optional**. When omitted, the bundle is an **unbound exchange** — verifiable by anyone who trusts the sender's public key. This supports public disclosures (e.g., Bellingcat publishing evidence).
- When present, recipient binding expresses **sender intent**, not access control. The signature covers the manifest (including recipient_instance_id), so tampering with the recipient field invalidates the signature — but any party with the sender's public key can still verify the bundle's cryptographic integrity.
- Receiver verification SHOULD check that `recipient_instance_id` matches their own instance ID (if present), and SHOULD warn if it doesn't match — but this is a policy check, not a cryptographic one.
- A stolen bundle with a recipient binding is still cryptographically valid. The binding proves "sender intended this for recipient X" — useful as evidence of directed disclosure, but not as DRM.

### 5. Custody Bridge

Two paired events create a cryptographic edge between independent custody chains.

**Sender side (Instance A):**
```
Action: DISCLOSED_TO_INSTANCE
Detail: {
    exchange_id, manifest_hash, recipient_instance_id, recipient_pubkey_fingerprint,
    disclosed_evidence_ids, scope_hash, merkle_root, scope_cardinality
}
→ Produces bridge event with hash_value computed by existing chain algorithm
```

**Receiver side (Instance B):**
```
Action: IMPORTED_FROM_INSTANCE
Detail: {
    exchange_id, manifest_hash, sender_instance_id, sender_pubkey_fingerprint,
    sender_custody_head, sender_bridge_event_hash, imported_evidence_ids
}
→ sender_bridge_event_hash = hash of A's DISCLOSED_TO_INSTANCE event
→ Creates cryptographic graph edge between chains
```

**Multi-hop provenance:** When B later shares a subset to C, C's bundle references B's custody chain which contains the IMPORTED_FROM_INSTANCE event referencing A. Provenance is traceable: C → B → A.

### 6. Redacted Evidence Provenance

For redacted versions of evidence (common in disclosure — witness protection, national security):

```go
type DerivationRecord struct {
    Type                  string    `json:"type"` // "redaction"
    ParentEvidenceID      uuid.UUID `json:"parent_evidence_id"`
    ChildEvidenceID       uuid.UUID `json:"child_evidence_id"`
    ChildSHA256           string    `json:"child_sha256"`
    ParentHashCommitment  *string   `json:"parent_hash_commitment,omitempty"` // optional: SHA-256 of original
    RedactionMethod       string    `json:"redaction_method"` // "pdf-redact-v1", "image-blur-v1"
    RedactionPurpose      string    `json:"redaction_purpose"`
    ParametersCommitment  string    `json:"parameters_commitment"` // SHA-256 of redaction params
    CreatedAt             time.Time `json:"created_at"`
    SignedByInstance       string    `json:"signed_by_instance"`
}
```

**Derivation commitment:** `SHA-256("VK:DERIVATION:v1" || canonical_json(derivation_record))`

This is an **attested provenance proof**, not a zero-knowledge proof. The sender asserts "this redacted file derives from evidence X under my custody." The assertion is signed and timestamped. This is operationally realistic and legally sufficient — courts accept attested derivation, not just mathematical proofs.

### 7. Bundle Format

ZIP container with strict internal structure:

```
vaultkeeper-exchange-{exchange_id}.zip
├── vkx/
│   ├── version.json                          # {"format":"VKE1","created_at":"..."}
│   ├── instance-identity.json                # sender pubkey, fingerprint, well_known_url
│   ├── scope.json                            # canonical scope descriptor
│   ├── exchange-manifest.json                # the signed manifest
│   ├── exchange-signature.json               # {"signature":"base64...","algorithm":"ed25519"}
│   ├── merkle-root.json                      # root hash + tree metadata
│   ├── merkle-proofs/
│   │   └── {evidence_id}.json                # per-item inclusion proof
│   ├── custody-bridge.json                   # sender's bridge event details
│   ├── tsa-token.json                        # RFC 3161 timestamp on manifest hash
│   └── derivations/
│       └── {evidence_id}.json                # derivation records for redacted items
├── evidence/
│   ├── {evidence_id}/
│   │   ├── descriptor.json                   # evidence metadata
│   │   ├── content.bin                       # the evidence file
│   │   └── tsa-token.bin                     # original TSA token for this evidence
│   └── ...
└── custody/
    └── chain.json                            # relevant custody events for disclosed items
```

### 8. Instance Discovery

**`.well-known/vaultkeeper-instance.json`:**
```json
{
    "protocol": "vaultkeeper-instance/v1",
    "instance_id": "lighthouse-reports",
    "display_name": "Lighthouse Reports",
    "ed25519_public_key": "base64...",
    "key_fingerprint": "sha256:...",
    "supported_bundle_versions": ["VKE1"],
    "federation_endpoint": "/api/federation/receive",
    "key_rotation": {
        "previous_key_fingerprint": null,
        "rotation_statement_url": null
    }
}
```

**Key rotation document:**
```json
{
    "type": "key_rotation",
    "old_key_fingerprint": "sha256:...",
    "new_key_fingerprint": "sha256:...",
    "effective_at": "2026-04-16T00:00:00Z",
    "signature_old_key": "base64...",
    "signature_new_key": "base64..."
}
```

**Key rotation security limitation:** Dual-signature rotation (old key signs new, new key signs statement) does NOT protect against an attacker who has already compromised the current key. The attacker possesses the old key and can generate any new key, so they can produce a valid rotation document unilaterally. **Peers SHOULD re-verify rotated keys through an independent channel** (phone call, in-person ceremony). The spec MUST state this explicitly. This is inherent to all TOFU/pinning systems — rotation ceremonies require out-of-band trust re-establishment when the current key may be compromised. HSM-backed keys (recommended for production) reduce the attack surface to physical access.

---

## Implementation Steps

### Step 1: Protocol Foundation — Canonical Hashing & Merkle Trees
**Deliverable:** `internal/federation/` package with core crypto primitives.

**Files:**
| File | Operation | Description |
|------|-----------|-------------|
| `internal/federation/canonical.go` | Create | RFC 8785 canonical JSON, domain-separated hashing |
| `internal/federation/merkle.go` | Create | Binary Merkle tree: build, root, proof generation, proof verification |
| `internal/federation/scope.go` | Create | ScopeDescriptor, ScopePredicate types, scope evaluator, canonical scope hash |
| `internal/federation/descriptor.go` | Create | EvidenceDescriptor builder from evidence model, leaf hash computation |
| `internal/federation/merkle_test.go` | Create | Table-driven tests: empty tree (error), single leaf, power-of-2, odd counts, proof verification, tampered proof rejection |
| `internal/federation/scope_test.go` | Create | Scope evaluation against test data, deterministic ordering, canonical form round-trip |

**Key algorithms:**
- `CanonicalHash(domain string, v any) []byte` — domain-separated SHA-256 over canonical JSON
- `BuildMerkleTree(leaves [][]byte) *MerkleTree` — binary tree, odd leaf duplicated
- `(*MerkleTree).Proof(index int) []ProofStep` — inclusion proof generation
- `VerifyProof(leaf, proof, expectedRoot) bool` — constant-time root verification
- `EvaluateScope(ctx, tx, scope) ([]EvidenceDescriptor, error)` — deterministic DB query within SERIALIZABLE transaction. **Note:** PostgreSQL SERIALIZABLE transactions can fail with serialization conflicts under concurrent writes. Implementation MUST include bounded retry logic (max 3 retries with jitter). Spec should document that scope evaluation may fail transiently if evidence is being actively modified on the same case — this is correct behavior (retry until quiescent, or fail with "case is being modified, try again").

### Step 2: Exchange Manifest & Signing
**Deliverable:** Manifest construction, Ed25519 signing, signature verification.

**Files:**
| File | Operation | Description |
|------|-----------|-------------|
| `internal/federation/manifest.go` | Create | ExchangeManifest struct, BuildExchangeManifest, manifest hash computation |
| `internal/federation/signing.go` | Create | Sign/verify manifests using existing `internal/migration/signing.go` Ed25519 signer |
| `internal/federation/manifest_test.go` | Create | Manifest hash determinism, signature round-trip, tampered manifest rejection |

**Design note:** Reuse `migration.DefaultSigner()` — same `INSTANCE_ED25519_KEY` env var. The instance has one identity key used for both migration attestation and federation.

### Step 3: Custody Bridge Events
**Deliverable:** Cross-instance custody chain linking.

**Files:**
| File | Operation | Description |
|------|-----------|-------------|
| `internal/federation/bridge.go` | Create | CreateDisclosureBridgeEvent, CreateImportBridgeEvent |
| `internal/custody/chain.go` | Modify | Add `DISCLOSED_TO_INSTANCE` and `IMPORTED_FROM_INSTANCE` action types |
| `internal/federation/bridge_test.go` | Create | Bridge event hash computation, cross-chain linking verification |

**Integration with existing custody system:**
- Uses existing `custody.RecordCaseEvent()` — no new custody primitives needed
- Bridge events are regular custody events with structured detail JSON
- Existing chain verification (`ChainVerifier.VerifyCaseChain`) works unchanged — bridge events are just events in the local chain

### Step 4: Bundle Packer & Unpacker
**Deliverable:** VKE1 ZIP bundle creation and parsing.

**Files:**
| File | Operation | Description |
|------|-----------|-------------|
| `internal/federation/bundle.go` | Create | PackBundle (streaming ZIP writer), UnpackBundle (ZIP reader + validation) |
| `internal/federation/bundle_format.go` | Create | Path constants, version struct, format validation |
| `internal/federation/bundle_test.go` | Create | Round-trip pack/unpack, missing files detection, corrupt bundle rejection |

**Streaming design:** Evidence files streamed from MinIO → ZIP writer → HTTP response (or disk). Never buffer entire bundle in memory. Follows pattern from existing `internal/cases/export.go`.

### Step 5: Offline Verification Pipeline
**Deliverable:** Self-contained verifier that consumes only a bundle + trusted public key.

**Files:**
| File | Operation | Description |
|------|-----------|-------------|
| `pkg/vkverify/verify.go` | Create | Core VerifyBundle logic — public package, zero DB/network dependencies, importable by standalone vkverify repo |
| `pkg/vkverify/verify_test.go` | Create | Each verification step tested independently, full pipeline test |
| `internal/federation/verify.go` | Create | Thin wrapper: calls `pkg/vkverify` with DB-backed peer trust store for in-app verification |

**Verification order (fail-fast):**
1. Parse bundle structure, validate VKE1 version
2. Resolve sender identity against trust store
3. Verify manifest signature (Ed25519)
4. Verify manifest hash (recompute from manifest fields)
5. Verify scope hash (recompute from scope descriptor)
6. Verify TSA token on manifest hash (RFC 3161)
7. Rebuild Merkle tree from all evidence descriptors, compare root
8. Verify each item's Merkle inclusion proof
9. Stream-verify each evidence file's SHA-256 against descriptor
10. Verify custody bridge event hash
11. Verify derivation commitments for redacted items
12. Check dependency policy compliance (parent/child)

### Step 6: Peer Management & Trust Store
**Deliverable:** Database-backed peer instance registry with key pinning.

**Files:**
| File | Operation | Description |
|------|-----------|-------------|
| `internal/federation/peers.go` | Create | PeerStore interface, PostgreSQL implementation |
| `internal/federation/identity.go` | Create | Instance identity document, .well-known handler |
| `migrations/036_federation.up.sql` | Create | peer_instances, peer_instance_keys, exchange_manifests tables |
| `internal/federation/peers_test.go` | Create | CRUD, trust state transitions, key rotation |

**New tables:**
```sql
CREATE TABLE peer_instances (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    instance_id      TEXT NOT NULL UNIQUE,
    display_name     TEXT NOT NULL,
    well_known_url   TEXT,
    trust_mode       TEXT NOT NULL DEFAULT 'untrusted',
    verified_by      TEXT,            -- user_id who verified
    verified_at      TIMESTAMPTZ,
    verification_channel TEXT,        -- 'phone', 'letter', 'in_person', 'secure_messenger'
    org_id           UUID REFERENCES organizations(id),
    created_at       TIMESTAMPTZ DEFAULT now(),
    updated_at       TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE peer_instance_keys (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    peer_instance_id UUID NOT NULL REFERENCES peer_instances(id),
    public_key       BYTEA NOT NULL,  -- 32 bytes Ed25519
    fingerprint      TEXT NOT NULL,   -- "sha256:base64..."
    status           TEXT NOT NULL DEFAULT 'active', -- active, rotated, revoked
    valid_from       TIMESTAMPTZ DEFAULT now(),
    valid_to         TIMESTAMPTZ,
    rotation_sig_old BYTEA,           -- old key signs new key
    rotation_sig_new BYTEA,           -- new key signs statement
    created_at       TIMESTAMPTZ DEFAULT now()
);
CREATE UNIQUE INDEX idx_peer_keys_active ON peer_instance_keys(peer_instance_id) WHERE status = 'active';

CREATE TABLE exchange_manifests (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    exchange_id           UUID NOT NULL UNIQUE,
    direction             TEXT NOT NULL,  -- 'outgoing' or 'incoming'
    peer_instance_id      UUID REFERENCES peer_instances(id),
    case_id               UUID REFERENCES cases(id),  -- nullable for incoming: assigned when user accepts and chooses destination case
    manifest_hash         TEXT NOT NULL,
    scope_hash            TEXT NOT NULL,
    merkle_root           TEXT NOT NULL,
    scope_cardinality     INT NOT NULL,
    signature             BYTEA NOT NULL,
    status                TEXT NOT NULL DEFAULT 'pending', -- pending, verified, accepted, rejected, revoked, failed
    verification_details  JSONB,
    created_at            TIMESTAMPTZ DEFAULT now(),
    completed_at          TIMESTAMPTZ
);

-- Junction table instead of UUID[] — supports "which exchanges included evidence X?" queries
-- that auditors will ask frequently. UUID[] requires ANY() scans that degrade on large arrays.
CREATE TABLE exchange_evidence_items (
    exchange_manifest_id  UUID NOT NULL REFERENCES exchange_manifests(id) ON DELETE CASCADE,
    evidence_id           UUID NOT NULL REFERENCES evidence_items(id),
    PRIMARY KEY (exchange_manifest_id, evidence_id)
);
CREATE INDEX idx_exchange_evidence_by_item ON exchange_evidence_items(evidence_id);

CREATE TABLE derivation_records (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    child_evidence_id     UUID NOT NULL REFERENCES evidence_items(id),
    parent_evidence_id    UUID NOT NULL,  -- NO FK: parent may exist only on the sending instance (cross-instance derivation)
    derivation_type       TEXT NOT NULL,  -- 'redaction'
    derivation_commitment TEXT NOT NULL,  -- SHA-256 of canonical derivation record
    parameters_commitment TEXT NOT NULL,  -- SHA-256 of redaction parameters
    signed_by_instance    TEXT NOT NULL,
    created_at            TIMESTAMPTZ DEFAULT now()
);
```

### Step 7: Federation HTTP Endpoints
**Deliverable:** REST API for federation operations.

**Files:**
| File | Operation | Description |
|------|-----------|-------------|
| `internal/federation/handler.go` | Create | HTTP handlers for all federation endpoints |
| `internal/federation/service.go` | Create | Business logic orchestrating scope evaluation → tree build → manifest → sign → pack |
| `cmd/server/main.go` | Modify | Register federation routes |

**Endpoints:**
```
# Instance identity
GET  /.well-known/vaultkeeper-instance.json    → public instance identity

# Peer management
POST   /api/federation/peers                    → register peer (admin only)
GET    /api/federation/peers                    → list peers
PATCH  /api/federation/peers/:id               → update trust status
DELETE /api/federation/peers/:id               → remove peer

# Exchange operations
POST   /api/federation/exchanges               → create exchange (scope + peer → bundle)
GET    /api/federation/exchanges               → list exchanges (outgoing + incoming)
GET    /api/federation/exchanges/:id           → get exchange details
GET    /api/federation/exchanges/:id/download  → download bundle ZIP
POST   /api/federation/exchanges/:id/verify    → verify received bundle
POST   /api/federation/exchanges/:id/accept    → accept and import evidence

# Receive (called by remote instance — two-phase flow)
POST   /api/federation/receive/manifest        → phase 1: receive signed manifest (small payload, verified before accepting data)
POST   /api/federation/receive/bundle           → phase 2: stream bundle data (only accepted after manifest signature verified)
```

**Two-phase receive flow:** For large bundles, you don't want to stream 500GB into MinIO before discovering the signature is invalid. Phase 1 accepts only the manifest + signature (a few KB), verifies the sender's identity and signature, and returns a `receive_token`. Phase 2 streams the bundle data, authenticated by the `receive_token`. If Phase 1 fails (unknown peer, bad signature), no evidence bytes are ever accepted.

**Authorization:**
- Peer management: org admin only
- Exchange creation: prosecutor or investigator with case role
- Bundle receipt: authenticated by sender's Ed25519 signature (no Keycloak session needed for remote calls)
- Dual-control option: require two users to approve outgoing exchanges (configurable per org)

### Step 8: Derivation Records for Redacted Evidence
**Deliverable:** Track and prove redaction provenance.

**Files:**
| File | Operation | Description |
|------|-----------|-------------|
| `internal/federation/derivation.go` | Create | DerivationRecord, BuildDerivationCommitment |
| `internal/evidence/redaction.go` | Modify | Record derivation when creating redacted versions |

**Integration:** When a redacted version is finalized (`internal/evidence/redaction.go`), create a DerivationRecord linking child → parent with a commitment hash. This record is included in the VKE1 bundle when the redacted version is part of an exchange.

### Step 9: Frontend — Peer Management
**Deliverable:** UI for managing trusted peer instances.

**Components:**
| Component | Description |
|-----------|-------------|
| `FederationPeersPage` | List of peer instances with trust status badges |
| `AddPeerDialog` | Form: instance URL, display name, public key (paste or fetch from .well-known) |
| `PeerFingerprint` | Displays fingerprint in human-readable groups (like SSH), with "I verified this" button |
| `PeerDetail` | Key history, exchange history, trust status management |

**Key UX decisions:**
- Fingerprint displayed as `sha256:XXXX XXXX XXXX XXXX` (4-char groups, 16 groups) for phone readback
- "Verify" action requires selecting verification channel (phone, in-person, letter, secure messenger)
- Traffic-light trust indicators: red (untrusted), yellow (TOFU pending), green (manually pinned)

### Step 10: Frontend — Exchange Builder (Scope-Based Sharing)
**Deliverable:** UI for creating evidence exchange packages.

**Components:**
| Component | Description |
|-----------|-------------|
| `ExchangeBuilderPage` | Multi-step wizard for creating an exchange |
| `ScopeBuilder` | Visual predicate builder — dropdowns for field/operator/value, AND composition |
| `ScopePreview` | Shows which evidence items match the current scope (two-tier: optimistic preview + exact at confirm) |
| `ExchangeReview` | Final review: peer, scope, matching items, dependency policy, confirm |
| `ExchangeProgress` | Bundle creation progress (scope eval → tree build → sign → pack → transfer) |

**Wizard steps:**
1. Select peer institution
2. Build scope (tag filter, date range, classification level, or manual evidence selection)
3. Preview matching evidence (with item count, total size)
4. Choose dependency policy (none / include parents / full ancestry)
5. Review and confirm — shows manifest preview, scope hash, Merkle root
6. Sign and send (or download bundle for manual transfer)

**Scope preview performance:** The preview uses a **two-tier approach** to keep the UI responsive on large cases:
- **During editing:** Optimistic preview via `READ COMMITTED` query (fast, possibly slightly stale if concurrent writes are happening). Displays item count, total size, and first N items.
- **At confirmation (Step 5):** Exact evaluation via `SERIALIZABLE` transaction. This is the legally binding result. If the exact result differs from the preview (items added/removed during editing), the wizard shows a diff and requires re-confirmation.
This preserves legal integrity (the manifest always uses the SERIALIZABLE result) while keeping the builder interactive.

### Step 11: Frontend — Received Exchanges & Verification
**Deliverable:** UI for reviewing and importing received evidence.

**Components:**
| Component | Description |
|-----------|-------------|
| `ReceivedExchangesPage` | List of incoming exchanges with verification status |
| `ExchangeVerification` | Step-by-step verification display (signature ✓, scope ✓, Merkle ✓, files ✓) |
| `ImportConfirmation` | Review evidence items before importing into local case |
| `CustodyBridgeView` | Visualize cross-instance custody chain link |

**Verification UX:**
- Each verification step shown as a checklist with pass/fail
- Failed steps show specific error (which file, which hash mismatch)
- "Accept" button only enabled after full verification passes
- Import creates evidence items + custody bridge events in single transaction

### Step 12: `vkverify` — Permissionless Offline Verifier

**Deliverable:** A single static binary with zero dependencies. No runtime, no Docker, no Node, no Python, no browser, no network. Go compiles to fully static binaries via `CGO_ENABLED=0`. Cross-compile for all platforms from one CI pipeline: `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`, `windows/amd64`. A judge downloads it to a USB stick, carries it to the courtroom, plugs it into whatever machine is available, runs it. No installation, no admin privileges, no internet.

**Strategic note:** `vkverify` is the most important marketing artifact in the entire federation feature. It MUST be published as a **separate open-source project** with its own GitHub releases page, README, and documentation. Non-VaultKeeper users verifying bundles is essential for protocol credibility.

**Architecture:**
- The verification library lives at `pkg/vkverify/` (NOT `internal/` — Go's `internal/` packages cannot be imported from outside the module). This is one of the few packages that must be importable by the standalone repo.
- `cmd/vkverify/` is the source of truth in the monorepo, imports `pkg/vkverify`
- Separate repo `vaultkeeper/vkverify` publishes standalone binaries via GitHub Releases, with a `go.mod` that imports `github.com/trelvio/vaultkeeper/pkg/vkverify`
- `internal/federation/verify.go` (Step 5) is a thin wrapper that calls `pkg/vkverify` with VaultKeeper-specific trust store integration. The core verification logic lives in the public package; the internal wrapper adds database-backed peer resolution.

**Four operating modes:**

1. **CLI mode** (`./vkverify verify --bundle X --pubkey Y`) — forensic experts and technical users. Structured output, `--format json` for machine parsing, `--report output.pdf` for court-filing PDF.
   ```
   vkverify verify --bundle vaultkeeper-exchange-abc123.zip \
                   --pubkey bellingcat-instance.pub \
                   --verbose
   # Exit code 0 = all checks pass, 1 = verification failure, 2 = bundle format error
   ```

2. **Interactive mode** (`./vkverify` with no args) — judges and lawyers. Launches an embedded HTTP server bound to `127.0.0.1` on a random port, serves a verification web UI from files compiled into the binary via `//go:embed`, auto-opens the default browser. Server shuts down on Ctrl+C (SIGINT/SIGTERM) or after 5 minutes of inactivity (no HTTP requests received). Same verification logic as CLI — the web UI is just an interface to the same function.

3. **Institutional server mode** (`./vkverify serve --listen 10.0.0.50:8443 --tls-cert court.pem --tls-key court.key`) — court IT departments run it as a persistent service on their LAN. Multiple lawyers access via browser. Still no external connections. Same binary.

4. **Self-verify mode** (`./vkverify --self-verify`) — prints the binary's own SHA-256 hash, build metadata (Go version, commit hash, build timestamp, target platform), and instructions for independent verification against published hashes.

**PDF report generation:**
Generated by Go (pure Go PDF library, no CGO), not by JavaScript or the browser. Report generation is covered by the same reproducible build as the verification logic. Contents:
- Verifier binary identity (hash, version, commit)
- Bundle identity (hash, size)
- Sender identity (fingerprint)
- Verdict (pass/fail)
- All verification steps with pass/fail
- Plain-language summary for non-technical readers
- Attestation statement, **mode-aware**: CLI/interactive/server modes state "verification was performed locally with no network connectivity." WASM mode states "verification was performed in a web browser using a WASM module loaded from [URL]." The report must never claim local-only verification when it wasn't.

**Files:**
| File | Operation | Description |
|------|-----------|-------------|
| `cmd/vkverify/main.go` | Create | CLI entry point, mode selection |
| `cmd/vkverify/serve.go` | Create | Embedded + institutional HTTP server |
| `cmd/vkverify/report.go` | Create | Pure-Go PDF court report generation |
| `cmd/vkverify/web/` | Create | `//go:embed` web UI (vanilla HTML/CSS/JS, no frameworks) |
| `internal/federation/verify.go` | Create (Step 5) | Verification library, zero external dependencies |

**Localization (architect from day one, ship English in v1):**
The plain-language verdict ("This evidence package is authentic and unmodified") should support at minimum the six UN languages (English, French, Spanish, Arabic, Russian, Chinese) plus German and Dutch for primary markets. Format: JSON locale files (`locales/en.json`, `locales/fr.json`, etc.) embedded via `//go:embed locales/*`, with a simple `func T(locale, key string) string` lookup. No i18n framework needed for this scope. The string architecture ships in v1 with only `en.json` populated.

**Effort estimate:** 4-5 weeks.

### Step 13: Large Bundle Transfer
**Deliverable:** Chunked/resumable transfer for evidence volumes exceeding single-request limits.

**Problem:** Real-world evidence exchanges can be hundreds of gigabytes (video surveillance, drone footage, document archives). The two-phase receive flow (Step 7) handles manifest verification before accepting data, but the data transfer itself needs to be resumable.

**Solution: Generate-and-fetch pattern:**
1. Sender creates exchange, bundle written to MinIO as a temporary object
2. Sender notifies recipient (via federation endpoint or out-of-band) with bundle metadata + download URL
3. Recipient fetches bundle via `GET /api/federation/exchanges/:id/download` with HTTP Range support
4. For air-gapped transfers: bundle downloaded to removable media, physically transported

**Endpoints (additions to Step 7):**
```
# Sender generates bundle, returns metadata + download token
POST   /api/federation/exchanges/:id/prepare    → prepare bundle for download
# Recipient fetches with resumable download (HTTP Range headers)
GET    /api/federation/exchanges/:id/download   → (already listed) + Range support + download token auth
```

**Bundle size metadata in manifest:**
```go
type BundleMetadata struct {
    TotalSizeBytes    int64  `json:"total_size_bytes"`
    EvidenceFileCount int    `json:"evidence_file_count"`
    LargestFileBytes  int64  `json:"largest_file_bytes"`
}
```

**Download token specification:**
Signed JWT preferred over opaque server-side token (verifiable without database lookup). Token constraints:
- **Single-use:** Nonce claim, tracked in Redis or PostgreSQL to prevent reuse after completion
- **Time-bounded:** `exp` claim, default 24 hours (configurable per exchange based on bundle size)
- **Bound to exchange:** `exchange_id` claim, verified on every Range request
- **Bound to recipient:** `recipient_instance_id` claim, verified against sender's manifest
- **Signed by sender instance:** Ed25519 signature using the instance key (reuses existing signing infrastructure)
- **Transmission:** Included in the Phase 1 manifest acknowledgment response (receiver gets token after manifest verification succeeds)
- **Mid-download expiry:** If token expires during a large download, the receiver requests a new token via the manifest endpoint (re-verifies identity). Partial downloads are valid — Range requests resume from last byte.

**Implementation notes:**
- MinIO presigned URLs for direct download (bypass Go server for large files)
- Progress tracking: receiver reports download progress, sender can see "70% transferred"
- Timeout: bundles expire from temporary storage after configurable period (default 7 days)

### Step 14: WASM Hosted Verifier & Distribution Infrastructure

**Deliverable:** Browser-based convenience verifier and multi-source distribution for the standalone binary.

**The WASM version is the *convenience tier*, not the primary tool.** It compiles from the same Go verification source via `GOOS=js GOARCH=wasm` (or TinyGo if crypto primitives work — test first, TinyGo produces 1-3MB vs Go's 10-15MB). Lives on `verify.vaultkeeper.eu` and on every VaultKeeper instance's `/verify` page.

The hosted page must explicitly state: *"For court proceedings and formal verification, download the standalone verification tool. This web page is provided for convenience but relies on your browser and this server. The standalone tool relies on nothing."*

**Security requirements for the hosted page (non-negotiable):**
- **No network requests.** The page must never transmit a single byte of the uploaded bundle.
- **CSP:** `connect-src 'none'` — browser blocks ALL outbound connections at the network layer. Even compromised JavaScript cannot exfiltrate data.
- **No "fetch public key from institution" convenience button** — public key must be pasted or dropped as a file.
- **SRI** (Subresource Integrity) hash on the WASM module — browser refuses to execute if hash doesn't match.
- **Additional headers:** `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy: no-referrer`, `Permissions-Policy: camera=(), microphone=(), geolocation=(), usb=()`
- **Page size:** Under 500 lines total HTML/CSS/JS. No frameworks, no React, no build tools. Vanilla code that loads the WASM, reads files via File API, calls the verification function, renders the result. Every additional line is attack surface. Auditable by a single person in under an hour.

**Reproducible builds with multi-party attestation (Bitcoin Core / Gitian model):**
- Build must be fully reproducible — same source, same compiler version, same flags produce byte-identical binaries
- Dockerfile pins Go version, uses `-trimpath` and `CGO_ENABLED=0`
- No embedded timestamps that vary between builds (build time injected as a fixed string via `-ldflags`, not `time.Now()`)
- Multiple independent parties build from source and publish their hashes
- Repository contains `VERIFICATION.md` updated with each release: SHA-256 hashes for every platform binary plus independent build attestations from multiple organizations (invite Bellingcat tech team, Lighthouse engineers, academic institutions)
- If all hashes match, the binary is trustworthy. If any diverges, something is wrong and the discrepancy is publicly visible
- Each release is a signed Git tag. PGP signatures from multiple attesters on `VERIFICATION.md`. Git history serves as the deployment transparency log.

**Distribution channels (multiple independent sources):**
An attacker must compromise all channels simultaneously. Any single compromised channel is detectable by comparing hashes across channels.

| Channel | Type | Description |
|---------|------|-------------|
| GitHub Releases | Primary | Standalone binaries + WASM + checksums |
| VaultKeeper instances (`/tools/vkverify`) | Secondary | Every deployed instance serves the binary |
| VaultKeeper instances (`/verify`) | Secondary | Every deployed instance serves the WASM page |
| `verify.vaultkeeper.eu` | Tertiary | Dedicated neutral domain for WASM page |
| Package managers (`brew install vkverify`) | Convenience | Where possible |

**Trust hierarchy (documented in tool's help output, web page, and verification report):**

| Tier | Trust Level | What You Trust |
|------|-------------|----------------|
| 1 (highest) | Build from source | Go compiler + source code |
| 2 | Download binary, verify hash against multiple attesters | Attesters aren't all colluding |
| 3 | Download from single trusted source | That source wasn't compromised (multi-attestation makes compromise detectable) |
| 4 | Run `./vkverify` embedded local web UI | Same as Tier 2/3 + your browser |
| 5 (lowest) | Hosted WASM page (`verify.vaultkeeper.eu` or `/verify`) | Hosting server, your network, your DNS, your browser. For journalists and quick checks. NOT for formal court proceedings. |

**Threat model specific to the verification tool:**
The verification page/binary is a high-value target for state-level adversaries. If Russia knows Ukrainian war crimes evidence is verified through `verify.vaultkeeper.eu`, compromising that page to return false positives on tampered bundles is an obvious attack vector. Defenses: SRI hashes (browser-enforced), `connect-src 'none'` (browser-enforced), reproducible builds (divergence detectable), multi-source hosting (must compromise all simultaneously), standalone binary as primary recommendation (no server trust required), deployment transparency via signed Git commits.

**Things that cannot be defended against (document explicitly):** A compromised machine running the verifier, a backdoored Go compiler (Ken Thompson's "Trusting Trust," 1984), social engineering convincing a user to use a fake page, and a colluding majority of build attesters. These are fundamental limits, not design flaws.

**Files:**
| File | Operation | Description |
|------|-----------|-------------|
| `cmd/vkverify/wasm/` | Create | WASM build target, JS glue code |
| `web/public/verify/index.html` | Create | Hosted WASM verification page (<500 lines) |
| `cmd/vkverify/Dockerfile` | Create | Reproducible build (pinned Go, -trimpath, CGO_ENABLED=0) |
| `VERIFICATION.md` | Create | Release hashes + attestation template |
| `.github/workflows/vkverify-release.yml` | Create | Cross-platform build + hash publication |

**Effort estimate:** 2-3 weeks (can overlap with Steps 9-11 frontend work).

---

## Risks and Mitigation

| Risk | Severity | Mitigation |
|------|----------|------------|
| Sender omits evidence matching scope | High | Merkle tree commits to full scope result set; scope is deterministic and auditable. **See "Lying Source Problem" section** — completeness depends on custody chain integrity. |
| Replay attack — old bundle presented as new | Medium | Exchange ID uniqueness, recipient binding, timestamp, duplicate import detection per exchange_id |
| MITM at first contact | Medium | Manual fingerprint verification as default trust bootstrap. TOFU optional, not default. |
| Scope semantics ambiguity | Medium | Strict canonical DSL with no `or` in v1. Deterministic evaluation within SERIALIZABLE transaction. |
| Metadata leakage via proof structure | Low | Per-disclosure trees (not whole-case). Minimal leaf payloads. No sibling leaf content exposed — only sibling hashes. |
| Key compromise | High | Key rotation with dual-signature statements. HSM-backed keys recommended for production. Rotation invalidates old key for future exchanges (past exchanges remain valid — timestamped). |
| Evidence changes between scope eval and export | Medium | Scope bound to `as_of_custody_hash` snapshot anchor. Evaluation within SERIALIZABLE DB transaction. |
| Large bundle performance | Low | Streaming ZIP construction. Parallel file hashing in Go worker pool. Merkle tree built in memory (leaf count bounded by scope cardinality). |
| Large bundle transfer (100GB+) | Medium | Generate-and-fetch pattern with MinIO presigned URLs, HTTP Range support, download tokens. Air-gapped transfer via removable media. See Step 13. |
| SERIALIZABLE transaction contention | Low | Bounded retry (3x with jitter). Scope eval fails cleanly under concurrent case modification — user retries when case is quiescent. |
| Key rotation after compromise | High | Dual-signature rotation does NOT protect against compromised key holder. Peers MUST re-verify rotated keys out-of-band. HSM recommended. See key rotation security note. |
| Accidental privileged disclosure | Medium | v1: dual-control approval (Step 7) requires two people to make the same mistake; operational recovery via out-of-band contact. v2: protocol-level revocation. Exchange status supports `revoked`. See v2 considerations. |

---

## Threat Model

### Adversaries
1. **Malicious sender** — omits evidence, substitutes files, manipulates scope
2. **Malicious receiver** — alters evidence post-receipt, claims different content
3. **Network attacker** — MITM during first contact or bundle transfer
4. **Insider with disclosure privileges** — unauthorized sharing
5. **Stolen instance key** — forge exchanges
6. **Curious recipient** — infer undisclosed evidence from proof metadata

### Controls
| Threat | Control |
|--------|---------|
| Evidence omission | Signed canonical scope + Merkle root + cardinality |
| File substitution | Per-file SHA-256 in descriptor, descriptor in Merkle leaf |
| Post-receipt alteration | Manifest hash + signature; receiver import event references signed manifest |
| MITM key swap | Manual fingerprint verification; key pinning |
| Proof metadata inference | Per-disclosure trees; minimal leaf payloads; no sibling content |
| Replay | Exchange ID + recipient binding + duplicate detection |
| Unauthorized disclosure | Keycloak role checks + optional dual-control approval |
| Key compromise | Rotation statements + HSM recommendation + TSA timestamps (past exchanges remain timestamped) |

---

## Multi-Hop Provenance

When B shares a subset of A's evidence to C:
1. B's exchange to C includes B's custody chain
2. B's custody chain contains `IMPORTED_FROM_INSTANCE` event referencing A
3. C can verify: B received from A (signed by A's key) → B shared to C (signed by B's key)
4. C holds two independent verification chains
5. If C later shares to D, the provenance graph extends: A → B → C → D

Each hop is independently verifiable with the respective instance's public key. No single entity controls the full chain.

---

## Dependency Policy for Parent-Child Evidence

When sharing redacted versions that reference originals:

| Policy | Behavior |
|--------|----------|
| `none` | Only listed items. No parent metadata. |
| `direct_parent` | Include parent evidence descriptor (metadata only, not file) if child has `parent_id` |
| `full_ancestry` | Include all ancestor descriptors up to root |

If a required dependency is omitted, it must be explicitly declared:
```json
{"evidence_id": "ev-010-r1", "missing_dependency": "ev-010", "reason_code": "withheld_privilege"}
```

Reason codes: `withheld_privilege`, `withheld_security`, `outside_scope`, `superseded_not_required`

---

## Protocol Versioning Strategy

**VKE1 exchanges do not require version negotiation.** The bundle is self-describing (`vkx/version.json` declares `"format": "VKE1"`), and the verifier either supports that version or rejects the bundle.

**Instance identity advertises capabilities** via `supported_bundle_versions: ["VKE1"]` in `.well-known/vaultkeeper-instance.json`. The exchange builder UI shows the intersection of sender and recipient supported versions and defaults to the highest common version.

**Future versions MAY introduce a negotiation step** — but VKE1 explicitly does not require one. A VKE1 bundle is valid regardless of whether the recipient advertised VKE1 support, because the bundle is self-contained and offline-verifiable. The recipient's instance identity is consulted as a convenience (to avoid sending a bundle the recipient can't import), not as a protocol requirement.

**Reserved version space:** VKE2, VKE3, etc. Version numbers are monotonic. A bundle declares exactly one version. Backward compatibility is NOT guaranteed — each version is a complete specification. Verifiers SHOULD support multiple versions simultaneously.

---

## Exchange Revocation (v2 Consideration)

**Problem:** A sender realizes they accidentally included privileged material in an exchange. There is currently no protocol-level mechanism to notify the receiver that an exchange should be withdrawn.

**v2 design direction:**
- New custody event type: `REVOKED_EXCHANGE` with detail `{exchange_id, reason, revoked_by, revoked_at}`
- Sender signs a revocation statement: `{"type":"revocation","exchange_id":"...","reason":"privileged_material","signature":"..."}`
- Revocation pushed to recipient via federation endpoint or included in next exchange
- Recipient is **notified** but **not obligated** to delete — deletion depends on jurisdiction, court order, and the recipient's own legal obligations
- Exchange status transitions: `accepted` → `revoked_by_sender` (receiver acknowledges) or `revocation_contested` (receiver disputes)

**Why v2, not v1:** Revocation semantics are jurisdiction-dependent and require legal review. The data model supports it (exchange_manifests.status already includes `revoked`), but the protocol behavior needs careful specification. For v1, accidental disclosure is handled operationally — sender contacts recipient out-of-band, as they would today.

---

## Key Files

| File | Operation | Description |
|------|-----------|-------------|
| `internal/federation/canonical.go` | Create | RFC 8785 canonical JSON, domain-separated hashing |
| `internal/federation/merkle.go` | Create | Merkle tree construction, proof generation/verification |
| `internal/federation/scope.go` | Create | Scope DSL types, evaluator, canonical hash |
| `internal/federation/descriptor.go` | Create | Evidence descriptor builder, leaf hash |
| `internal/federation/manifest.go` | Create | Exchange manifest construction, hash computation |
| `internal/federation/signing.go` | Create | Manifest signing using instance Ed25519 key |
| `internal/federation/bridge.go` | Create | Cross-instance custody bridge events |
| `internal/federation/bundle.go` | Create | VKE1 ZIP bundle packer/unpacker |
| `pkg/vkverify/` | Create | Public verification library (importable by standalone vkverify repo) |
| `internal/federation/verify.go` | Create | Thin wrapper over pkg/vkverify with DB-backed trust store |
| `internal/federation/peers.go` | Create | Peer trust store (PostgreSQL) |
| `internal/federation/identity.go` | Create | Instance identity, .well-known handler |
| `internal/federation/derivation.go` | Create | Redaction provenance records |
| `internal/federation/handler.go` | Create | HTTP endpoints |
| `internal/federation/service.go` | Create | Business logic orchestrator |
| `internal/custody/chain.go` | Modify | Add federation action types |
| `internal/evidence/redaction.go` | Modify | Record derivation on redaction |
| `cmd/server/main.go` | Modify | Register federation routes |
| `cmd/vkverify/main.go` | Create | Verifier CLI entry point + mode selection (publish separately as open-source) |
| `cmd/vkverify/serve.go` | Create | Embedded + institutional HTTP server modes |
| `cmd/vkverify/report.go` | Create | Pure-Go PDF court report generation |
| `cmd/vkverify/web/` | Create | `//go:embed` verification web UI (vanilla HTML/CSS/JS) |
| `cmd/vkverify/wasm/` | Create | WASM build target + JS glue |
| `cmd/vkverify/Dockerfile` | Create | Reproducible build (pinned Go, -trimpath, CGO_ENABLED=0) |
| `web/public/verify/index.html` | Create | Hosted WASM verification page (<500 lines, CSP `connect-src 'none'`) |
| `VERIFICATION.md` | Create | Release hashes + multi-party attestation template |
| `.github/workflows/vkverify-release.yml` | Create | Cross-platform build + hash publication pipeline |
| `internal/federation/transfer.go` | Create | Large bundle transfer: presigned URLs, download tokens, Range support |
| `migrations/036_federation.up.sql` | Create | Federation tables (incl. exchange_evidence_items junction) |
| `docs/federation/VKE1-SPEC.md` | Create | Protocol specification |
| `web/src/app/(app)/federation/` | Create | Federation UI pages |

---

## Effort Estimate

| Phase | Steps | Estimate | Parallelizable |
|-------|-------|----------|----------------|
| Protocol foundation + crypto | 1-5 | 6-8 weeks | No (sequential dependencies) |
| Peer management + endpoints | 6-8 | 4-6 weeks | Partially (6-7 parallel, 8 after) |
| Frontend | 9-11 | 6-8 weeks | Yes (parallel with Steps 6-8) |
| vkverify (all modes + PDF) | 12 | 4-5 weeks | Yes (parallel with Steps 9-11) |
| Large bundle transfer | 13 | 2-3 weeks | Yes (parallel with Step 12) |
| WASM + distribution infra | 14 | 2-3 weeks | Yes (after Step 12) |

**Total sequential:** ~24-33 weeks. **With parallelization:** ~18-24 weeks (4.5-6 months) of focused full-time work. At part-time pace alongside other portfolio work: 12-16 months.

---

## Definition of Done

- [ ] Scope DSL evaluates deterministically against evidence items
- [ ] Merkle tree proofs verify correctly for any subset size
- [ ] Signed manifest rejects tampering (any field change → signature failure)
- [ ] VKE1 bundle round-trips: pack → unpack → verify passes
- [ ] Custody bridge links sender and receiver chains cryptographically
- [ ] Peer trust store supports manual pinning + key rotation (with out-of-band re-verification warning)
- [ ] Redacted evidence carries provenance commitments
- [ ] Multi-hop provenance traceable (A → B → C verifiable by C)
- [ ] Frontend wizard creates exchanges from scope definitions
- [ ] Frontend verification displays step-by-step results
- [ ] RFC 3161 timestamp on exchange manifests
- [ ] Large bundle transfer works for 100GB+ exchanges (presigned URLs, Range support, signed download tokens)
- [ ] Two-phase receive flow: manifest verified before evidence bytes accepted
- [ ] SERIALIZABLE scope evaluation retries cleanly under concurrent writes
- [ ] Lying source limitation documented in VKE1 spec "Security Considerations" with concrete scenario
- [ ] All tests passing, coverage ≥ 80% on `internal/federation/`
- [ ] VKE1 protocol specification published in `docs/federation/`
- [ ] Protocol versioning strategy documented (no negotiation in v1, reserved version space)
- [ ] Exchange revocation noted as v2 consideration with data model support in v1
- **vkverify (Step 12):**
- [ ] `vkverify verify` produces correct verdict for authenticated, tampered, and invalid bundles
- [ ] `vkverify verify --report output.pdf` generates court-filing PDF with all required sections
- [ ] `vkverify --self-verify` prints binary hash and build metadata
- [ ] `vkverify` (no args) launches embedded local web UI on localhost
- [ ] `vkverify serve` runs persistent institutional server with TLS
- [ ] Embedded web UI: file never leaves browser, same verification logic as CLI
- [ ] Cross-platform binaries for all five targets (`linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`, `windows/amd64`)
- [ ] Localization string architecture in place (English populated, message keys for 8 languages)
- [ ] Published as standalone open-source project with own GitHub Releases
- **WASM & Distribution (Step 14):**
- [ ] Hosted WASM page: `connect-src 'none'`, SRI on WASM module, <500 lines HTML/CSS/JS
- [ ] WASM module produces identical results to CLI for all test bundles
- [ ] Reproducible build: Docker produces byte-identical binaries across runs
- [ ] `VERIFICATION.md` template populated with release hashes
- [ ] Every VaultKeeper instance serves `/verify` page and `/tools/vkverify` binary
- [ ] Trust hierarchy documented in plain language for legal professionals

---

## SESSION_ID (for /ccg:execute use)
- CODEX_SESSION: codex-1776331003-64849
- GEMINI_SESSION: (quota exhausted — not available)
