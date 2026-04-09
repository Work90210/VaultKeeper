# Implementation Plan: VaultKeeper Marketing Landing Site & Pilot Registration

## Task Type
- [x] Frontend (cult-ui Pro components, pages, design)
- [x] Backend (pilot registration API endpoint)
- [x] Fullstack (route restructuring, i18n, SEO)

---

## Technical Solution

**Architecture**: Single Next.js app with route-group separation — `(marketing)` for public pages, `(app)` for the existing authenticated product. Marketing pages use cult-ui Pro blocks customized to VaultKeeper's "Archive" design system. Pilot registration uses a public API endpoint with zod validation, rate limiting, and DB-backed storage.

**Hero**: `Hero Apple` — best match for institutional authority. Refined animations, editorial hierarchy, serif headings. Subdued stone/amber palette, no startup energy.

**Component Strategy**: Download cult-ui Pro blocks as ZIP, extract into `web/src/components/marketing/`, customize colors/typography to match The Archive design tokens.

---

## Implementation Steps

### Phase 1: Route Restructuring (Foundation)

#### Step 1.1 — Create route groups
Reorganize `web/src/app/[locale]/` into two route groups:

```
web/src/app/[locale]/
├── (marketing)/          ← NEW: public pages
│   ├── layout.tsx        ← Marketing shell (header + footer)
│   ├── page.tsx           ← Landing / Home
│   ├── features/page.tsx
│   ├── pricing/page.tsx
│   ├── about/page.tsx
│   └── contact/page.tsx
├── (app)/                ← MOVED: existing authenticated pages
│   ├── layout.tsx        ← Existing auth shell (sidebar + header)
│   ├── cases/...
│   ├── evidence/...
│   ├── witnesses/...
│   ├── disclosures/...
│   ├── notifications/...
│   ├── search/...
│   └── settings/...
└── login/page.tsx        ← Keep at root level
```

- Move existing `cases/`, `evidence/`, `witnesses/`, `disclosures/`, `notifications/`, `search/`, `settings/` into `(app)/`
- The `(app)` layout wraps with auth guard, sidebar, and authenticated header
- The `(marketing)` layout wraps with marketing header and footer
- Route groups don't affect URLs — existing routes remain unchanged

#### Step 1.2 — Update middleware
Ensure auth middleware only protects `(app)` routes. Marketing routes must be fully public.

```ts
// middleware.ts — protected route matchers
const protectedPaths = [
  "/:locale/cases",
  "/:locale/evidence", 
  "/:locale/witnesses",
  "/:locale/disclosures",
  "/:locale/notifications",
  "/:locale/search",
  "/:locale/settings"
];
```

#### Step 1.3 — Marketing layout
```tsx
// (marketing)/layout.tsx
export default function MarketingLayout({ children }) {
  return (
    <>
      <MarketingHeader />
      <main>{children}</main>
      <MarketingFooter />
    </>
  );
}
```

**Deliverable**: Route groups working, existing app unaffected, marketing pages accessible without auth.

---

### Phase 2: Design Infrastructure

#### Step 2.1 — Install cult-ui Pro dependencies
```bash
pnpm dlx shadcn@latest add badge button card input label separator textarea navigation-menu slider
```

Additional deps for animated components:
```bash
pnpm add framer-motion lucide-react
```

#### Step 2.2 — Create marketing component directory
```
web/src/components/marketing/
├── layout/
│   ├── marketing-header.tsx      ← Nav Apple or Nav Floating
│   ├── marketing-footer.tsx      ← Footer Apple
│   └── locale-switcher.tsx
├── hero/
│   └── hero-section.tsx          ← Hero Apple, customized
├── sections/
│   ├── features-section.tsx      ← Features Regal or Features Modern
│   ├── stats-section.tsx         ← Stats Hex
│   ├── social-proof-section.tsx  ← Social Proof Apple
│   ├── how-it-works-section.tsx  ← Process Heerich or Roadmap
│   ├── pricing-section.tsx       ← Pricing Apple
│   ├── faq-section.tsx           ← FAQ Apple
│   └── cta-section.tsx           ← CTA Hex or CTA Vite
└── forms/
    ├── pilot-registration-form.tsx
    └── pilot-registration-schema.ts
```

#### Step 2.3 — Extend design tokens for marketing
Add marketing-specific CSS variables that extend The Archive system:

```css
/* Marketing-specific extensions */
:root {
  --marketing-hero-bg: var(--stone-950);
  --marketing-hero-text: var(--stone-50);
  --marketing-section-gap: clamp(4rem, 3rem + 3vw, 8rem);
  --marketing-max-width: 1280px;
  --marketing-navy: oklch(0.250 0.040 260);  /* deep navy anchor */
  --marketing-navy-light: oklch(0.350 0.050 260);
}
```

**Deliverable**: Component structure ready, design tokens extended, cult-ui blocks downloaded.

---

### Phase 3: Core Marketing Components

#### Step 3.1 — Navigation (`Navigation Apple` or `Nav Floating`)
- Logo (VaultKeeper shield) + wordmark
- Links: Features, Pricing, About, Contact
- Locale switcher (EN/FR)
- CTA button: "Request Pilot Access"
- Transparent on hero, solid on scroll (glass blur)
- Mobile: hamburger with slide-out

#### Step 3.2 — Hero Section (`Hero Apple`)
- **Eyebrow badge**: "Sovereign Evidence Management"
- **Headline**: "Where custody, provenance, and disclosure are beyond question."
- **Subheadline**: "VaultKeeper gives investigation teams authoritative control over evidence workflows — from secure intake to court-ready disclosure."
- **Primary CTA**: "Request Pilot Access" → /contact
- **Secondary CTA**: "Explore Capabilities" → /features
- Subtle background: muted gradient (stone-950 → navy) or restrained animation
- Optional: product screenshot or abstract institutional visual

#### Step 3.3 — Footer (`Footer Apple`)
- VaultKeeper logo + tagline
- Link columns: Product, Company, Legal
- Locale switcher
- Copyright + compliance badges
- Minimal, institutional tone

**Deliverable**: Header, hero, and footer complete. Landing page has basic structure.

---

### Phase 4: Landing Page Sections

#### Step 4.1 — Social Proof Section (`Social Proof Apple`)
- Logos of institution types (courts, prosecution offices, defense firms)
- Testimonial cards with role/organization attribution
- Trust signals: "Used in 12+ jurisdictions"
- Muted, typography-led — no flashy animations

#### Step 4.2 — Stats Section (`Stats Hex`)
- Key metrics: evidence items managed, jurisdictions served, uptime, audit events
- Chamfered panels matching hex design language
- Animated counters on scroll-into-view
- Numbers should feel institutional, not startup-vanity

#### Step 4.3 — Features Grid (`Features Regal` or `Features Modern`)
- 6 core capabilities as bento cards:
  1. **Evidence Intake** — Secure upload with hash verification & trusted timestamping
  2. **Chain of Custody** — Immutable audit trail for every access, transfer, and modification
  3. **Role-Based Access** — Granular permissions by case, role, and classification level
  4. **Disclosure Management** — Structured workflows for prosecution/defense disclosure obligations
  5. **Redaction & Review** — Collaborative PDF redaction with version control
  6. **Search & Intelligence** — Full-text search, filtering, and evidence correlation
- Spring-based hover interactions, scroll-triggered reveals
- Each card links to detailed features page section

#### Step 4.4 — How It Works (`Process Heerich` or `Roadmap`)
- 3-4 step process:
  1. **Ingest** — Upload evidence with automated integrity verification
  2. **Organize** — Classify, tag, and assign to cases with full audit trails
  3. **Collaborate** — Review, redact, and prepare with role-based controls
  4. **Disclose** — Generate court-ready disclosure packages with chain-of-custody proof
- Visual timeline or step progression

#### Step 4.5 — FAQ Section (`FAQ Apple`)
- 8-10 questions covering:
  - Data sovereignty & hosting options
  - Security certifications & compliance
  - Integration with existing systems
  - Pilot program details & eligibility
  - Pricing model
  - Multi-language support
  - Training & onboarding
  - Data migration from existing systems

#### Step 4.6 — CTA Section (`CTA Hex`)
- "Ready to secure your evidence workflows?"
- Primary: "Request Pilot Access"
- Secondary: "Schedule a Demo"
- Trust line: compliance/security badges

**Deliverable**: Complete landing page with all sections.

---

### Phase 5: Additional Pages

#### Step 5.1 — Features Page (`/features`)
```tsx
<PageIntro title="Capabilities" subtitle="..." />
<FeaturesDetailedGrid />     // Expanded version with descriptions
<EvidenceLifecycleFlow />     // Visual workflow diagram
<SecurityArchitecture />      // Trust & sovereignty details
<IntegrationsSection />       // API, SSO, storage backends
<CtaSection />
```

#### Step 5.2 — Pricing Page (`/pricing`)
Using `Pricing Apple` with 3 tiers:
- **Pilot** — Free, limited cases, single team, community support
- **Professional** — Per-seat, unlimited cases, advanced features, priority support
- **Enterprise** — Custom, dedicated infrastructure, SLA, compliance packages
- All CTAs lead to contact form (enterprise sales model)
- Feature comparison table below tiers

#### Step 5.3 — About Page (`/about`)
- Mission statement: evidence integrity in justice systems
- Product principles (sovereignty, auditability, precision)
- Security posture & compliance commitments
- Team section (optional, use `Teams` cult-ui block)

#### Step 5.4 — Contact / Pilot Registration Page (`/contact`)
Using `Contact Apple` or custom form section:
- Form fields: Name, Email, Organization, Role (dropdown), Message
- Role options: Investigator, Prosecutor, Defence Counsel, Forensic Analyst, Court Administrator, Other
- Trust copy: "Your information is handled with the same care we bring to evidence management."
- Success state: confirmation message with expected response timeline

**Deliverable**: All 5 pages complete.

---

### Phase 6: Pilot Registration Backend

#### Step 6.1 — Validation schema (shared)
```ts
// web/src/lib/pilot-registration-schema.ts
const pilotRegistrationSchema = z.object({
  name: z.string().min(2).max(100),
  email: z.string().email(),
  organization: z.string().min(2).max(160),
  role: z.enum(["investigator", "prosecutor", "defense", "analyst", "court_admin", "other"]),
  message: z.string().min(20).max(2000),
  locale: z.enum(["en", "fr"])
});
```

#### Step 6.2 — API endpoint
```ts
// web/src/app/api/pilot/route.ts
export async function POST(request: Request) {
  // 1. Parse & validate with zod
  // 2. Rate limit (by IP hash, 5 submissions/hour)
  // 3. Honeypot field check
  // 4. Save to database (pilot_registrations table)
  // 5. Send notification (email/webhook)
  // 6. Return success
}
```

#### Step 6.3 — Database migration (Go backend)
```sql
-- migrations/015_pilot_registrations.up.sql
CREATE TABLE pilot_registrations (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name VARCHAR(100) NOT NULL,
  email VARCHAR(255) NOT NULL,
  organization VARCHAR(160) NOT NULL,
  role VARCHAR(50) NOT NULL,
  message TEXT NOT NULL,
  locale VARCHAR(5) NOT NULL DEFAULT 'en',
  source VARCHAR(50) NOT NULL DEFAULT 'marketing-site',
  ip_hash VARCHAR(64),
  user_agent TEXT,
  utm_source VARCHAR(100),
  utm_medium VARCHAR(100),
  utm_campaign VARCHAR(100),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  reviewed_at TIMESTAMPTZ,
  reviewed_by UUID REFERENCES users(id),
  status VARCHAR(20) NOT NULL DEFAULT 'pending'
);

CREATE INDEX idx_pilot_registrations_email ON pilot_registrations(email);
CREATE INDEX idx_pilot_registrations_status ON pilot_registrations(status);
CREATE INDEX idx_pilot_registrations_created ON pilot_registrations(created_at DESC);
```

#### Step 6.4 — Go handler for pilot registrations
Add handler in Go backend to:
- Accept POST from Next.js API route (or directly from form)
- Validate, persist, and respond
- Admin endpoint to list/review registrations

**Deliverable**: Working pilot registration flow with persistence.

---

### Phase 7: i18n Content

#### Step 7.1 — Marketing message namespaces
```
web/messages/
├── en/
│   ├── marketing.json      ← All marketing page content
│   └── pilot.json           ← Pilot form labels, validation messages
├── fr/
│   ├── marketing.json
│   └── pilot.json
```

#### Step 7.2 — Content structure
```json
{
  "hero": {
    "badge": "Sovereign Evidence Management",
    "title": "Where custody, provenance, and disclosure are beyond question.",
    "description": "VaultKeeper gives investigation teams authoritative control...",
    "primaryCta": "Request Pilot Access",
    "secondaryCta": "Explore Capabilities"
  },
  "features": { "..." : "..." },
  "pricing": { "..." : "..." },
  "faq": { "..." : "..." },
  "contact": { "..." : "..." }
}
```

**Deliverable**: Full English and French content for all marketing pages.

---

### Phase 8: SEO & Performance

#### Step 8.1 — Metadata per page
- `generateMetadata()` with locale-aware title, description, OG tags
- `hreflang` alternates for en/fr
- Canonical URLs

#### Step 8.2 — Sitemap & robots
- Sitemap includes only public marketing pages
- robots.txt excludes `/*/cases`, `/*/evidence`, etc.

#### Step 8.3 — Performance
- Server components for all marketing sections (no client JS unless interactive)
- Client components only for: mobile nav toggle, locale switcher, pilot form, animated hero elements
- Lazy-load below-fold animations
- Optimized images with `next/image`
- Target: Lighthouse 90+ on all metrics

**Deliverable**: SEO-ready, performant marketing site.

---

## Cult-UI Pro Component Selection Matrix

| Section | Cult-UI Component | Why |
|---------|-------------------|-----|
| **Navigation** | Navigation Apple | Refined, minimal, institutional feel |
| **Hero** | Hero Apple | Authoritative animations, editorial hierarchy, premium |
| **Features** | Features Regal | Spring animations, scroll reveals, bento grid — dignified |
| **Stats** | Stats Hex | Chamfered panels, structured data display |
| **Social Proof** | Social Proof Apple | Refined typography, restrained animation |
| **How It Works** | Process Heerich | Institutional, structured progression |
| **Pricing** | Pricing Apple | Clean tier comparison, enterprise-appropriate |
| **FAQ** | FAQ Apple | Accordion with social proof, refined animations |
| **CTA** | CTA Hex | Chamfered panel, dual actions, trust line |
| **Contact** | Contact Apple | Apple-style form, minimalist, trustworthy |
| **Footer** | Footer Apple | Institutional, comprehensive link structure |

---

## Key Files

| File | Operation | Description |
|------|-----------|-------------|
| `web/src/app/[locale]/(marketing)/layout.tsx` | Create | Marketing shell with header + footer |
| `web/src/app/[locale]/(marketing)/page.tsx` | Create | Landing page composing all sections |
| `web/src/app/[locale]/(marketing)/features/page.tsx` | Create | Detailed features page |
| `web/src/app/[locale]/(marketing)/pricing/page.tsx` | Create | 3-tier pricing page |
| `web/src/app/[locale]/(marketing)/about/page.tsx` | Create | Mission, principles, security |
| `web/src/app/[locale]/(marketing)/contact/page.tsx` | Create | Pilot registration form page |
| `web/src/app/[locale]/(app)/layout.tsx` | Create | Authenticated app shell (move from current) |
| `web/src/components/marketing/layout/marketing-header.tsx` | Create | Nav Apple component |
| `web/src/components/marketing/layout/marketing-footer.tsx` | Create | Footer Apple component |
| `web/src/components/marketing/hero/hero-section.tsx` | Create | Hero Apple customized |
| `web/src/components/marketing/sections/*.tsx` | Create | 7 section components |
| `web/src/components/marketing/forms/pilot-registration-form.tsx` | Create | React Hook Form + Zod |
| `web/src/components/marketing/forms/pilot-registration-schema.ts` | Create | Shared Zod schema |
| `web/src/app/api/pilot/route.ts` | Create | Pilot registration API |
| `web/messages/en/marketing.json` | Create | English marketing content |
| `web/messages/fr/marketing.json` | Create | French marketing content |
| `web/src/app/globals.css` | Modify | Add marketing design tokens |
| `web/src/middleware.ts` | Modify | Ensure marketing routes are public |
| `migrations/015_pilot_registrations.up.sql` | Create | Pilot registrations table |
| `migrations/015_pilot_registrations.down.sql` | Create | Drop pilot registrations |
| `internal/pilot/handler.go` | Create | Go handler for pilot registrations |
| `internal/pilot/repository.go` | Create | Go repository for pilot data |

---

## Risks and Mitigation

| Risk | Mitigation |
|------|------------|
| Auth middleware accidentally blocking marketing pages | Test all public routes without auth; explicit allowlist in middleware |
| cult-ui components don't match Archive design tokens | Download and fully customize — don't use as black-box imports |
| French copy expansion breaks layouts | Use `text-balance`, fluid spacing, test all pages in both locales |
| Heavy hero animations hurt performance | Lazy-load, prefer CSS over JS animation, test Lighthouse |
| Pilot form spam/abuse | Honeypot field, rate limiting, zod validation, IP hash dedup |
| Route group migration breaks existing app URLs | Route groups are transparent — URLs don't change |
| SEO: internal routes accidentally indexed | Explicit robots.txt exclusions, no sitemap entries for app routes |

---

## SESSION_ID (for /ccg:execute use)
- CODEX_SESSION: codex-1775655798-27817
- GEMINI_SESSION: unavailable (quota exhausted)
