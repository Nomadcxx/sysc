# SYSC Documentation Site Implementation Plan

> **For the implementing engineer:** Use `superpowers:executing-plans` to implement these tasks in dependency order. This plan does not authorize site publication; deployment requires a later instruction.

**Goal:** Build the approved instructional documentation site for SYSC and its component projects.

**Architecture:** Keep reviewed content and the static site in `Nomadcxx/sysc/docs-site/`. Reuse the reviewed sysc-greet Next.js/Fumadocs shell and export static files with an explicit base path. Publish trusted builds of `main` only after a separate deployment instruction. Track current component source separately from exact SYSC release pins.

**Tech Stack:** Existing Next.js 16.2.10, Fumadocs, React, MD/MDX, TypeScript, Tailwind CSS, Orama static search, Node 22, npm lockfile, and GitHub Pages.

**Source of truth:** Follow `2026-10-09-sysc-documentation-site-design.md` and the evidence boundaries in `2026-10-09-sysc-documentation-site-handover.md`. Treat the handover's local component snapshots as research leads. At implementation time, read current default-branch source for behavior guidance, use the documented SYSC release tag for installer pins, and record both. Update source references with code changes; component releases can continue independently.

---

## Task 1: Refresh the source and route record

**Files:**
- Create: `docs-site/README.md`
- Create: `docs-site/content/docs/reference/sources.mdx`
- Create: `docs-site/content/docs/reference/compatibility.mdx`
- Read: `internal/pin/pin.json` at the SYSC release described by the install guide

1. Record the sysc-greet shell commit, lockfile digest, latest deployed commit, and current source revision separately.
2. Record each component's current default-branch commit and the suite-pinned tag where one exists. Identify branch-only or dirty local work as excluded unless it is intentionally part of the selected source.
3. Compare support, runtime, privilege, and installation claims with those sources. Label unknowns instead of filling gaps by inference.
4. Write the route inventory from the handover into Fumadocs navigation metadata. Include the twelve requested projects, plus integration pages for sysc-notify and gSlapper and a sysc-Go reference.
5. Record copied material and artwork attribution; prefer original factual instructions when copying would complicate licensing.

**Acceptance:** Reviewers can trace each support claim to current source and each install claim to an actual SYSC release pin. This is a maintained source record, not a gate that freezes component development.

## Task 2: Reuse the static site shell

**Files:**
- Create: `docs-site/` from the chosen reviewed sysc-greet docs source and lockfile
- Adapt: `docs-site/next.config.mjs`, `source.config.ts`, `lib/shared.ts`, `lib/layout.shared.tsx`
- Adapt: `docs-site/app/layout.tsx`, `docs-site/app/docs/[[...slug]]/page.tsx`
- Adapt: `docs-site/components/brand.tsx`, `greeter-header.tsx`, `docs-home.tsx`
- Adapt: `docs-site/app/global.css`, `docs-site/public/`, navigation and check scripts

1. Copy from the reviewed source commit. Exclude `node_modules`, `.next`, `.source`, and `out`.
2. Keep the existing Next/Fumadocs versions and copy the matching `package-lock.json` before dependency changes.
3. Replace greet branding and edit links with SYSC. Keep component provenance links separate from central edit links.
4. Use an explicit `DOCS_BASE_PATH`: empty for local development and `/sysc` for the Pages export. Use the same value for metadata, assets, markdown exports, and search paths.
5. Keep the static export, Orama search, article TOCs, code-copy controls, and accessible Fumadocs navigation.

**Acceptance:** Local root-path and `/sysc` exports resolve their routes, assets, source links, and nonempty search payload without fetching remote images.

## Task 3: Build the home page and navigation

**Files:**
- Create: `docs-site/content/docs/index.mdx` and section `meta.json` files
- Adapt: `docs-site/components/docs-home.tsx`, `docs-site/components/brand.tsx`
- Adapt: `docs-site/scripts/check-content.mjs`, `docs-site/scripts/check-export.mjs`

1. Build the five-action home: install, configure, troubleshoot, develop, and author a plugin.
2. Add the compact component directory and visible compatibility link.
3. Use the five sidebar groups: Start, Guides, Components, Developers, Plugins. Nest troubleshooting and reference pages under the closest task group without changing their stable routes.
4. Make search results include project names. Keep full-site search; add no custom filter or indexing system.
5. Remove greet-specific copy, fixed chapter counts, layout-string assertions, and postinstall-script checks. Retain route, search, source, asset, and accessibility invariants.

**Acceptance:** A new reader can enter through an action, while an existing user can find a project page from the sidebar or global search.

## Task 4: Write installation and recovery guidance

**Files:**
- Create content under `docs-site/content/docs/start/`, `guides/`, `troubleshooting/`, `reference/`, and `components/sysc/`
- Add or adapt navigation metadata for those routes

1. Write requirements, supported install paths, wizard controls, privilege boundaries, and verification from the reviewed installer release.
2. Explain weather input/privacy, conflict choices, service setup, partial results, and what the installer records.
3. Document upgrade, uninstall, restore behavior, XDG paths, and existing configuration preservation from the actual stamp/restore logic.
4. Add read-only service and symptom diagnostics first; put recovery effects and rollback steps beside any state-changing commands.
5. Keep support limits beside install links. Do not claim full-suite arm64/Fedora/Debian support unless the tagged installer and component assets establish it. Do not prefix `systemctl --user` commands with sudo.

**Acceptance:** Readers can install, verify, troubleshoot, upgrade, and remove the suite without relying on repository planning documents.

## Task 5: Write component and integration pages

**Files:**
- Create: `docs-site/content/docs/components/<project>/` for the requested components
- Create integration pages for sysc-notify and gSlapper
- Add sysc-Go reference links and shared theme, wallpaper, session, architecture, and interface pages

1. Give each component an overview with its role, support status, suite pin when applicable, source link, and links to shared tasks.
2. Write system-specific guidance from current default-branch behavior, then state separately what the selected suite release installs.
3. Explain ownership boundaries for shell, tray, clipboard, notifications, wallpaper providers, lock, and greeter services.
4. Give libraries small working usage examples and versioned public API links. Do not invent daemon services for libraries.
5. Link each repository's own build/deploy rules. Check examples for usernames, host paths, credentials, and other private data.

**Acceptance:** All requested projects have substantive, source-backed documentation. Integration pages identify which process or configuration owns each behavior.

## Task 6: Write plugin authoring and submission guidance

**Files:**
- Create: `docs-site/content/docs/plugins/` and plugin navigation metadata
- Optional: a small deterministic catalog listing script under `docs-site/scripts/`

1. Use the selected sysc-shell SDK and catalog sources; include a small existing plugin example.
2. Explain manifests, protocol handshake, host capabilities, accessible trees, geometry checks, release tags, archive shape, and catalog validation.
3. Generate installable listings from a reviewed catalog input. Never download or execute plugin assets during documentation builds.
4. Describe official inclusion proposals through PRs to `sysc-plugins`; distinguish technical validation from maintainer acceptance and make no SLA promise.
5. Keep license metadata optional to match the current catalog schema. State the current screenshot validation gap without claiming CI enforces image dimensions.

**Acceptance:** Authors can write, validate, release, and propose a plugin using current interfaces and accurate trust boundaries.

## Task 7: Add read-only documentation CI and preview artifacts

**Files:**
- Create: `.github/workflows/docs.yml`
- Adapt: `docs-site/scripts/check-content.mjs`, `docs-site/scripts/check-export.mjs`, `docs-site/README.md`

1. Run a stable `docs-check` job on all PRs and pushes to `main`; if branch protection requires it, let an unchanged-files gate succeed without skipping the check.
2. Grant the check only read access, set `persist-credentials: false`, a bounded timeout, and cancellation for superseded PR runs.
3. Install Node 22, run `npm ci`, and run the `/sysc` export check. Cache npm downloads by lockfile; do not cache production output.
4. Upload `docs-site/out` as a seven-day downloadable PR artifact. Record source SHA, base path, and build command in the job summary.
5. Validate internal routes, assets, anchors, navigation, provenance, and suite-pin consistency. Keep external link checks advisory.
6. Keep MDX/npm builds away from deployment secrets. Do not use `pull_request_target` to execute PR source or deploy a PR artifact from a privileged workflow.

**Acceptance:** A PR provides build/content evidence and an artifact preview; a fork cannot publish the production site.

## Task 8: Review the exported experience

**Files:**
- Review: `docs-site/out/` and preview artifact
- Add screenshots to the PR or review record; no new test dependency by default

1. Run `cd docs-site && npm ci && npm run check` for the local root path.
2. Run `DOCS_BASE_PATH=/sysc npm run check` for Pages output.
3. Preview the artifact beneath a local `sysc/` directory and open `/sysc/docs/`.
4. Review desktop/mobile navigation, keyboard-only search/menu/copy controls, visible focus, 200% zoom, reduced motion, contrast, and code/table overflow.
5. Capture home, procedure, component, and plugin pages at desktop and mobile widths.

**Acceptance:** Routes, local assets, search, keyboard interaction, and responsive reading work in both base-path builds.

## Task 9: Prepare publishing and ownership; deploy only after a later instruction

**Files/settings:**
- Prepare: `.github/workflows/docs.yml` production job and `docs-site/README.md`
- Later configuration: repository Pages settings, `github-pages` environment, branch protection, and approved component README links

1. Identify the actual site maintainer and component reviewers before transferring ongoing publication responsibility. Do not invent handles.
2. Confirm Pages is configured for Actions, production is restricted to `main`, and the deployment environment's review policy is known.
3. Rebuild reviewed `main`; grant Pages/OIDC permissions only to the deployment job and pin Actions to reviewed full SHAs.
4. Deploy `docs-site/out` only after a separate publishing instruction. Smoke-check the home, one page per project, search payload, and assets under `/sysc`.
5. Cross-link from greet first and preserve its existing URLs. Plan old-origin redirects only after the new routes pass review.
6. Document rollback through a reviewed revert or a retained trusted production artifact; never move release tags or force-push.

**Acceptance:** Publishing is restricted to trusted main, the public site passes its smoke check, old greet routes remain available, and rollback points to a known-good commit or artifact.
