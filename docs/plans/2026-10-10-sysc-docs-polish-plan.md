# SYSC documentation polish implementation plan

> **Implementation note:** Execute the tasks in order and verify each change before continuing.

**Goal:** Use the official SYSC wordmark, establish clear page-title hierarchy, shorten long section headings, and record a source-aware competitor gap analysis.

**Architecture:** Reuse the shared brand component and documentation CSS so the visual changes apply to all article routes. Keep the current page structure, navigation, and slash-framed section style. Store the logo in the static public directory and record its source and license.

**Tech Stack:** Next.js static export, Fumadocs MDX, CSS, Python Playwright with system Chromium.

---

### Task 1: Add visual regression checks

**Files:**
- Create: `docs/reviews/2026-10-10-sysc-docs-gap-analysis/check-ui.py`

Check the logo and label, heading hierarchy, slash weight, overflow, and single-line section headings on every article at 390 px. Also check mobile navigation opens with Enter and closes with Escape.

Run the check against the current static preview first. It must fail on the existing title hierarchy before any CSS changes.

### Task 2: Update the shared brand and heading styles

**Files:**
- Create: `docs-site/public/sysc-logo.png`
- Create: `docs-site/public/SYSC-LOGO-SOURCE.txt`
- Modify: `docs-site/components/brand.tsx`
- Modify: `docs-site/components/docs-home.tsx`
- Modify: `docs-site/app/global.css`
- Modify: `docs-site/app/layout.tsx`

Use the transparent SYSC wordmark from `sysc-greet/assets/logo.png`; show “Documentation” beneath it on every viewport. Set article titles to 32–48 px, level-two headings to 20–24 px, and level-three headings to 18–20 px. Keep the slash frames and uppercase section headings, but reduce slash size, weight, and spacing. `docs-site/LICENSE` already covers GPL-3.0 assets.

### Task 3: Shorten long section labels

**Files:**
- Modify: section headings across component, developer, guide, plugin, reference, start, and troubleshooting pages
- Update: any same-page link whose heading slug changes

Use the approved concise labels in `docs/plans/2026-10-10-sysc-docs-heading-design.md`, then scan every article heading at 390 px and shorten any remaining labels that wrap. Preserve each section's meaning, update any affected same-page links, and keep MDX navigation generation intact.

### Task 4: Record the competitor gap analysis and visual review

**Files:**
- Create: `docs/reviews/2026-10-10-sysc-docs-gap-analysis/README.md`
- Add: refreshed captures for desktop, tablet, and mobile review

Compare the SYSC page groups and first-use flow with the public Noctalia and DankMaterialShell documentation structures. Separate source-backed coverage gaps from features that belong to those larger products. Record the audit scope and browser limits.

### Task 5: Verify and publish

Run `npm run check` and `DOCS_BASE_PATH=/sysc npm run check` from `docs-site/`. Serve the `/sysc` export on port 8766, then run `python3 docs/reviews/2026-10-10-sysc-docs-gap-analysis/check-ui.py`. Inspect representative pages at 390, 720, and 1440 px, confirm no horizontal overflow and no wrapped section headings at 390 px, and save desktop, tablet, and mobile captures. Open a PR, wait for `docs-check` and CI, merge, then confirm the Pages deployment and live routes.
