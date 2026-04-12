# Landing Page Credibility Overhaul

## Task Type
- [x] Frontend (primary — component + copy changes)
- [ ] Backend (no changes needed)
- [x] Fullstack (translations EN + FR)

## Problem Statement

The marketing landing page contains multiple credibility-destroying elements that would immediately disqualify VaultKeeper in the eyes of its target audience (NGO documentation leads, human rights legal case builders, ICC procurement officers):

1. **Fake stats** — 2.4M evidence items, 34 jurisdictions, 99.99% uptime, 18M audit events (product is pre-launch)
2. **False social proof** — "Trusted by investigation teams worldwide" (no users yet)
3. **Misleading compliance badges** — "SOC 2 Type II" without qualifier implies certification
4. **No Berkeley Protocol mention** — the #1 signal for this audience
5. **Sovereignty unexplained** — tagline says "sovereign" but never defines the threat model
6. **No AGPL-3.0/open-source mention** — major trust differentiator for NGOs absent
7. **RFC 3161 buried** — strongest legal-defensibility feature underemphasized
8. **FAQ only first item open** — 8 real answers hidden behind collapsed accordions
9. **CTA compliance badges misleading** — "ISO 27001 Ready / SOC 2 Type II / GDPR Compliant"

## Technical Solution

**Approach: Truth-first credibility architecture rewrite**

Remove every claim requiring unearned trust. Replace with verifiable, concrete signals: Berkeley Protocol alignment, RFC 3161 timestamping, AGPL-3.0 openness, sovereignty threat model.

### New Page Section Order
```
Hero (revised indicators + sovereignty sentence)
CredibilitySignals (replaces SocialProof + Stats)
Features (existing, minor copy tweaks)
HowItWorks (existing, unchanged)
OpenSource (new section)
FAQ (fix accordion defaults)
CTA (fix compliance badges)
```

### What Gets Removed
- `SocialProofSection` component — "Trusted by investigation teams worldwide" is false
- `StatsSection` component — all four counters are fabricated
- Unqualified compliance badges from CTA section

### What Gets Added
- `CredibilitySignalsSection` — 4 factual trust cards replacing stats/social proof
- `OpenSourceSection` — AGPL-3.0 trust signal with GitHub link
- Sovereignty explainer sentence in hero description
- Berkeley Protocol mention in hero indicators

### What Gets Modified
- Hero indicators: replace generic → Berkeley Protocol / RFC 3161 / AGPL-3.0
- Hero description: add CLOUD Act sovereignty framing
- CTA badges: soften to "Built for ISO 27001 & SOC 2 compliance. GDPR by design."
- FAQ: allow multiple open, default open sovereignty + security + pilot items
- FAQ security answer: soften compliance claim language
- All translations: EN + FR for every changed string

## Implementation Steps

### Step 1: Remove deceptive sections from page composition
**File**: `web/src/app/[locale]/(marketing)/page.tsx`
- Remove `SocialProofSection` and `StatsSection` imports and usage
- Add `CredibilitySignalsSection` and `OpenSourceSection` imports
- New order: Hero → CredibilitySignals → Features → HowItWorks → OpenSource → FAQ → CTA

### Step 2: Rewrite hero trust indicators
**File**: `web/src/messages/en.json` (marketing.hero section)
- `indicator1`: "End-to-end encryption" → "Berkeley Protocol-aligned"
- `indicator2`: "Immutable audit trails" → "RFC 3161 trusted timestamps"
- `indicator3`: "Court-ready exports" → "AGPL-3.0 open source"
- Same changes in `fr.json`

### Step 3: Add sovereignty framing to hero description
**File**: `web/src/messages/en.json` (marketing.hero.description)
- Append CLOUD Act framing: "Your data stays in your jurisdiction, on your infrastructure — never on third-party cloud providers subject to foreign legal access demands."
- Translate for `fr.json`

### Step 4: Create CredibilitySignalsSection component
**File**: `web/src/components/marketing/sections/credibility-signals-section.tsx` (new)
- 4 trust cards in a responsive grid:
  1. **Berkeley Protocol** — "Berkeley Protocol-aligned workflows for digital open-source investigations"
  2. **RFC 3161 Timestamps** — "Cryptographic proof of when evidence was sealed — court-admissible under international standards"
  3. **Sovereign Deployment** — "Your infrastructure, your jurisdiction. No exposure to the CLOUD Act or third-party data access regimes."
  4. **Immutable Audit Trails** — "Every access, transfer, and modification recorded. Tamper-evident chain of custody from intake to courtroom."
- Style: match existing feature card aesthetic, no fake numbers, no aspirational claims
- Add translation keys under `marketing.credibilitySignals`

### Step 5: Create OpenSourceSection component
**File**: `web/src/components/marketing/sections/open-source-section.tsx` (new)
- Heading: "Built in the open"
- Body: "VaultKeeper is AGPL-3.0 licensed. Every line of code can be inspected, audited, and self-hosted. For organizations that can't afford black-box trust assumptions, transparency isn't optional — it's the foundation."
- CTA: "View on GitHub" button (link to repo)
- Add translation keys under `marketing.openSource`

### Step 6: Fix CTA section (badges + description)
**File**: `web/src/messages/en.json` (marketing.cta section)
- `badge1`: "ISO 27001 Ready" → "Built for ISO 27001"
- `badge2`: "SOC 2 Type II" → "SOC 2 Type II roadmap"
- `badge3`: "GDPR Compliant" → "GDPR by design"
- `description`: "Join investigation teams that trust VaultKeeper for sovereign evidence management. Start with a free pilot — no commitment required." → "Sovereign evidence management for human rights documentation and legal case-building. Start with a free pilot — no commitment required."
  - Removes false "trust" claim and retargets to v1 audience
- Same in `fr.json`
- **Note**: CTA section is shared across 3 pages (landing, /features, /pricing) — translation fix propagates everywhere automatically

### Step 7: Fix FAQ accordion behavior
**File**: `web/src/components/marketing/sections/faq-section.tsx`
- Change from single-open accordion to multi-open
- Default open: sovereignty (index 0), security (index 1), pilot (index 3)
- State: `Set<number>` instead of `number | null`

### Step 8: Soften FAQ security answer
**File**: `web/src/messages/en.json` (marketing.faq.security)
- Current: "VaultKeeper is designed to meet the requirements of ISO 27001, SOC 2 Type II, and GDPR."
- New: "VaultKeeper is built for ISO 27001 and SOC 2 Type II compliance, with GDPR compliance by design. All data is encrypted at rest (AES-256) and in transit (TLS 1.3). Certification roadmap available on request."
- Same in `fr.json`

### Step 9: Update hero description to target NGO documentation leads
**File**: `web/src/messages/en.json` (marketing.hero)
- Reframe description from generic "investigation teams" to specifically address documentation leads and legal case builders
- Current: "VaultKeeper gives investigation teams authoritative control over evidence workflows..."
- New: "VaultKeeper gives human rights documentation teams and legal case builders authoritative control over evidence workflows — from secure intake and chain-of-custody tracking to court-ready disclosure packages. Your data stays in your jurisdiction, on your infrastructure."

### Step 10: Update French translations
**File**: `web/src/messages/fr.json`
- Mirror all EN changes for every modified translation key
- Maintain same tone and accuracy standards

### Step 11: Clean up unused components and dead code
- Delete: `stats-section.tsx`, `social-proof-section.tsx`
- Delete: `web/src/components/ui/animated-number.tsx` (only consumer was stats section — now dead code)
- Remove unused translation keys from both EN + FR: `marketing.socialProof`, `marketing.stats`

## Key Files

| File | Operation | Description |
|------|-----------|-------------|
| `web/src/app/[locale]/(marketing)/page.tsx` | Modify | Remove SocialProof/Stats, add CredibilitySignals/OpenSource |
| `web/src/components/marketing/sections/credibility-signals-section.tsx` | Create | 4 factual trust cards |
| `web/src/components/marketing/sections/open-source-section.tsx` | Create | AGPL-3.0 trust signal + GitHub link |
| `web/src/components/marketing/sections/faq-section.tsx` | Modify | Multi-open accordion, 3 defaults |
| `web/src/components/marketing/sections/cta-section.tsx` | No change | Badges driven by translations |
| `web/src/messages/en.json` | Modify | All marketing copy changes |
| `web/src/messages/fr.json` | Modify | French translations mirror |
| `web/src/components/marketing/sections/stats-section.tsx` | Delete | Fake stats removed |
| `web/src/components/marketing/sections/social-proof-section.tsx` | Delete | False claim removed |
| `web/src/components/ui/animated-number.tsx` | Delete | Dead code — only consumer was stats section |

## Risks and Mitigation

| Risk | Mitigation |
|------|------------|
| Copy claims "Berkeley Protocol-aligned" without defensible mapping | Use "aligned with" / "informed by", never "compliant" or "certified" |
| Removing stats/social proof makes page feel empty | Replace with CredibilitySignals section — denser, more trustworthy |
| FR translations may lose nuance | Review each FR string for accuracy, not just machine translation |
| Pricing section still has specific $ amounts | Keep as-is — pricing is real and contextualizes pilot-led motion well |
| Open source section needs real GitHub URL | Use placeholder until public repo is live, or link to current repo |

## Copy Constraints (Critical)
- **Never claim certification** unless externally verified
- **Never use "trusted by"** without named, real references
- **Never fabricate metrics** — use qualitative trust signals instead
- **Prefer standards and protocols** over adjectives ("RFC 3161" > "secure")
- **Berkeley Protocol**: use "aligned with" or "informed by", never "compliant"

## SESSION_ID (for /ccg:execute use)
- CODEX_SESSION: codex-1775999911-7408
- GEMINI_SESSION: N/A (quota exhausted)
