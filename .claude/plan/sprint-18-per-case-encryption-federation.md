# Sprint 18: Per-Case Encryption & Federation Protocol

**Phase:** 3 — AI & Advanced Features
**Duration:** Weeks 35-36
**Goal:** Implement per-case encryption key isolation and the federation protocol for cross-institutional evidence sharing. Ship v3.0.0 completing Phase 3.

---

## Prerequisites

- Sprint 17 complete (external API, API keys)
- All Phase 3 AI features operational

---

## Task Type

- [x] Backend (Go)
- [x] Frontend (Next.js)

---

## Implementation Steps

### Step 1: Per-Case Encryption Keys

**Deliverable:** Optional encryption key isolation per case.

**Architecture:**
- Each case can have its own encryption key
- Evidence in Case A encrypted with Key A, Case B with Key B
- Compromise of Key A doesn't expose Case B
- Key derivation: HKDF from master key + case_id as context
- Or: explicit key provided by institution's KMS (Keycloak or HashiCorp Vault)

**Key management options:**
1. **Derived keys (default):** HKDF from `MASTER_ENCRYPTION_KEY` + case UUID
   - Simple, no external KMS needed
   - Compromise of master key compromises all cases
2. **External KMS:** Key fetched from Keycloak or HashiCorp Vault per case
   - Stronger isolation
   - Institution manages key lifecycle
   - VaultKeeper never stores the key — fetches on demand, keeps in memory

**Implementation:**
```go
type CaseKeyManager interface {
    GetCaseKey(ctx context.Context, caseID uuid.UUID) ([]byte, error)
    RotateCaseKey(ctx context.Context, caseID uuid.UUID) error
}
```

**Encryption layer:**
- On evidence upload: encrypt file with case-specific key before storing in MinIO
- On evidence download: decrypt with case-specific key after reading from MinIO
- SSE-S3 (MinIO server-side encryption) still active as baseline
- Per-case encryption is an additional layer (defense in depth)

**New table:**
```sql
CREATE TABLE case_encryption (
    case_id         UUID PRIMARY KEY REFERENCES cases(id),
    key_source      TEXT NOT NULL,     -- 'derived' or 'external'
    key_version     INT DEFAULT 1,
    kms_key_id      TEXT,              -- reference to external KMS key
    enabled         BOOLEAN DEFAULT false,
    created_at      TIMESTAMPTZ DEFAULT now(),
    rotated_at      TIMESTAMPTZ
);
```

**Key rotation:**
1. Generate new key version
2. Background job re-encrypts all evidence files in case
3. Update key_version
4. Log rotation in custody chain
5. Old key version retained until re-encryption complete

**Tests:**
- Evidence encrypted with case-specific key
- Different cases → different encryption keys
- Derived key deterministic (same case_id → same key)
- Decrypt with correct key → original file
- Decrypt with wrong key → failure
- Key rotation → all files re-encrypted
- External KMS integration (mock KMS in tests)
- Case without per-case encryption → uses baseline SSE-S3 only
- Enable per-case encryption on existing case → encrypts existing files

### Step 2: Federation Protocol Design

**Deliverable:** Open specification for cross-institutional evidence sharing.

**Protocol specification (`docs/federation/SPEC.md`):**

**Core concept:** Two VaultKeeper instances share specific evidence items with cryptographic verification that nothing was altered in transit.

**Federation flow:**
1. **Initiate share:** Institution A selects evidence items to share with Institution B
2. **Create share package:**
   - Evidence files (encrypted in transit)
   - Metadata for each item
   - Custody chain for each item
   - SHA-256 hashes for each file
   - RFC 3161 timestamps
   - Digital signature of the package (Institution A's key)
3. **Transfer:** Package sent via HTTPS to Institution B's federation endpoint
4. **Receive and verify:**
   - Institution B verifies package signature
   - Computes SHA-256 for each received file
   - Compares against sender's hashes
   - If all match → accept, create evidence items + custody chain
   - If any mismatch → reject entire package, alert both institutions
5. **Custody bridge:**
   - Institution B's custody chain includes: "Received from Institution A via federation"
   - References Institution A's custody chain (hash of the sent custody chain)
   - RFC 3161 timestamp on the federation event

**Federation endpoints:**
```
POST   /api/federation/share              → Initiate share to another instance
POST   /api/federation/receive            → Receive share from another instance
GET    /api/federation/shares             → List outgoing shares
GET    /api/federation/received           → List incoming shares
POST   /api/federation/verify/:id         → Verify received package integrity
```

**Trust model:**
- Institutions exchange public keys during setup (out-of-band)
- Each instance has a federation keypair (Ed25519)
- Shares signed with sender's private key, verified with sender's public key
- No central authority — peer-to-peer trust

**New tables:**
```sql
CREATE TABLE federation_peers (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            TEXT NOT NULL,
    url             TEXT NOT NULL,
    public_key      BYTEA NOT NULL,
    verified        BOOLEAN DEFAULT false,
    created_at      TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE federation_shares (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    direction       TEXT NOT NULL,  -- 'outgoing' or 'incoming'
    peer_id         UUID REFERENCES federation_peers(id),
    case_id         UUID REFERENCES cases(id),
    evidence_ids    UUID[] NOT NULL,
    package_hash    TEXT NOT NULL,
    signature       BYTEA NOT NULL,
    status          TEXT DEFAULT 'pending',
    created_at      TIMESTAMPTZ DEFAULT now(),
    completed_at    TIMESTAMPTZ
);
```

**Tests:**
- Share package created with correct hashes + signature
- Package signature verified correctly
- File hash verification on receive
- Hash mismatch → package rejected
- Tampered signature → rejected
- Custody chain bridged correctly
- RFC 3161 timestamp on federation event
- Unknown peer → rejected
- Duplicate share → idempotent

### Step 3: Federation Frontend

**Components:**
- `FederationPeers` — Manage trusted institutions
  - Add peer (name, URL, public key)
  - Verify connectivity
  - Revoke trust
- `FederationShare` — Share evidence with peer
  - Select evidence items
  - Select destination institution
  - Review package contents
  - Send
- `FederationReceived` — View received packages
  - Verification status
  - Accept/reject
  - View bridged custody chain

### Step 4: Custody Report Format Specification

**Deliverable:** Published open specification for custody report format.

**`docs/spec/custody-report-format.md`:**
- JSON schema for custody chain data
- PDF report layout specification
- Hash chain algorithm specification
- RFC 3161 timestamp format
- Federation custody bridge format
- Version: 1.0.0

**Goal:** Other tools can produce/consume this format, making VaultKeeper the reference implementation.

### Step 5: v3.0.0 Release

**Checklist:**
- [ ] All Phase 3 features working
- [ ] AI services (Whisper, Ollama, Tesseract) operational in Docker
- [ ] Semantic search + entity extraction functional
- [ ] Per-case encryption tested
- [ ] Federation protocol tested between two instances
- [ ] API documentation complete (OpenAPI + Swagger UI)
- [ ] Custody report format spec published
- [ ] All tests passing, coverage >= 80%
- [ ] CHANGELOG.md v3.0.0
- [ ] Docker image tagged v3.0.0

---

## Key Files

| File | Operation | Description |
|------|-----------|-------------|
| `internal/encryption/case_keys.go` | Create | Per-case key management |
| `internal/federation/protocol.go` | Create | Federation share/receive logic |
| `internal/federation/crypto.go` | Create | Ed25519 signing + verification |
| `internal/federation/handler.go` | Create | Federation HTTP endpoints |
| `migrations/014_case_encryption.up.sql` | Create | Case encryption table |
| `migrations/015_federation.up.sql` | Create | Federation tables |
| `docs/federation/SPEC.md` | Create | Federation protocol spec |
| `docs/spec/custody-report-format.md` | Create | Custody format spec |
| `web/src/components/federation/*` | Create | Federation UI |

---

## Definition of Done

- [ ] Per-case encryption isolates evidence between cases
- [ ] Key rotation re-encrypts all case evidence
- [ ] External KMS integration works (Keycloak / HashiCorp Vault)
- [ ] Federation protocol shares evidence between instances
- [ ] Federation verifies file integrity on receive
- [ ] Federation custody chain bridged correctly
- [ ] Digital signatures prevent share tampering
- [ ] Open specifications published
- [ ] v3.0.0 released

---

## Security Checklist

- [ ] Per-case keys derived via HKDF (not truncation/XOR)
- [ ] Master encryption key from env var only
- [ ] Key rotation atomic (no partial re-encryption state)
- [ ] Federation uses Ed25519 signatures (not RSA)
- [ ] Federation transport encrypted (HTTPS only)
- [ ] Peer public keys verified out-of-band
- [ ] Federation packages encrypted in transit
- [ ] Unknown/revoked peers rejected
- [ ] Custody format spec doesn't expose implementation vulnerabilities

---

## Test Coverage Requirements (100% Target)

Every line of code introduced in Sprint 18 must be covered by automated tests. CI blocks merge if coverage drops below 100% for new code.

### Unit Tests

- **`internal/encryption/case_keys.go`** — `GetCaseKey` (derived mode): HKDF with master key + case UUID produces deterministic 256-bit key; same case ID always yields same key; different case IDs yield different keys
- **`internal/encryption/case_keys.go`** — `GetCaseKey` (external KMS mode): fetches key from mock KMS; caches in memory; returns error if KMS unreachable
- **`internal/encryption/case_keys.go`** — `RotateCaseKey`: increments key_version; returns new key different from old key; logs rotation in custody chain
- **HKDF derivation** — uses SHA-256; info parameter includes case UUID; salt is consistent; output length is 32 bytes; not using truncation or XOR
- **Encryption layer** — encrypt file with case key, decrypt with same key yields original bytes; decrypt with wrong key returns error; encrypted output differs from plaintext; different case keys produce different ciphertext for same input
- **Case without per-case encryption** — `GetCaseKey` returns nil/sentinel; upload path skips per-case encryption layer; baseline SSE-S3 still active
- **Enable encryption on existing case** — marks case as encrypted; triggers re-encryption job for all existing evidence files
- **`internal/federation/crypto.go`** — Ed25519 keypair generation: produces valid 32-byte public key and 64-byte private key
- **`internal/federation/crypto.go`** — sign package: produces detached Ed25519 signature; signature length is 64 bytes
- **`internal/federation/crypto.go`** — verify signature: valid signature returns true; tampered payload returns false; wrong public key returns false
- **`internal/federation/protocol.go`** — `CreateSharePackage`: includes file hashes (SHA-256), metadata, custody chain, RFC 3161 timestamps, and Ed25519 signature
- **`internal/federation/protocol.go`** — `ReceiveSharePackage`: verifies signature; computes SHA-256 for each file; compares against sender hashes; all match returns success; any mismatch returns rejection with details
- **Federation package integrity** — tampered file detected (hash mismatch); tampered metadata detected (signature invalid); tampered custody chain detected (signature invalid)
- **Peer management** — add peer with public key; verify peer connectivity (mock); revoke peer trust; unknown peer rejected on receive
- **Custody bridge** — received evidence custody chain includes "Received from [Peer Name] via federation"; references hash of sender's custody chain; RFC 3161 timestamp on federation event
- **Duplicate share handling** — same package sent twice is idempotent; no duplicate evidence items created
- **Migration `014_case_encryption.up.sql`** — table created; default values correct; foreign key to cases enforced
- **Migration `015_federation.up.sql`** — federation_peers and federation_shares tables created; indexes created; foreign keys enforced

### Integration Tests (testcontainers)

- **Per-case encryption round-trip** — enable encryption on case, upload evidence file, verify file stored encrypted in MinIO, download and verify decrypted content matches original
- **Key isolation between cases** — enable encryption on cases A and B, upload evidence to both, verify case A's file cannot be decrypted with case B's key
- **Key rotation end-to-end** — enable encryption, upload 5 files, rotate key, verify all 5 files re-encrypted with new key, verify download still returns original content
- **External KMS integration** — configure mock KMS (HashiCorp Vault testcontainer), enable external KMS mode, upload evidence, verify key fetched from KMS, download succeeds
- **Federation share end-to-end (two instances)** — spin up two VaultKeeper instances in testcontainers, register as peers, share evidence from instance A to instance B, verify evidence received with correct hashes and custody chain
- **Federation signature verification** — share package from instance A, tamper with one file before delivery to instance B, verify instance B rejects entire package
- **Federation custody bridge** — complete a federation share, verify instance B's custody chain includes federation receipt entry with RFC 3161 timestamp and reference to instance A's custody hash
- **Federation with unknown peer** — attempt to receive a package from an unregistered peer, verify rejection with clear error

### E2E Automated Tests (Playwright)

- **Enable per-case encryption** — as Case Admin, navigate to case settings, toggle "Per-case encryption" on, confirm dialog, verify status shows "Encryption: Enabled"
- **Upload evidence to encrypted case** — upload a file to an encryption-enabled case, verify upload succeeds; download the file, verify content matches original
- **Key rotation UI** — navigate to case encryption settings, click "Rotate Key", confirm dialog, verify progress indicator for re-encryption; after completion, download evidence and verify content intact
- **Federation peer management** — navigate to Federation > Peers, add a new peer with name, URL, and public key, verify peer appears in list with "Unverified" status
- **Federation share flow** — select evidence items, click "Share with Institution", select peer, review package contents, confirm; verify share appears in outgoing shares list with "Pending" status
- **Federation received packages** — on the receiving instance, navigate to Federation > Received, verify incoming package appears with verification status; accept the package; verify evidence items appear in the target case
- **Federation integrity failure** — (requires test harness to tamper with package) verify that a tampered package shows "Verification Failed" status with details about which file failed hash check

---

## Manual E2E Testing Checklist

1. [ ] **Action:** As Case Admin, open Case A settings and enable per-case encryption
   **Expected:** Confirmation dialog warns about re-encryption of existing evidence; after confirming, status changes to "Encryption: Enabled" with progress bar for existing file re-encryption
   **Verify:** Check case_encryption table shows case_id with enabled=true and key_source='derived'; if case had existing files, verify re-encryption job completes; download a previously uploaded file and confirm content is unchanged

2. [ ] **Action:** Upload a 50MB video file to the encryption-enabled Case A
   **Expected:** Upload succeeds normally; file stored in MinIO is encrypted (not readable as raw video)
   **Verify:** Directly access the file in MinIO (via MinIO console or mc CLI); confirm the raw bytes are not valid video; download via VaultKeeper UI; confirm the downloaded file plays correctly as video

3. [ ] **Action:** Initiate key rotation for Case A from the encryption settings page
   **Expected:** Key rotation starts with progress indicator; all evidence files in the case are re-encrypted with the new key version; key_version increments by 1
   **Verify:** Check case_encryption table shows incremented key_version and updated rotated_at timestamp; download 3 random evidence files; confirm all content matches originals; check custody log for key rotation entry

4. [ ] **Action:** Create two cases (A and B) both with per-case encryption enabled; upload the same file to both cases
   **Expected:** Both uploads succeed; the encrypted bytes stored in MinIO for Case A differ from those stored for Case B (different keys)
   **Verify:** Compare encrypted file sizes (should be similar) but raw bytes (should differ); download from both cases; both return the identical original file

5. [ ] **Action:** On VaultKeeper Instance A, navigate to Federation > Peers and add Instance B as a peer by entering its URL and public key (obtained from Instance B's admin panel)
   **Expected:** Peer appears in list; "Test Connection" button confirms reachability; status shows "Verified" after successful handshake
   **Verify:** Instance B's peer list also shows Instance A (if bidirectional registration was performed); connectivity test completes without error

6. [ ] **Action:** On Instance A, select 3 evidence items from a case, click "Share with Institution", select Instance B as the destination, and confirm the share
   **Expected:** Share package created with SHA-256 hashes for each file, custody chain, and Ed25519 signature; package transmitted to Instance B; outgoing share status updates to "Delivered"
   **Verify:** Check federation_shares table on Instance A shows status "completed"; check Instance B received the package

7. [ ] **Action:** On Instance B, navigate to Federation > Received and view the incoming package from Instance A
   **Expected:** Package shows verification status "Verified" (all file hashes match, signature valid); each evidence item listed with hash, metadata, and custody chain excerpt
   **Verify:** Click "Verify" to re-run integrity check; confirm all checks pass; accept the package; verify 3 new evidence items appear in the target case on Instance B

8. [ ] **Action:** On Instance B, open the custody chain for one of the federation-received evidence items
   **Expected:** Custody chain begins with "Received from [Instance A Name] via federation" entry, including RFC 3161 timestamp and a reference hash of Instance A's custody chain for that item
   **Verify:** The federation receipt entry includes the peer name, transfer timestamp, package hash, and custody chain reference; subsequent entries (if any) are from Instance B's own operations

9. [ ] **Action:** (Requires test harness) Simulate a tampered federation package by modifying one evidence file's bytes after signing but before delivery to Instance B
   **Expected:** Instance B's verification detects the hash mismatch; entire package is rejected; alert generated for both institutions
   **Verify:** Federation share status on Instance B shows "Verification Failed" with the specific file that failed; no evidence items were created from the tampered package; Instance A receives notification of rejection

10. [ ] **Action:** Attempt to send a federation package to Instance B from an unregistered Instance C (whose public key is not in Instance B's peer list)
    **Expected:** Instance B rejects the package immediately with "Unknown peer" error
    **Verify:** No evidence items created; federation_shares table shows rejection; no sensitive data from the rejected package persisted
