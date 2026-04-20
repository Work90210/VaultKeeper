# Implementation Plan: Formal Verification of the Chain-of-Custody Engine

## Task Type
- [x] Backend (→ Codex authority)
- [ ] Frontend (→ Gemini authority)
- [ ] Fullstack

---

## Executive Summary

Formally verify VaultKeeper's chain-of-custody engine to produce machine-checked proofs of correctness for the core invariants. This creates a defensibility moat: "formally verified" is a phrase that survives ICC-level cross-examination in ways "industry-standard" doesn't. No DEMS competitor has done this.

---

## When to Build This

This is speculative moat-building, not blocking any active deal. Prioritize accordingly.

**Build now if**: A specific procurement conversation (ICC OTP, national war crimes unit, ECCC successor mechanism) names "integrity guarantees" as an evaluation criterion, or if a competitor starts making formal claims about their custody chain. In that case, this becomes a 4-6 month sprint with a Coq consultant.

**Build later if**: Current priority is pilot traction, GovLens stability, and APIFold launch. The existing ~130-test suite with property-based coverage and RLS append-only enforcement is already stronger than anything in the DEMS market. Formal verification amplifies a lead you already have — it doesn't create the lead.

**Recommended trigger**: Start Phase 1 (TLA+, 3-4 weeks, learnable) as a nights-and-weekends project to build intuition and surface any protocol-level bugs. Defer Phase 2 (Coq, the hard part) until either (a) a deal justifies the investment, or (b) you have 3-4 months of focused time without competing deliverables.

**Review cadence**: Revisit this plan every 6 months, or when a procurement conversation names integrity guarantees as an evaluation criterion, whichever comes first. If neither trigger fires within 18 months of Phase 1 completion, archive the TLA+ artifacts and deprioritize. The model won't rot — TLA+ specs are stable — but the opportunity cost of holding mental context does.

---

## Threat Model

The proofs must be scoped to a specific attacker. Different attackers require different verification targets.

### Attacker Capabilities Matrix

| Attacker | Capabilities | What they can do | What the verification proves against them |
|----------|-------------|-----------------|------------------------------------------|
| **External attacker** (network access only) | SQL injection, application-layer exploits | Attempt to insert/modify custody entries via the API | Hash chain integrity detects any modification. RLS prevents direct mutation. Verified by Coq Theorems 1-5. |
| **Compromised application server** | Full application-role DB access, can execute arbitrary SQL as `vaultkeeper_app` | INSERT arbitrary rows, but cannot UPDATE/DELETE (RLS). Can attempt to forge hash values. | Hash chain integrity: forged hashes fail verification unless attacker can find SHA-256 collisions. Verified by Coq under collision-resistance axiom. |
| **Database administrator** | Superuser access, can bypass RLS, can UPDATE/DELETE rows | Modify or delete any row. Disable RLS. Alter schema. | **Partially out of scope.** Verification proves that tampering is *detectable* if the chain is re-verified from an independent copy. Does NOT prove tampering is *preventable* by a superuser. The threat model assumes RLS is intact (Axiom 2). |
| **Insider with migration key** | Can run schema migrations, potentially alter hash format | Modify hash computation rules, backfill hashes | **Out of scope for the formal model.** Mitigated operationally: hash_version field, migration audit trail, dual-approval deployment. The proof covers hash_version=1 semantics only. |
| **Physical storage attacker** | Direct disk access, can modify PostgreSQL data files | Byte-level modification of stored rows | Same as DB admin: detectable via re-verification from backup, not preventable. TSA timestamps provide external anchor. |

### What the Proofs Guarantee (Precisely)

Given a custody chain retrieved from the database:
1. If `VerifyCaseChain` returns `valid=true`, then no entry has been modified, deleted, inserted, or reordered **since the chain was constructed** — under the assumption that SHA-256 is collision-resistant.
2. If any entry was tampered with by an attacker who cannot compute SHA-256 preimages, `VerifyCaseChain` will return `valid=false` with the exact position and nature of the break.

### What the Proofs Do NOT Guarantee

- That tampering is *prevented* (only that it's *detected*)
- That the initial data entered into the chain was truthful
- That the system clock was accurate when timestamps were recorded
- That a superuser didn't disable RLS and modify rows (operational control, not algorithmic)
- That the TSA authority was honest (external trust assumption)

This distinction — **detection vs. prevention** — must be stated explicitly in all legal-facing documentation. The white paper's central claim is: "any tampering with the evidence chain is *detectable*," not "any tampering is *impossible*."

---

## Technical Analysis

### Tool Selection: TLA+ (Protocol) + Coq (Algorithm)

**Recommendation: Two-layer approach.**

| Layer | Tool | What it verifies | Why this tool |
|-------|------|-----------------|---------------|
| **Protocol/Concurrency** | TLA+ | Advisory locking, serialization, transaction atomicity, append-only enforcement | TLA+ excels at distributed/concurrent protocol verification. Industry precedent: Amazon (DynamoDB, S3, EBS all verified with TLA+). Model checker finds bugs in finite state spaces. Lower barrier to entry. |
| **Algorithm/Cryptographic** | Coq | Hash chain integrity, linear dependency, completeness, detail determinism | Coq produces machine-checked proofs with full mathematical rigor. Proofs are publishable artifacts. Code extraction to OCaml/Haskell possible. Academic gold standard for court-defensible claims. |

**Why not just one tool:**
- TLA+ alone: Can model-check finite instances but cannot prove universal properties ("for ALL chains of ANY length"). Model checking != proof.
- Coq alone: Poor at expressing concurrent database interactions and transaction semantics. Overkill for the locking protocol.
- Lean 4: Viable alternative to Coq with better tooling (modern language, mathlib). Consider if team has Lean experience. Less published precedent in audit/tamper-evident log verification.
- Isabelle/HOL: Strong but smaller community. Less code extraction tooling.

### What Exactly to Verify

**Tier 1 -- Core Algorithm (Coq, highest value):**
1. **Hash chain integrity**: For all chains and positions i, verify(chain) = true implies chain[i].hash = H(chain[i-1].hash, chain[i].fields)
2. **Tamper detection completeness**: For all chains, tampered(chain) implies verify(chain) = false
   - Field modification detection
   - Deletion detection (intermediate entry removal breaks subsequent hashes)
   - Insertion detection (spurious entry has wrong hash)
   - Reordering detection
3. **Linear dependency**: For all chains and positions i, chain[i].hash depends on chain[0..i-1]
4. **Canonicalization determinism** (scoped down -- see note below)

**Canonical JSON: Scoped proof target.** Do NOT attempt to prove semantic equivalence for arbitrary JSON. Instead, prove properties of VaultKeeper's specific `canonicalJSON` function operating on its specific input domain (Go `map[string]interface{}` with string/number/bool/null/nested-map values). The proof target is:

> For all inputs `m1`, `m2` of type `map[string]interface{}`, if `reflect.DeepEqual(m1, m2)` then `canonicalJSON(m1) = canonicalJSON(m2)`.

This is much smaller than general JSON semantic equivalence. It avoids the Unicode normalization, number representation, and whitespace handling rabbit holes. The Go stdlib guarantees sorted keys; we prove that our function preserves that property through the marshal-unmarshal-remarshal cycle.

**Tier 2 -- Concurrency Protocol (TLA+, high value):**
1. **Serialization safety**: Advisory lock ensures no two concurrent inserts to the same case produce inconsistent chains
2. **Atomicity**: Transaction boundaries guarantee hash is computed-then-inserted atomically
3. **Ordering preservation**: Chronological ordering maintained under concurrent writes

**Tier 3 -- Database Constraints (Out of scope for formal verification):**
- RLS append-only enforcement -> verified by PostgreSQL's RLS mechanism, not our code
- This is an operational guarantee, not an algorithmic one
- Document as an assumption/axiom in the formal model

### Verification Boundary

```
+-------------------------------------------------------------+
|                    FORMAL MODEL (Coq)                       |
|                                                             |
|  Abstract hash chain:                                       |
|    Event = record { id, fields... }                         |
|    Chain = list Event                                       |
|    ComputeHash : string -> Event -> string                  |
|    VerifyChain : Chain -> Prop                              |
|                                                             |
|  Theorems:                                                  |
|    chain_integrity, tamper_detection, linear_dependency,    |
|    canonicalization_determinism                              |
+----------------------+--------------------------------------+
                       |
                       |  CORRESPONDENCE (manual + tests)
                       |  NOTE: conformance testing is
                       |  statistical evidence, not proof.
                       |  See "Conformance Limitations" below.
                       |
+----------------------v--------------------------------------+
|                    GO IMPLEMENTATION                         |
|                                                             |
|  internal/custody/chain.go                                  |
|    ComputeLogHash()    <->  Coq ComputeHash                 |
|    VerifyCaseChain()   <->  Coq VerifyChain                 |
|    canonicalJSON()     <->  Coq canonicalize                |
|                                                             |
|  internal/custody/repository.go                             |
|    Insert()            <->  TLA+ InsertAction               |
|    advisory lock       <->  TLA+ Lock/Unlock                |
+-------------------------------------------------------------+
```

### Interaction with Ed25519 Attestation System

The chain-of-custody engine and the migration/integrity attestation system (`internal/integrity/`, `internal/migration/`) both provide integrity guarantees but operate at different levels:

| System | What it protects | Mechanism | Scope of this verification |
|--------|-----------------|-----------|---------------------------|
| Custody hash chain | Ordering and completeness of the audit trail | SHA-256 hash chaining | **Primary target** |
| TSA timestamps (`internal/integrity/tsa.go`) | Proof that evidence existed at a specific time | RFC 3161 timestamp authority | Out of scope (external trust) |
| Migration signing (`internal/migration/signing.go`) | Integrity of evidence during cross-system migration | Ed25519 signatures | Out of scope (separate invariants) |
| Integrity verifier (`internal/integrity/verifier.go`) | File-level hash verification of evidence items | SHA-256 file hash | Out of scope (complementary system) |

**Interaction point**: When the integrity verifier detects a hash mismatch, it records a custody event (`hash_mismatch`). The formal model should treat this as an ordinary append -- the proof covers "the custody chain correctly records that a mismatch was detected," not "the file hash verification itself is correct."

**Worked example for the white paper** (this distinction is subtle enough that it needs a concrete scenario):

> *Scenario: An evidence video file is corrupted on storage (bit rot or deliberate tampering). The integrity verifier runs its periodic check, computes the file's SHA-256 hash, finds it doesn't match the hash stored at upload time, and records a `hash_mismatch` custody event.*
>
> *What the formal verification covers: The custody chain now contains an entry saying "hash mismatch detected at time T by actor SYSTEM." The chain's integrity proof guarantees this entry cannot be silently removed, modified, or reordered. If someone later tries to delete the "hash_mismatch" event to conceal the corruption, chain verification will detect that deletion.*
>
> *What the formal verification does NOT cover: Whether the integrity verifier correctly computed the file hash. Whether the original hash stored at upload time was correct. Whether the file was actually corrupted vs. the verifier having a bug. These are properties of the integrity subsystem, not the custody chain.*
>
> *In court: "Your Honor, the formally verified custody chain proves that the system recorded a file integrity violation on [date]. The recording of that event is tamper-evident. Whether the underlying file verification was accurate is a separate technical question that we address through [testing/operational controls], not through formal proof."*

These systems are complementary, not overlapping. Verifying them together would expand scope dramatically for minimal additional legal value. The white paper should describe all three systems but only claim formal verification for the custody chain.

### Bridging the Verification Gap

The gap between "we proved the algorithm correct in Coq" and "the Go binary implements that algorithm" is the hardest part. **Three-pronged approach:**

1. **Structural correspondence** (documentation):
   - Line-by-line mapping between Coq definitions and Go functions
   - Published as part of the proof artifact
   - Reviewable by third-party auditors

2. **Conformance test suite** (automated):
   - Generate test vectors from Coq (extract -> OCaml -> run -> capture inputs/outputs)
   - Run same inputs through Go implementation
   - Property: for all test vectors, coq_output = go_output
   - Integrate into CI -- any Go change that breaks correspondence fails the build

3. **Runtime verification monitor** (belt-and-suspenders):
   - Lightweight runtime check: after every Insert, re-verify the last N entries
   - Already implemented in `VerifyCaseChain` -- extend to run as background job
   - If runtime monitor ever detects a break, it proves the Go code diverged from spec

#### Conformance Limitations (Must Be Documented)

Generating N test vectors from Coq and running them through Go proves behavioral equivalence on those N inputs. **It does not prove behavioral equivalence on all inputs.** A sufficiently adversarial input not covered by the vectors could still expose a divergence between the Coq model and Go implementation.

This is inherent to the approach -- without full code extraction (Coq does not extract to Go), we cannot achieve mathematical certainty of implementation correspondence. The conformance suite provides **high-confidence statistical evidence**, not proof.

The white paper must be explicit about this. The legally defensible claim is:

> "The chain-of-custody algorithm has been mathematically proven correct in Coq. The Go implementation has been validated against the proven model via [N] conformance test vectors and continuous runtime verification. No divergence has been detected."

NOT:

> "The Go implementation has been formally verified."

Opposing counsel will find this distinction if we don't state it first. Stating it first makes it a strength (intellectual honesty) rather than a weakness (caught concealing).

**Why not full code extraction (Coq -> Go)?**
- Coq extracts to OCaml/Haskell, not Go
- Extracted code would need a separate runtime, defeating the purpose
- The hash chain logic is ~100 lines of Go -- small enough for structural correspondence
- Full extraction is years of work for marginal benefit on this code size

---

## Implementation Steps

### Phase 0: Null-Byte Audit (Prerequisite, 1-2 days)

Before any formal work begins, validate Axiom 7 ("no custody event field contains `\x00`"). This axiom is load-bearing in the hash-input construction proof -- if it's wrong, an attacker could construct two distinct events with identical hash inputs by embedding null bytes that shift the separator boundaries.

**Audit scope**:
- Trace every code path that populates a custody `Event` struct: `RecordEvidenceEvent`, `RecordCaseEvent`, `Record`, and any direct `Insert` callers
- For each field (`Action`, `ActorUserID`, `Detail`, `EvidenceID.String()`, `CaseID.String()`): confirm that the input domain cannot contain `\x00`
- UUID fields: safe (hex encoding). Timestamps: safe (RFC3339). Action strings: safe (hardcoded literals). **Detail field**: this is the risk -- it's freeform JSON from callers. Confirm that the API layer rejects or strips null bytes in all JSON payloads that flow into custody Detail.
- Add a regression test: `TestNullByteRejection` that attempts to insert a custody event with `\x00` in each field and confirms rejection

**Deliverable**: Audit documented in a code comment on `ComputeHashInput`. Regression test in `custody_unit_test.go`.

---

### Phase 1: TLA+ Protocol Model (4-5 weeks)

**Step 1.0: TLA+ onboarding (1 week)**
- Install TLA+ Toolbox and TLC model checker
- Work through Lamport's "Specifying Systems" chapters 1-5 (the die hard water jug problem through to state machines)
- Model a toy mutual-exclusion protocol (e.g., Peterson's algorithm) to get comfortable with the tools
- Model a stripped-down version of just the advisory lock: two processes, one lock, acquire/release. Verify mutual exclusion. This is ~30 lines of TLA+ and gives you the mechanics before tackling the full protocol.

**Step 1.1: Model the custody insert protocol**
- State variables: `cases` (map case_id -> chain), `locks` (set of held locks), `pending_txns`
- Actions: `BeginTx`, `AcquireLock`, `GetLastHash`, `ComputeHash`, `Insert`, `CommitTx`, `AbortTx`
- Model concurrent writers (2-3 processes writing to same case)

**Step 1.2: Specify safety properties**
- `ChainConsistency`: After every committed insert, the chain is valid
- `NoLostUpdates`: No committed insert is overwritten or lost
- `Serialization`: Concurrent inserts to same case produce a valid total order

**Step 1.3: Specify liveness properties**
- `EventualInsert`: Every started insert eventually commits or aborts
- `LockFreedom`: No permanent deadlock on advisory locks (trivially true: single lock per case)

**Step 1.4: Run TLC model checker**
- Finite instances: 2 cases, 3 concurrent writers, chains up to length 5
- Check all safety/liveness properties
- If bugs found: document, fix Go implementation, re-run. See "Contingency" section.

**Deliverable**: `formal/tlaplus/CustodyProtocol.tla`, `formal/tlaplus/CustodyProtocol.cfg`

---

### Phase 2: Coq Hash Chain Proofs (10-16 weeks)

_Timeline assumes Coq is new. If you've done Coq before, cut by 40%._

**Step 2.1: Coq onboarding (2-3 weeks)**
- Work through "Software Foundations" Volume 1 (Logical Foundations) chapters 1-8
- Build and prove a toy hash chain (3-5 entry fixed list) to get the mechanics down
- This is not wasted time -- the toy proof becomes the scaffold for the real one

**Step 2.2: Define abstract types and hash function (1-2 weeks)**
```
(* Pseudo-Coq *)
Record Event := {
  event_id : UUID;
  case_id : UUID;
  evidence_id : option UUID;
  action : string;
  actor : string;
  detail : CanonicalJSON;
  timestamp : Timestamp;
}.

(* SHA-256 modeled as injective for proof mechanics.
   Real SHA-256 maps arbitrary-length inputs to 256-bit outputs,
   so collisions must exist by pigeonhole. The security claim
   is that they're computationally infeasible to find, not that
   they don't exist. We axiomatize injectivity (stronger than
   collision resistance) because it yields cleaner inductive proofs.

   WHITE PAPER DEFENSE SENTENCE (use verbatim):
   "We model SHA-256 as an injective function in the proof;
   in reality, SHA-256 is collision-resistant under standard
   cryptographic assumptions, which is sufficient for the
   security claim that tampering is computationally infeasible
   to conceal rather than mathematically impossible to conceal.
   This idealization is standard in formal verification of
   cryptographic protocols (cf. HACL*, miTLS, Verified SCrypt)
   and does not weaken the practical security guarantee."

   An expert witness with a cryptography background will note
   the pigeonhole gap. The defense is that collision resistance
   is the standard assumption in ALL deployed cryptographic
   systems, and that finding a SHA-256 collision for a
   specifically-structured custody event (with UUID, timestamp,
   and actor constraints) is harder than finding a generic
   collision. *)
Parameter H : list byte -> hash_value.
Axiom H_injective :
  forall x y, H x = H y -> x = y.

Definition compute_hash_input (prev: hash_value) (e: Event) : list byte :=
  concat_null_sep [prev; e.event_id; e.actor; e.action; ...].

Definition compute_hash (prev: hash_value) (e: Event) : hash_value :=
  H (compute_hash_input prev e).
```

**Step 2.3: Define chain validity and verification (1-2 weeks)**
```
Fixpoint valid_chain (chain: list (Event * hash_value * hash_value)) : Prop :=
  match chain with
  | [] => True
  | [(e, hash, prev_hash)] =>
      hash = compute_hash "" e /\ prev_hash = ""
  | (e, hash, prev_hash) :: ((_, prev_h, _) :: _ as rest) =>
      hash = compute_hash prev_h e
      /\ prev_hash = prev_h
      /\ valid_chain rest
  end.
```

**Step 2.4: Prove tamper detection theorems (3-4 weeks)**

This is the core. Budget generously.

- `Theorem field_tamper_detected`: Modifying any field of any entry in a valid chain makes it invalid
- `Theorem deletion_detected`: Removing any intermediate entry from a valid chain makes it invalid
- `Theorem insertion_detected`: Inserting a spurious entry into a valid chain makes it invalid (unless attacker can invert H)
- `Theorem reorder_detected`: Swapping any two entries in a valid chain makes it invalid

**Step 2.5: Prove linear dependency (1 week)**

- `Theorem linear_dependency`: For any entry at position i, its hash value is a function of all entries at positions 0..i

**Step 2.6: Prove canonicalization properties (2-3 weeks)**

Scoped to VaultKeeper's specific canonicalJSON, NOT arbitrary JSON semantic equivalence.

- Model `CanonicalJSON` as sorted association list `list (string * json_value)` where `json_value` is a simple inductive type (string | number | bool | null | nested CanonicalJSON)
- `Theorem canonical_deterministic`: `canonicalize m1 = canonicalize m2` when `m1` and `m2` have the same key-value pairs regardless of insertion order
- `Theorem canonical_stable`: `canonicalize (canonicalize m) = canonicalize m` (idempotent)
- Do NOT model: Unicode normalization, floating-point edge cases, escape sequence variants. These are tested empirically in the conformance suite.

**Step 2.7: Prove verification soundness/completeness (1 week)**

- `Theorem verify_sound`: `valid_chain chain -> verify chain = true`
- `Theorem verify_complete`: `verify chain = true -> valid_chain chain`

**Deliverable**: `formal/coq/CustodyTypes.v`, `formal/coq/CustodyChain.v`, `formal/coq/CustodyJSON.v`, `formal/coq/CustodyVerify.v`, `formal/coq/Makefile`

---

### Phase 3: Conformance Bridge (2-3 weeks)

**Step 3.1: Extract test vectors from Coq**
- Use Coq's extraction mechanism to generate OCaml code
- Write OCaml driver that produces JSON test vectors: `{input_events: [...], expected_hashes: [...], expected_verification: bool}`
- Generate ~1000 vectors covering: empty chains, single entry, multi-entry, tampered fields, deleted entries, reordered entries, edge-case JSON

**Step 3.2: Go conformance test**
- `formal/conformance_test.go`: Reads Coq-generated vectors, runs through Go `ComputeLogHash` and `VerifyCaseChain`
- CI integration: `go test ./formal/...`
- Any divergence = build failure

**Step 3.3: Structural correspondence document**
- Markdown mapping: each Coq definition <-> Go function with line numbers
- Highlight any simplifications or assumptions in the Coq model
- Explicit section: "What the Coq model simplifies relative to Go"
- Publishable as part of proof artifact

**Deliverable**: `formal/vectors/`, `formal/conformance_test.go`, `formal/CORRESPONDENCE.md`

---

### Phase 4: Documentation & Publication (3-4 weeks)

**Step 4.1: White paper first, everything else derived from it**

Write the white paper BEFORE any marketing materials. All claims, badges, and procurement summaries are derived from the white paper's precise language, never invented independently.

- Title: "Formally Verified Chain-of-Custody: Mathematical Guarantees for Digital Evidence Admissibility"
- Target audience: judges, legal counsel, ICC tribunal technical advisors
- Structure:
  1. Plain-language explanation (what does "formally verified" mean?)
  2. Threat model (what attackers are covered, what aren't)
  3. What was proved, with precise claim language
  4. What was assumed (axioms section, including SHA-256 idealization defense)
  5. What was NOT proved (verification gap, conformance limitations)
  6. Worked examples (hash_mismatch scenario, tamper detection scenario)
  7. Technical summary (for expert witnesses)
  8. Full proof artifact reference
- Key phrase: "machine-checked mathematical proof that any tampering with the evidence chain is *detectable*"
- Anti-pattern: never say "tamper-proof" or "impossible to tamper with"

**Step 4.1.1: White paper review (non-negotiable)**

The white paper's entire value depends on precise language. Two named reviewers before publication:

1. **Evidence law specialist**: Dutch or Belgian lawyer familiar with digital evidence admissibility in international criminal proceedings (EU jurisdiction aligns with AGPL/sovereignty positioning). Reviews: claim language, threat model framing, courtroom defensibility of the detection-vs-prevention distinction. Budget: EUR 2-3K for a focused review of a 20-30 page document.

2. **Academic cryptographer**: Reviews: SHA-256 axiomatization defense, collision-resistance vs. injectivity distinction, whether the axioms section would survive expert cross-examination. Can be the same person engaged for Step 4.4 academic review, or a separate lightweight engagement. Budget: EUR 1-2K.

Total white paper review budget: EUR 3-5K. This is not optional -- it's the cheapest insurance in the entire plan.

**Step 4.2: Proof artifact package**
- `formal/README.md`: What was verified, what assumptions were made, how to reproduce
- `formal/BUILD.md`: Instructions to build and check proofs (Coq version, dependencies)
- Docker image that checks proofs from scratch (reproducibility)

**Step 4.3: Derived marketing materials**
- "Formally Verified" badge for product UI -- links to axioms page, not just a badge
- One-page procurement summary -- states "the chain-of-custody engine has been machine-checked against specified properties, assuming [axioms]"
- Comparison table: VaultKeeper (formally verified) vs competitors (self-attested)
- All materials reviewed against white paper for overclaim

**Step 4.4: Third-party academic review (deferred)**

This is real money (EUR 20-50K) and real time (3-6 months). Do NOT include in the initial plan timeline. Instead:

- **Immediate**: Package proofs for reproducibility (Docker image, clear build instructions)
- **When justified by deal size**: Engage formal verification research group (INRIA, Cambridge, Imperial, Data61) for independent review
- **Alternative**: Academic collaboration (co-authorship on a peer-reviewed paper in exchange for review -- lower cost, slower, but higher credibility)
- **Target venues if publishing**: IEEE S&P, ACM CCS, Digital Investigation journal, or DFRWS

**Deliverable**: White paper, `formal/` package, Docker image

---

### Phase 5: CI Integration & Maintenance (1 week)

**Step 5.1: CI pipeline**
- GitHub Action: `make verify` checks Coq proofs compile
- GitHub Action: `go test ./formal/...` runs conformance vectors
- Block merges to `internal/custody/chain.go` unless conformance passes

**Step 5.2: Change protocol**
- Any change to `ComputeLogHash` or `VerifyCaseChain` requires updating Coq model
- Conformance test vectors regenerated after Coq changes
- Document this in CONTRIBUTING.md

---

## Contingency: What If the Proof Reveals a Bug?

This is not hypothetical. Formal verification routinely surfaces bugs that testing misses -- that's the point.

### If TLA+ finds a concurrency bug (Phase 1)

**Likely scenario**: Edge case in advisory lock acquisition/release ordering under transaction abort.

**Response**:
1. Document the bug with TLC counterexample trace
2. Fix the Go implementation (`repository.go`)
3. Re-run TLC to confirm fix
4. Add integration test that reproduces the counterexample
5. No migration needed -- concurrency bugs affect future writes, not existing data

### If Coq proof reveals a hash chain bug (Phase 2)

**Likely scenario**: `canonicalJSON` is not fully deterministic for some edge-case input (nested empty objects, unicode edge cases, number formatting). Or: the null-byte separator doesn't fully prevent injection when a field contains `\x00`.

**Response -- depends on whether existing chains are affected**:

**Case A: Bug only affects hypothetical inputs, no existing chains are broken**
1. Fix the Go implementation
2. Add regression test
3. Re-run conformance suite
4. No migration needed

**Case B: Bug affects existing chains -- some chains would fail re-verification under the corrected algorithm**

This is the hard case. Options:

1. **Hash versioning** (recommended): Bump `hash_version` to 2. Existing chains (v1) are verified under v1 rules. New entries use v2. The Coq proof covers v2. The white paper states: "Chains constructed under hash_version >= 2 are formally verified. Legacy chains (v1) are verified under the original algorithm, which has been empirically validated."

2. **Re-hash migration**: Re-compute all hashes under the corrected algorithm. This is a breaking change -- every existing hash value changes. Requires: (a) full case-by-case re-verification, (b) new TSA timestamps for the re-hashed chains, (c) documentation that the migration occurred and why. Legal risk: opposing counsel argues the re-hash itself is tampering. Avoid if possible.

3. **Accept the bug as a known limitation**: If the bug is purely theoretical (requires inputs that VaultKeeper's application layer can never produce), document it as a known limitation in the axioms section. This is honest and may be the pragmatic choice.

**In all cases**: The discovery itself is a feature, not a failure. "Our formal verification effort discovered and corrected a subtle edge case that no testing methodology would have found" is a stronger story than "we found no bugs."

---

## Key Files

| File | Operation | Description |
|------|-----------|-------------|
| `internal/custody/chain.go:71-86` | Reference | `ComputeLogHash` -- core hash function to model in Coq |
| `internal/custody/chain.go:14-69` | Reference | `VerifyCaseChain` -- verification logic to prove correct |
| `internal/custody/chain.go:88-103` | Reference | `canonicalJSON` -- determinism to prove |
| `internal/custody/repository.go:37-89` | Reference | `Insert` with advisory locking -- model in TLA+ |
| `internal/custody/models.go:1-35` | Reference | Type definitions -- map to Coq records |
| `internal/integrity/tsa.go` | Reference | TSA integration -- out of scope, document interaction |
| `internal/migration/signing.go` | Reference | Ed25519 attestation -- out of scope, document interaction |
| `formal/tlaplus/CustodyProtocol.tla` | Create | TLA+ protocol specification |
| `formal/coq/CustodyTypes.v` | Create | Coq abstract types and hash axiomatization |
| `formal/coq/CustodyChain.v` | Create | Coq hash chain proofs |
| `formal/coq/CustodyJSON.v` | Create | Coq canonicalization proofs (scoped) |
| `formal/coq/CustodyVerify.v` | Create | Coq verification soundness/completeness |
| `formal/conformance_test.go` | Create | Go <-> Coq conformance test |
| `formal/CORRESPONDENCE.md` | Create | Structural mapping document |
| `formal/THREAT_MODEL.md` | Create | Attacker capabilities and proof coverage |

---

## Risks and Mitigation

| Risk | Severity | Mitigation |
|------|----------|------------|
| **Overclaiming** -- marketing says "verified" when proof says "model-checked under assumptions" | **Critical** | White paper first, all materials derived from it. Legal review before publication. Explicit axioms and limitations sections. |
| **Proof reveals implementation bug** in existing chains | **High** | Hash versioning (v1 legacy, v2 verified). Re-hash migration as last resort. See Contingency section. |
| **Coq learning curve** -- 6 weeks becomes 6 months | **High** | Start with Phase 1 (TLA+, lower barrier). Evaluate Coq difficulty after toy proof. Budget for consultant if self-learning stalls. Lean 4 as fallback (better tooling, steeper community curve). |
| **Canonical JSON proof explodes in scope** | **High** | Scoped to VaultKeeper's specific function and input domain. NOT arbitrary JSON semantic equivalence. Empirical edge cases tested in conformance suite, not proved. |
| **Verification gap exploited by opposing counsel** | **Medium** | Conformance limitations stated explicitly in white paper. Three-pronged bridge. Intellectual honesty is the defense. |
| **SHA-256 modeling in Coq** | **Medium** | Abstract injective function with collision-resistance axiom. Standard practice (HACL*, seL4). |
| **Go code diverges from Coq model after future changes** | **Medium** | Conformance test suite in CI. Block merges that break conformance. |
| **Model assumptions invalidated** (e.g., Go json.Marshal key ordering changes) | **Low** | Pin Go version. Conformance tests catch behavioral changes. |
| **Third-party review too expensive** | **Low** | Defer until deal justifies cost. Academic collaboration as lower-cost alternative. |

---

## Effort Estimates (Honest)

| Phase | Duration (Coq-experienced) | Duration (learning Coq) | Prerequisites | Notes |
|-------|---------------------------|------------------------|---------------|-------|
| Phase 0: Null-byte audit | 1-2 days | 1-2 days | None | Prerequisite for all formal work. Validates Axiom 7. |
| Phase 1: TLA+ Protocol | 4-5 weeks | 4-5 weeks | TLA+ Toolbox, TLC | Includes 1 week TLA+ onboarding. Good starting point -- gentler learning curve than Coq. |
| Phase 2: Coq Proofs | 6-10 weeks | 10-16 weeks | Coq 8.18+, coqc | Hardest phase. Includes 2-3 weeks Coq onboarding if learning. Canonical JSON sub-proof is 2-3 weeks alone even scoped down. |
| Phase 3: Conformance Bridge | 2-3 weeks | 2-3 weeks | Phase 2 complete | Mechanical but important. |
| Phase 4: Documentation | 3-4 weeks | 3-4 weeks | Phases 1-3 complete | White paper is the bottleneck. Includes EUR 3-5K for legal + crypto reviewers. Can partially overlap with Phase 3. |
| Phase 5: CI Integration | 1 week | 1 week | Phases 1-3 complete | Straightforward. |

**Total with Coq experience: ~16-24 weeks (4-6 months)**
**Total learning Coq: ~20-30 weeks (5-7.5 months)**
**Total nights-and-weekends (learning Coq, ~15 hrs/week): ~8-12 months**

These estimates do NOT include third-party academic review (add 3-6 months and EUR 20-50K if pursued).

**Budget**: EUR 3-5K for white paper reviewers (non-optional). EUR 20-50K for academic review (deferred). Zero tooling cost (Coq and TLA+ are open source).

---

## Prior Art to Build On

| Work | Relevance | How to use |
|------|-----------|------------|
| **Amazon TLA+ papers** (Newcombe et al., 2015) | TLA+ for real production systems | Pattern for Phase 1 protocol modeling |
| **HACL\*** (INRIA) | Verified crypto in F* | Reference for modeling hash functions abstractly; SHA-256 axiomatization precedent |
| **Verdi** (UW) | Verified distributed systems in Coq | Patterns for modeling concurrent state machines |
| **Tezos blockchain verification** (Nomadic Labs, Coq) | Hash chain properties verified in Coq specifically | Closest direct precedent: append-only blockchain state verified in Coq. Reuse proof structure for hash chain integrity and tamper detection lemmas. Published work on verified smart contracts provides additional chain-of-state patterns. |
| **Bitcoin Lightning Network channel state papers** | Verified append-only log with external trust anchor | The channel state is structurally similar to a custody chain: ordered, hash-linked, with an external anchor (blockchain) analogous to TSA timestamps. Useful for the white paper's "prior art in verified append-only state" section. |
| **Certificate Transparency** (Google) | Merkle tree append-only log with formal analysis | Similar append-only tamper-evident structure |
| **seL4** (NICTA/Data61) | Full OS kernel verification | Gold standard for "verified binary" claim scoping |
| **CompCert** (INRIA, Coq) | Verified C compiler | Precedent for executable trust arguments and the "verified binary" marketing claim boundary |

---

## Assumptions / Axioms in the Formal Model

These are explicitly NOT verified -- they are assumed correct:

1. **SHA-256 is collision-resistant** (standard cryptographic assumption; modeled as injective function which is strictly stronger -- documented as idealization)
2. **PostgreSQL RLS correctly enforces append-only** (operational guarantee; superuser bypass is out of scope)
3. **PostgreSQL advisory locks provide mutual exclusion under READ COMMITTED** (database guarantee)
4. **Go's `json.Marshal` produces sorted keys** (Go stdlib guarantee since 1.12; pinned via go.mod)
5. **Go's `crypto/sha256` correctly implements SHA-256** (stdlib guarantee; FIPS 140-2 reference)
6. **Timestamps have microsecond precision and UTC normalization** (application convention enforced in `ComputeLogHash`)
7. **No field in a custody event contains the null byte `\x00`** (separator safety; validated by Phase 0 null-byte audit and enforced by regression test at the API layer)

Each assumption is documented in the Coq development with an explicit `Axiom` or `Parameter` declaration. The white paper's axioms section mirrors this list verbatim.

---

## SESSION_ID (for /ccg:execute use)
- CODEX_SESSION: codex-1776321922-46970
- GEMINI_SESSION: 27 (quota exhausted)
