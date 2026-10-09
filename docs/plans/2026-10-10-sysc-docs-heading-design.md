# SYSC documentation identity and heading design

**Status:** Approved on 2026-10-10.

## Decision

Use the transparent SYSC wordmark from `Nomadcxx/sysc-greet/assets/logo.png` in the site header. Place the visible label `Documentation` directly below it on desktop and mobile. Remove the current text-only slash mark from the brand and omit `Desktop` from the lockup.

Make article titles visibly larger than section headings. Target 32–48 px for article titles, 20–24 px for level-two headings, and 18–20 px for level-three headings. Keep the existing uppercase section-heading style and slash frames, but reduce the frame size, weight, and spacing. Shorten the eight longest section labels without changing their meaning.

## Heading copy

- `Keep history readable across changes` → `Protect clipboard history`
- `Read GPU results with their limits` → `GPU results and limits`
- `Required identity and runtime fields` → `Manifest fields`
- `Test behavior and protocol handling` → `Test plugin behavior`
- `Give every control an accessible identity` → `Accessible controls`
- `SYSC module versions in the installed shell` → `Installed module versions`
- `Plugin SDK selected by sysc-plugins` → `Plugin SDK version`
- `If uninstall reports a restore failure` → `Recover uninstall files`

## Gap analysis

Compare SYSC's current information architecture and source-backed coverage with the Noctalia and DankMaterialShell documentation sites. Record competitor patterns separately from gaps that apply to SYSC's own source scope. Do not treat competitor-only features as missing SYSC documentation.

## Acceptance

- The logo and `Documentation` label appear in the header at desktop and mobile sizes; no `Desktop` label appears in the brand.
- Article titles remain clearly larger than section headings on representative component, guide, start, and plugin pages.
- The shortened plugin UI heading fits on one line at 390 px.
- Slash frames remain visible with less visual weight.
- The root build and `/sysc` static export pass. Browser checks cover 390, 720, and 1440 px.
- The review report records the competitor comparison, coverage gaps, and limits of the audit.
