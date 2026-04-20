# Implementation Plan: All Public-Facing Pages

## Task Type
- [x] Frontend (marketing pages)
- [ ] Backend
- [ ] Fullstack

## Context

### Source
24 static HTML design files at `web/public/design/*.html` define every public-facing page. Each uses a shared `assets/style.css` with CSS variables matching the existing Tailwind design tokens in `globals.css`.

### Existing Pages (6)
| Route | Design equiv. | Status | Issue |
|-------|--------------|--------|-------|
| `/(marketing)/page.tsx` | `index.html` | Done | — |
| `/(marketing)/features/page.tsx` | `platform.html` | Done | **RENAME to `/platform`** — header links to `/platform` |
| `/(marketing)/about/page.tsx` | `manifesto.html` | Done | **RENAME to `/manifesto`** — header links to `/manifesto` |
| `/(marketing)/pricing/page.tsx` | `pricing.html` | Done | — |
| `/(marketing)/contact/page.tsx` | `contact.html` | Done | — |
| `/login/page.tsx` | `sign-in.html` | Done | — |

### CRITICAL: Route Renames Required
The marketing header nav links to `/platform` and `/manifesto`, but existing pages live at `/features` and `/about`. Must either:
- **Option A (recommended):** Rename `/features` → `/platform` and `/about` → `/manifesto` (matching design)
- **Option B:** Add redirects from `/platform` → `/features` and `/manifesto` → `/about`

### Pages to Build (18)
| # | Route | Design file | Category | Complexity |
|---|-------|-------------|----------|------------|
| 1 | `/(marketing)/security/page.tsx` | `security.html` | Top-nav | High — layers, threat model, arch diagram, Merkle SVG, standards grid, audit cards |
| 2 | `/(marketing)/evidence/page.tsx` | `evidence.html` | Feature detail 01/05 | Medium — sp-hero, ingest transcript, formats grid |
| 3 | `/(marketing)/custody/page.tsx` | `custody.html` | Feature detail 02/05 | Medium — dark chain visual, RLS section, lemma proof |
| 4 | `/(marketing)/witness/page.tsx` | `witness.html` | Feature detail 03/05 | Medium — ID card, break-the-glass flow, masking rows |
| 5 | `/(marketing)/collaboration/page.tsx` | `collaboration.html` | Feature detail 04/05 | High — CRDT diagram, split-view, federation diagram |
| 6 | `/(marketing)/search-discovery/page.tsx` | `search.html` | Feature detail 05/05 | Medium — search demo, pipeline rows |
| 7 | `/(marketing)/ngos/page.tsx` | `ngos.html` | Vertical | Medium — checklist, plan card, production rows |
| 8 | `/(marketing)/midtier/page.tsx` | `midtier.html` | Vertical | Medium — case stats, rows, cols |
| 9 | `/(marketing)/icc/page.tsx` | `icc.html` | Vertical | Medium — big stats, dark architecture, rows |
| 10 | `/(marketing)/commissions/page.tsx` | `commissions.html` | Vertical | Medium — timeline, rows |
| 11 | `/(marketing)/pilot/page.tsx` | `pilot.html` | CTA | Medium — step list, pilot form |
| 12 | `/(marketing)/source/page.tsx` | `source.html` | Open source | Medium — repo meta, module grid, commits |
| 13 | `/(marketing)/docs/page.tsx` | `docs.html` | Open source | High — repo card, doc grid, validator, changelog |
| 14 | `/(marketing)/federation/page.tsx` | `federation.html` | Open source | Medium — RFC header, TOC, implementations, governance |
| 15 | `/(marketing)/validator/page.tsx` | `validator.html` | Open source | Medium — download grid, terminal output |
| 16 | `/(marketing)/disclosure/page.tsx` | `disclosure.html` | Legal | Medium — report channels, bounty grid, hall of thanks |
| 17 | `/(marketing)/privacy/page.tsx` | `privacy.html` | Legal | Low — mostly prose |
| 18 | `/(marketing)/legal/page.tsx` | `legal.html` | Legal | Low — mostly prose, imprint grid, canary |

## Technical Solution

### Pattern (proven by existing pages)
Each marketing page follows the same pattern established in `features/page.tsx`:
1. **Server component** with `export const metadata` for SEO
2. **Inline `pageStyles`** string for page-specific CSS (injected via `<style dangerouslySetInnerHTML>`)
3. **JSX body** translating the HTML structure verbatim, using the design system's CSS variables
4. Shared layout provides `<MarketingHeader>` and `<MarketingFooter>` automatically
5. Pages that need interactivity (pilot form, search) use a separate `content.tsx` client component

### Shared Components to Extract
Several design patterns repeat across pages and should be extracted into reusable components:

1. **`SpHero`** — The "sub-page hero" used by all feature-detail + vertical + open-source pages (`sp-hero`, `sp-hero-inner`, `sp-eyebrow`, `sp-hero-meta`)
2. **`SpSection`** — Section wrapper with optional `dark` variant (`sp-section`, `sp-section dark`)
3. **`SpGrid`** — The rail+body 12-col grid layout (`sp-grid-12`, `sp-rail`, `sp-body`)
4. **`SpQuote`** — Blockquote section (`sp-quote`)
5. **`SpCols3`** — Three-column feature columns (`sp-cols3`)
6. **`SpRows`** — Stacked detail rows (`sp-rows`, `sp-row`)
7. **`SpNextPrev`** — Prev/next navigation at bottom (`sp-nextprev`)
8. **`CtaBanner`** — CTA banner with blob decoration (`cta-banner`)
9. **`CodeCard`** — Terminal-style code block (`code-card`)

These CSS classes come from the shared `assets/style.css`. The corresponding styles need to be added to `design-marketing.css`.

### CSS Strategy
- Port the full sub-page scaffold from `assets/style.css` lines 508–639 into `web/src/app/design-marketing.css` (~150 lines)
- Classes to port: `sp-hero`, `sp-hero-inner`, `sp-eyebrow`, `sp-hero-meta`, `sp-section`, `sp-section.dark`, `sp-grid-12`, `sp-rail`, `sp-body`, `sp-lead`, `sp-cols3`, `sp-cols2`, `sp-rows`, `sp-row`, `sp-stats`, `sp-quote`, `sp-nextprev`, `code-card`, `cta-banner` + all child selectors and `@media` queries
- Page-specific styles stay inline via `pageStyles` (same pattern as existing pages)
- All CSS variables already exist in `globals.css` / `design-marketing.css`

### Link Updates
- **Rename routes:** `/features` → `/platform`, `/about` → `/manifesto` (to match header nav + design file names)
- Update `marketing-header.tsx` nav links to match new routes
- Update `marketing-footer.tsx` — wire all 18 new page links; use `/search-discovery` (NOT `/search`) for the search page to avoid collision with authenticated route
- Internal links between pages (next/prev, CTAs) use Next.js `<Link>`

## Implementation Steps

### Phase 0: Route Renames
0. **Rename existing routes to match designs**
   - Move `/(marketing)/features/` → `/(marketing)/platform/` (rename directory + update imports)
   - Move `/(marketing)/about/` → `/(marketing)/manifesto/` (rename directory + update imports)
   - Update any internal links referencing `/features` or `/about` throughout the codebase

### Phase 1: Foundation (shared CSS + navigation)
1. **Port full sub-page CSS scaffold to `design-marketing.css`** — Extract lines 508–639 from `assets/style.css`: all `sp-*` classes, `code-card`, `cta-banner`, responsive `@media` queries
   - Expected: ~150 lines of CSS added
   - Includes: `sp-hero`, `sp-hero-inner`, `sp-eyebrow`, `sp-hero-meta`, `sp-section`, `sp-section.dark`, `sp-grid-12`, `sp-rail`, `sp-body`, `sp-lead`, `sp-cols3`, `sp-cols2`, `sp-rows`, `sp-row`, `sp-stats`, `sp-quote`, `sp-nextprev`, `code-card`

2. **Update navigation links** — Fix `marketing-header.tsx` and `marketing-footer.tsx`
   - Header: verify Platform, Security, Pricing, Manifesto, Open source links
   - Footer: wire all 18 new page links (use `/search-discovery` not `/search`)

### Phase 2: Feature Detail Pages (01–05)
These share the most structure. Build them in order since they link prev/next:

3. **`/evidence`** — Evidence management (01/05)
4. **`/custody`** — Chain of custody (02/05)
5. **`/witness`** — Witness protection (03/05)
6. **`/collaboration`** — Live collaboration (04/05)
7. **`/search-discovery`** — Search & discovery (05/05)

### Phase 3: Top-Level Pages
8. **`/security`** — Security model (complex: Merkle SVG, arch diagram, standards)

### Phase 4: Vertical Pages (For Institutions)
9. **`/ngos`** — NGOs in The Hague
10. **`/midtier`** — Mid-tier tribunals
11. **`/icc`** — ICC-scale bodies
12. **`/commissions`** — Truth commissions
13. **`/pilot`** — Start a pilot (includes form — client component)

### Phase 5: Open Source Pages
14. **`/source`** — Source code
15. **`/docs`** — Open source & docs (self-hosting, validator, changelog)
16. **`/federation`** — VKE1 federation spec
17. **`/validator`** — Clerk validator

### Phase 6: Legal/Company Pages
18. **`/disclosure`** — Responsible disclosure (bounty table)
19. **`/privacy`** — Privacy policy
20. **`/legal`** — Legal & imprint (warrant canary)

## Key Files

| File | Operation | Description |
|------|-----------|-------------|
| `web/src/app/[locale]/(marketing)/features/` | Rename | Move to `/(marketing)/platform/` |
| `web/src/app/[locale]/(marketing)/about/` | Rename | Move to `/(marketing)/manifesto/` |
| `web/src/app/design-marketing.css` | Modify | Port full sp-* scaffold (~150 lines) |
| `web/src/components/marketing/layout/marketing-header.tsx` | Modify | Add Security nav link, verify others |
| `web/src/components/marketing/layout/marketing-footer.tsx` | Modify | Wire all 18 new page links |
| `web/src/app/[locale]/(marketing)/security/page.tsx` | Create | Security model page |
| `web/src/app/[locale]/(marketing)/evidence/page.tsx` | Create | Evidence management 01/05 |
| `web/src/app/[locale]/(marketing)/custody/page.tsx` | Create | Chain of custody 02/05 |
| `web/src/app/[locale]/(marketing)/witness/page.tsx` | Create | Witness protection 03/05 |
| `web/src/app/[locale]/(marketing)/collaboration/page.tsx` | Create | Live collaboration 04/05 |
| `web/src/app/[locale]/(marketing)/search-discovery/page.tsx` | Create | Search & discovery 05/05 |
| `web/src/app/[locale]/(marketing)/ngos/page.tsx` | Create | NGOs vertical |
| `web/src/app/[locale]/(marketing)/midtier/page.tsx` | Create | Mid-tier tribunals vertical |
| `web/src/app/[locale]/(marketing)/icc/page.tsx` | Create | ICC-scale bodies vertical |
| `web/src/app/[locale]/(marketing)/commissions/page.tsx` | Create | Truth commissions vertical |
| `web/src/app/[locale]/(marketing)/pilot/page.tsx` | Create | Pilot signup page |
| `web/src/app/[locale]/(marketing)/pilot/content.tsx` | Create | Pilot form client component |
| `web/src/app/[locale]/(marketing)/source/page.tsx` | Create | Source code page |
| `web/src/app/[locale]/(marketing)/docs/page.tsx` | Create | Open source & docs |
| `web/src/app/[locale]/(marketing)/federation/page.tsx` | Create | VKE1 federation spec |
| `web/src/app/[locale]/(marketing)/validator/page.tsx` | Create | Clerk validator |
| `web/src/app/[locale]/(marketing)/disclosure/page.tsx` | Create | Responsible disclosure |
| `web/src/app/[locale]/(marketing)/privacy/page.tsx` | Create | Privacy policy |
| `web/src/app/[locale]/(marketing)/legal/page.tsx` | Create | Legal & imprint |

## Risks and Mitigation

| Risk | Mitigation |
|------|------------|
| Large surface area (18 pages) | Group by structural similarity; batch phases |
| CSS conflicts with existing marketing styles | Namespace page-specific styles inline; shared styles use established sp-* prefix |
| Internal links between pages must all resolve | Build pages in linked order (evidence→custody→witness→collab→search) |
| Pilot form needs backend | Client component with `onSubmit` stub; can wire to API later |
| SVG (Merkle tree on security page) | Inline JSX SVG, same as design |
| Responsive breakpoints | Design HTML already includes `@media` queries — port them directly |

## Verification Results (4 parallel agents, 2026-04-20)
- **Design file coverage:** 24/24 (100%) — all design files accounted for
- **Cross-page link integrity:** 0 broken links across 51 design files
- **CSS gap:** ~150 lines of sp-* scaffold classes need porting (accounted for in Phase 1)
- **Route naming:** Mismatch found and resolved — Phase 0 renames `/features`→`/platform`, `/about`→`/manifesto`
- **Footer link collision:** `/search` → `/search-discovery` to avoid authenticated route conflict

## Notes
- Phase 0 renames existing routes to match design file names and header nav links
- The existing `/login` page IS the design's `sign-in.html` — no changes needed
- Route `/search-discovery` instead of `/search` to avoid collision with the authenticated `/search` route
- All pages are server components except pilot (form interactivity)
- Total scope: 2 renames + 18 new pages + CSS scaffold + nav updates = ~20 files modified/created
