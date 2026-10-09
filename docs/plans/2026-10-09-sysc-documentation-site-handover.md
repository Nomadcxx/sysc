# SYSC Documentation Site Implementation Handover

> **For the implementing engineer:** Use `superpowers:executing-plans` to implement the sub-plans in dependency order. This commission covers research and planning; building and publishing the site need a subsequent implementation instruction.

**Goal:** Provide one instructional documentation site for the SYSC installer and its component projects, with accurate installation, configuration, recovery, development and plugin submission guidance.

**Architecture:** Keep the site and reviewed documentation in `Nomadcxx/sysc`, under `docs-site/`. Reuse the current sysc-greet Next.js/Fumadocs static export and GitHub Pages workflow. Component repositories retain implementation and release authority; the site records the component versions and source commits its instructions describe.

**Tech stack:** Existing Next.js 16.2.10, Fumadocs, React, MD/MDX, TypeScript, Tailwind CSS, static Orama search, Node 22, npm lockfile and GitHub Pages. Copy the reviewed upstream lockfile before changing dependencies. No application server, database or new hosting provider.

---

## 1. Commission and evidence boundaries

The owner requested coverage of sysc, sysc-greet, sysc-lock, sysc-shell, sysc-tray, sysc-terminal, sysc-clipboard, sysc-wayland, sysc-metrics, sysc-launch, sysc-walls and sysc-plugins, including submission requirements. This document contains the design audit, content inventory and implementation handover. It does not change those applications, deploy documentation, create releases or introduce CI jobs.

Research date: 2026-10-09. Read-only inspection covered local source and Git objects, plus an initial remote GitHub/Pages comparison. The parent inspected the existing site at desktop width and one 375px quick-start page, and confirmed that keyboard search for “key” returned documentation results. The mobile menu toggle, a full accessibility review and the future SYSC site remain unverified. We did not install dependencies, build the site, run component tests or exercise sudo. Subsequent implementation review can use a local export and local browser; live Cloak inspection is unnecessary.

**Source of truth matters:** `/home/nomadx/sysc-greet` is checked out at `ccfcdde`, branch `feat/issue-53-void-bsd-investigation`. Its `mkdocs.yml`, `docs-src/` and Python workflow describe an older site. The locally available `origin/master` object is `fcf15a8fbc882023a134a1e7f5c96a0786f85712`; use `git show fcf15a8fbc882023a134a1e7f5c96a0786f85712:<path>` for the frozen local audit below. That object contains `docs-site/` and removes the MkDocs files. Do not copy the stale checked-out greet tree as the site template.

An earlier remote comparison observed master at `4c98abc5bdddbf6c84d2a55d71b9120cbfccad0f`. That SHA is not available in this local object database. Most inspected site files match the local baseline, but its workflow and content checker differ: the remote comparison includes a PR check and a remote-image guard absent from the local baseline. Treat these as distinct snapshots; the audit does not establish ancestry between them. Before implementation, choose a reviewed local source commit and compare its actual workflow/checker, rather than inferring deployment behavior from a mutable ref.

The most recent successful documentation push deployment observed during inspection used `d927105097d79259a24493d20944dd3ae5fdd4b3` on 2026-10-08: [workflow run](https://github.com/Nomadcxx/sysc-greet/actions/runs/37809785059). A default-branch commit does not establish a later documentation deployment. Obtain the deployed commit and current source commit separately when implementation starts.

Other local checkouts also contain branches and unrelated work. This audit records their content as source leads, not proof that it shipped. In particular, shell and plugins contain dirty files; no changes from those files should enter a published guide without comparing with the intended release. Shell `AGENTS.md` also keeps its plans local and tracks implementation in beads. This handover lives in sysc and must not overwrite or duplicate shell's issue status.

### Local component snapshots used as source leads

| Checkout | Audited HEAD | Publication caveat |
|---|---|---|
| sysc | `0541bbb` | Instructional README on main; read the installer pin at its release tag. |
| sysc-greet | `ccfcdde` checked out; `fcf15a8fbc882023a134a1e7f5c96a0786f85712` local docs object | Use the Git object, not the legacy checked-out docs. |
| sysc-lock | `408570a` | Local docs/bin untracked; review published release before importing behavior. |
| sysc-shell | `8140efc2` | Feature branch with unrelated dirty tracker/register files; source leads need release comparison. |
| sysc-tray | `a7928eb` | Review CLI statements against sysc itself. |
| sysc-terminal | `6f65364` | Dirty tracker file; suite component currently disabled. |
| sysc-clipboard | `8b32855` | Documentation branch; establish published provenance. |
| sysc-wayland | `1677185` | Documentation branch; establish published provenance. |
| sysc-metrics | `0ceba66` | Library source lead. |
| sysc-launch | `da6c1a6` | Library/diagnostic CLI source lead. |
| sysc-plugins | `b916193` | Feature branch with unrelated aiusage edits; exclude those edits. |
| sysc-walls | No local checkout found | Initial read-only remote README/tree comparison; freeze a tag/commit before importing. |

### Evidence references

Greet source citations below identify the frozen local Git object and use GitHub commit links for readable file/line references; the contents are available locally through `git show`, without browsing. Where we discuss the initial remote comparison, we identify it separately. Component local references name the audited file and line; implementation must compare them with the intended release and replace publication references with immutable links.

| Ref | Evidence |
|---|---|
| G1 | [Local docs workflow, lines 3–56](https://github.com/Nomadcxx/sysc-greet/blob/fcf15a8fbc882023a134a1e7f5c96a0786f85712/.github/workflows/docs.yml#L3) |
| G2 | [Site dependencies and check commands, lines 5–35](https://github.com/Nomadcxx/sysc-greet/blob/fcf15a8fbc882023a134a1e7f5c96a0786f85712/docs-site/package.json#L5) |
| G3 | [Static export and base path, lines 5–16](https://github.com/Nomadcxx/sysc-greet/blob/fcf15a8fbc882023a134a1e7f5c96a0786f85712/docs-site/next.config.mjs#L5) |
| G4 | [Local content check, lines 7–47](https://github.com/Nomadcxx/sysc-greet/blob/fcf15a8fbc882023a134a1e7f5c96a0786f85712/docs-site/scripts/check-content.mjs#L9) |
| G5 | [Export checks, lines 38–307](https://github.com/Nomadcxx/sysc-greet/blob/fcf15a8fbc882023a134a1e7f5c96a0786f85712/docs-site/scripts/check-export.mjs#L38) |
| G6 | [Theme and typography, lines 9–54](https://github.com/Nomadcxx/sysc-greet/blob/fcf15a8fbc882023a134a1e7f5c96a0786f85712/docs-site/app/global.css#L9), [motion/mobile rules, line 526](https://github.com/Nomadcxx/sysc-greet/blob/fcf15a8fbc882023a134a1e7f5c96a0786f85712/docs-site/app/global.css#L526) |
| G7 | [Hardcoded source branch, lines 7–10](https://github.com/Nomadcxx/sysc-greet/blob/fcf15a8fbc882023a134a1e7f5c96a0786f85712/docs-site/lib/shared.ts#L7), [article source link, line 40](https://github.com/Nomadcxx/sysc-greet/blob/fcf15a8fbc882023a134a1e7f5c96a0786f85712/docs-site/app/docs/%5B%5B...slug%5D%5D/page.tsx#L40) |
| G8 | [Static search endpoint, lines 4–8](https://github.com/Nomadcxx/sysc-greet/blob/fcf15a8fbc882023a134a1e7f5c96a0786f85712/docs-site/app/api/search/route.ts#L4), [search client, lines 18–35](https://github.com/Nomadcxx/sysc-greet/blob/fcf15a8fbc882023a134a1e7f5c96a0786f85712/docs-site/components/search.tsx#L18) |
| G9 | [Content schema, lines 6–16](https://github.com/Nomadcxx/sysc-greet/blob/fcf15a8fbc882023a134a1e7f5c96a0786f85712/docs-site/source.config.ts#L6), [layout/sidebar, lines 8–13](https://github.com/Nomadcxx/sysc-greet/blob/fcf15a8fbc882023a134a1e7f5c96a0786f85712/docs-site/app/docs/layout.tsx#L8) |
| S1 | [Installer README](/home/nomadx/sysc/README.md:11), source one-liner at line 49, suite/architecture limits at line 126, distro routes at line 148 |
| S2 | [Suite manifest](/home/nomadx/sysc/internal/pin/pin.json:2), [preflight](/home/nomadx/sysc/internal/preflight/preflight.go:22), [Arch-family gate](/home/nomadx/sysc/internal/distro/distro.go:37), [native package routes](/home/nomadx/sysc/cmd/sysc/packages.go:25) |
| S3 | [Installer CI](/home/nomadx/sysc/.github/workflows/ci.yml:16), [binary release workflow](/home/nomadx/sysc/.github/workflows/release.yml:11), [unit templates](/home/nomadx/sysc/internal/units/templates/sysc-lock-session.service:1) |
| P1 | [Plugin authoring](/home/nomadx/sysc-plugins/docs/writing-plugins.md:12), [view geometry rules](/home/nomadx/sysc-plugins/docs/plugin-ui-rules.md:30) |
| P2 | [Plugin publishing](/home/nomadx/sysc-plugins/docs/publishing.md:7), catalog metadata at line 46, screenshots at line 88, third-party sources at line 98 |
| P3 | [Manifest validator](/home/nomadx/sysc-plugins/tools/validate-manifests/main.go:17), [catalog validation](/home/nomadx/sysc-plugins/tools/catalog/validate.go:95), [catalog bounds/schema](/home/nomadx/sysc-shell/plugin/catalog/catalog.go:23) |
| P4 | [Plugin release workflow](/home/nomadx/sysc-plugins/.github/workflows/release.yml:17), catalog PR at line 105, [plugin CI](/home/nomadx/sysc-plugins/.github/workflows/ci.yml:23) |
| P5 | [Process launch/environment](/home/nomadx/sysc-shell/internal/plugin/supervisor.go:152), [host capability checks](/home/nomadx/sysc-shell/internal/plugin/hostcall.go:150), [built-in catalog source](/home/nomadx/sysc-shell/internal/config/config.go:91) |

## 2. Current sysc-greet documentation audit

### Ownership, build and hosting: observations

Maintainers author MD/MDX in `docs-site/content/docs/`, order navigation with `meta.json`, and use the Fumadocs content schema [G9]. The same repository owns its layout, styles, images, dependencies and deployment. Its documentation source contains getting-started, feature, configuration, compositor and development chapters. Current remote content includes Mango, Cagebreak and a development testing page that the stale local MkDocs tree cannot represent.

The live site uses GitHub Pages at <https://nomadcxx.github.io/sysc-greet/docs/>. The Pages API reports workflow builds, HTTPS enforcement and no custom domain. The root URL returns HTTP 200 with a meta refresh and fallback link to `/sysc-greet/docs/`. Embedded Next.js 404 machinery in its HTML does not establish a visible page failure.

The local baseline docs workflow deploys pushes to `master` that touch `docs-site/**` or the workflow, plus manual dispatch; it has no pull-request trigger [G1]. The initial remote comparison adds a separate PR check and skips deploy on PR events. That improvement is a source lead to compare during implementation, not a property of the frozen local baseline. It installs Node 22, restores npm's download cache using `package-lock.json`, runs `npm ci`, then `npm run check`. The check runs content assertions, Fumadocs generation, Next type generation, TypeScript, a webpack static build and export assertions [G2]. Pages receives `docs-site/out`, rather than a running Next.js service. Search uses an exported payload and client-side Orama [G8]; no hosted search API is necessary.

### Design: retain and change

The inspected desktop page has an ASCII masthead, IBM Plex Sans reading text, Fira Code commands, RAMA red/pink accents, sidebar, search and article navigation. CSS provides focus treatments, a reduced-motion fallback and a compact mobile masthead [G6]. The parent confirmed a normal desktop render with a long home article and animated quotation ticker. At 375px, the quick-start content/code stayed inside the document width, and keyboard search returned results. Menu toggling, contrast, zoom and a full keyboard journey remain unverified. The quick-start description appears in both `DocsDescription` and the first content paragraph; remove the redundant paragraph when importing it.

Retain the typography, document reading measure, code presentation, chapter navigation and static search. Use `#000000` for the umbrella site's canvas as the recommended continuation of the owner's installer direction; the current greet canvas is `#161722`. Keep visible borders around code blocks, callouts, search and actionable controls. Use accessible red text such as the existing `#ff6678`, and verify contrast against the chosen surfaces. Do not infer contrast from the palette alone.

Replace the long combined home article with an instructional front door: install the desktop, configure a component, fix a problem, build with the libraries, and submit a plugin. Each entry should name the result and next step. Keep requirements and support limits beside installation links. Put detailed command sequences inside their chapters. Use static artwork by default; if the owner keeps a ticker, provide a visible pause control, retain reduced-motion behavior and keep it outside installation instructions.

For twelve projects, retain a short top-level sidebar: Start, Guides, Components, Developers and Plugins. Each component gets its own section. Search results should include the project name so “install” or “theme” does not produce indistinguishable titles. Begin with search across the whole site; add project filters only if the installed Fumadocs search API covers them without custom indexing infrastructure.

### Engineering gaps and limits

| Observation | Change for SYSC |
|---|---|
| Site source links name `development` although production deploys `master`; an export assertion enforces that branch [G7/G5]. | Point “Edit this page” to central sysc `main`; use a separate immutable component source link for provenance. Remove the inherited branch assertion. |
| Base path derives from `GITHUB_ACTIONS` and hardcodes `/sysc-greet` [G3]. | Use one explicit build setting for `/sysc`; use the same value in images, metadata, markdown exports and search. Local default stays empty. Preview checks must exercise `/sysc`, too. |
| The local workflow has no PR docs check and grants Pages/OIDC globally [G1]. The remote comparison adds a PR job under those same global permissions. | Add a read-only PR check; grant `pages: write` and `id-token: write` only to the trusted deployment job. |
| Manual dispatch can run deploy on a selected branch [G1]. | Require `github.ref == 'refs/heads/main'` for ordinary production deployments; handle rollback through a reviewed revert on main. |
| The local baseline serializes Pages using group `pages`; the remote comparison uses a per-ref group. Neither inspected snapshot offers a hosted preview. | Retain a single production Pages group. PRs produce a downloadable preview artifact without production privileges. |
| Existing checks assert exact brand copy, ticker jokes, CSS layout strings, README wording and a greet postinstall script [G5]. | Keep route, search, assets, source links and accessibility invariants. Adapt brand assertions; remove greet-specific prose and application-script checks from the umbrella site's export checker. |
| Local content checks reject legacy links/admonitions but do not ban remote images [G4]. The remote comparison adds that guard after a recorded build-time image-fetch failure. | Adopt the remote-image guard and keep local licensed artwork. Remote badges in TSX still contact third parties at runtime; omit stars badges unless the owner wants them. |
| The expected-page list covers 18 entries but omits the Mango page present in the source tree [G4]. | Validate declared navigation and the route inventory rather than treating the inherited count as completeness. |
| Source has pinned Next/Fumadocs versions and a committed lockfile; some package ranges allow updates [G2]. Actions use major tags [G1]. | Use `npm ci`, retain the lockfile and review dependency PRs. Pin Actions to reviewed commit SHAs with version comments. A security audit or reproduction of the upstream build has not occurred in this commission. |
| Site has no demonstrated release selector, archive policy or component provenance model. | Add a readable compatibility/provenance table first. Avoid a multi-version UI until there is a second supported suite release to document. |

Repository protection, Pages environment reviewers, private security reporting settings and future custom-domain ownership remain unverified. Check these settings during deployment preparation; do not claim them from workflow text.

## 3. Scope and information architecture

Recommended public base: `https://nomadcxx.github.io/sysc/`. Keep articles under `/docs/`, matching the greet routing pattern. Paths below are relative to that base. Use the same paths in the route inventory and content navigation.

### Shared routes and user journeys

| Journey | Routes | Reader outcome |
|---|---|---|
| First installation | `/docs/`, `/docs/start/requirements/`, `/docs/start/install/`, `/docs/start/wizard/`, `/docs/start/verify/` | Choose a supported route, understand sudo prompts and conflict handover, complete the wizard, confirm the session services. |
| Existing desktop | `/docs/guides/conflicts/`, `/docs/guides/upgrade/`, `/docs/guides/uninstall/` | Preserve existing configuration, understand skip vs download, update with a known suite pin, restore prior providers. |
| Customization | `/docs/guides/themes/`, `/docs/guides/wallpapers/`, `/docs/guides/niri-session/` | Identify which process owns a theme, wallpaper or startup line and edit the correct configuration. |
| Recovery | `/docs/troubleshooting/`, `/docs/troubleshooting/services/`, `/docs/troubleshooting/greeter/`, `/docs/troubleshooting/lock/` | Collect relevant diagnostics, follow symptom-specific recovery and avoid commands that acquire a lock or restart greetd unexpectedly. |
| Developer | `/docs/developers/architecture/`, `/docs/developers/build/`, `/docs/developers/interfaces/`, `/docs/developers/releases/`, `/docs/developers/documentation/` | Find library APIs, protocol ownership, dependency pins, contribution and documentation procedures. |
| Plugin author | `/docs/plugins/`, `/docs/plugins/write/`, `/docs/plugins/manifest/`, `/docs/plugins/protocol/`, `/docs/plugins/ui/`, `/docs/plugins/test/`, `/docs/plugins/publish/`, `/docs/plugins/submit/` | Build a valid plugin, prove the host can render it, publish checked assets and supply a reviewable submission. |
| Version/support | `/docs/reference/compatibility/`, `/docs/reference/suite/`, `/docs/reference/paths/`, `/docs/reference/privileges/` | Distinguish installer support, standalone component support, build architecture, runtime requirements and component release versions. |

Write each procedural page with prerequisites, numbered steps, expected output, and a linked recovery/uninstall path. Label commands as `normal user`, `user service`, or `system administrator` where that affects execution. Keep examples copyable and explain placeholders before the command. Troubleshooting begins with read-only diagnostics; operations that replace configuration, restart a login service, clear history or acquire a lock need their effect stated alongside the command.

### Component route and content inventory

Every row needs an overview and links to the relevant shared guides. Add the listed chapters when there is material content; do not create empty “configuration” pages for libraries.

| Project / route root | Source leads | Chapters and distinctions |
|---|---|---|
| sysc: `/docs/components/sysc/` | [README](/home/nomadx/sysc/README.md:11), `.install`, `cmd/sysc/main.go`, `internal/pin/`, `internal/install/`, `internal/units/` | Installer route selection, wizard keys/screens, flags, weather/privacy, conflict choices, footprint/stamp, idempotent rerun, package escalation, partial results and uninstall. |
| sysc-greet: `/docs/components/sysc-greet/` | Remote `docs-site/content/docs/`, `install.sh`, `cmd/installer/`, `nfpm*.yaml`, `flake.nix` at a reviewed ref | Standalone installation by supported distro, greetd/greeter user, compositor setup, settings/themes/ASCII/wallpaper, keyboard layout, test mode, recovery, service rollback. Import current chapters for Niri, Hyprland, Sway, Mango and Cagebreak; verify their support labels. |
| sysc-lock: `/docs/components/sysc-lock/` | [README](/home/nomadx/sysc-lock/README.md:12), `cmd/installer/`, `scripts/install`, `internal/lockd/PROTOCOLS.md` | Security model, standalone vs suite installation, PAM/build/runtime dependencies, session registration, presentation/theme following, service readiness and recovery. No unlocked preview/test-mode advice copied from greet. |
| sysc-shell: `/docs/components/sysc-shell/` | [README](/home/nomadx/sysc-shell/README.md:32), `docs/niri-hotkeys.md`, `docs/niri-blur.md`, `docs/metrics-widgets.md`, `docs/development.md`, `packaging/systemd/README.md`, public IPC/plugin packages | Niri session setup, panels/launcher/settings, keyboard shortcuts, services, themes, wallpaper backend ownership, companion-daemon integration, plugins, diagnostics. Preserve the repository's `scripts/deploy` policy for developer deployment. |
| sysc-tray: `/docs/components/sysc-tray/` | [README](/home/nomadx/sysc-tray/README.md:14), `contrib/sysc-tray.service`, client/protocol source | User service, StatusNotifierWatcher handover, presenter socket permissions/reconnect, host vs daemon responsibilities, limits such as XEmbed. Correct the README's `sysc install` claim against the actual sysc CLI before reuse. |
| sysc-terminal: `/docs/components/sysc-terminal/` | [README](/home/nomadx/sysc-terminal/README.md:11), CLI flags/control socket, rendering source | Niri background engine, fonts, per-output lifecycle, effects/art, socket commands, resource limits and shell supervision. It is a wallpaper engine despite its name. Suite slot is currently disabled. |
| sysc-clipboard: `/docs/components/sysc-clipboard/` | [README](/home/nomadx/sysc-clipboard/README.md:73), `contrib/`, client package | Data-control requirement, encrypted persistence/Secret Service/key-file, user service, private socket, history/thumbnail operations, `--check` scope, privacy/retention and data-loss consequences of clearing. |
| sysc-wayland: `/docs/components/sysc-wayland/` | [README](/home/nomadx/sysc-wayland/README.md:85), `UPSTREAM.md`, public transport/bindings/scanner | Go module installation, connection/FD ownership, dispatch threading, protocol generation and XML provenance, version compatibility. This is a library, not a desktop daemon to enable. |
| sysc-metrics: `/docs/components/sysc-metrics/` | [README](/home/nomadx/sysc-metrics/README.md:21), exported readers/samplers | Library quick start, units and validity, first-sample baseline, permissions/unavailable devices, GPU backend limits, resource closure and shell integration. No new metrics server. |
| sysc-launch: `/docs/components/sysc-launch/` | [README](/home/nomadx/sysc-launch/README.md:3), exported engine/providers, diagnostic CLI | Library/CLI distinction, desktop-file discovery, ranking/providers, Niri activation, history/privacy. sysc-shell owns its UI; query can work without Niri. |
| sysc-walls: `/docs/components/sysc-walls/` | Remote [README](https://github.com/Nomadcxx/sysc-walls/blob/master/README.md#L17), [troubleshooting](https://github.com/Nomadcxx/sysc-walls/blob/master/TROUBLESHOOTING.md), `cmd/installer/`, `internal/config/`, `systemd/` | Idle screensaver, runtime/build dependencies, standalone system path vs suite user prefix, idle protocols, Kitty/multi-monitor effects, configuration, diagnostics and removal. Explain that screensaving does not establish a session lock. Preserve GPL-3.0 attribution. |
| sysc-plugins: `/docs/plugins/` | P1–P5; `catalog.json`, `catalog-meta.json`, `plugins/*/manifest.json`, `ATTRIBUTION.md` | User installation/catalog vs source, per-plugin requirements/configuration, author SDK/geometry, release/submission policy and third-party-source trust. Generate current listings from a pinned catalog; source-only plugins must not appear as installable releases. |

The requested list omits **sysc-notify**, which the current suite installs [S2], and **gSlapper**, whose installation can request sudo. Add companion sections or integration pages for both so the suite walkthrough is complete. Link sysc-Go as the animation dependency. Full standalone manuals for these three are an additional scope decision; a truthful suite dependency explanation does not depend on that decision.

## 4. Compatibility, privileges and version accuracy

Maintain one compatibility table with these fields: project/version, suite inclusion, CPU architecture, distro and version, compositor/protocol requirements, service manager, runtime libraries, source-build requirements, install route, privilege boundary, qualification evidence and known limits. Separate “code has a route”, “release asset exists” and “qualified configuration”. Unknown is an acceptable entry until someone supplies evidence.

### Required initial distinctions

| Area | What the initial docs must say | Evidence / qualification boundary |
|---|---|---|
| Full sysc suite | Arch family, Niri-first, normal-user install. Current component assets support amd64; publishing an arm64 installer does not make the suite installable on arm64. | S1/S2/S3. Generate architecture availability from enabled component assets, not the installer release matrix. |
| gSlapper | Arch helpers run as the user and elevate package work. Native code has Debian 13, Ubuntu 24.04, Ubuntu 25.x and Fedora 42+ package routes. | S1/S2. These routes do not bypass the complete suite's distro gate. Do not label the entire suite multi-distro today. |
| sysc-greet | Separate administrator installation: greetd/system configuration, greeter account and assets. Compositor/package/Nix/Void support needs review at the selected greet version. | Current greet installer/packaging; old local Void content alone is not evidence of current published support. Keep a spare TTY/recovery procedure beside activation instructions. |
| sysc-lock | User-prefix installer does not use sudo, modify PAM, enable or start the service. The sysc suite separately enables its user unit. Source build needs libpam headers; current amd64 binary has glibc/PAM/EGL/GLES runtime requirements. | [Lock README](/home/nomadx/sysc-lock/README.md:39), installation distinctions at lines 63–74. Installing prerequisites may require an administrator. Acquiring a lock and starting the owner are different operations. |
| sysc-shell/tray/clipboard | Run in the user session; units belong under user configuration. Document session environment, D-Bus conflicts and clipboard persistence separately. | Component sources. Do not prepend sudo to `systemctl --user`. |
| sysc-terminal | Standalone Niri wallpaper backend supervised per output by shell; current suite manifest disables its entry. | S2 and terminal README. A placeholder unit must not become an instruction. |
| sysc-walls | Standalone documentation installs binaries to `/usr/local/bin` with administrator privileges; sysc installs its pinned daemon under the user's prefix. | Walls README lines 38–45/165–178; S1/S2. Compare service templates before copying paths. |
| sysc-wayland/metrics/launch | Module dependency/build guidance; launch also provides a diagnostic command. | Their READMEs. Do not invent daemon services or graphical settings for libraries. |
| Plugins | User-owned subprocesses, required commands and host capability checks. External packages/services may have their own administrator setup. | P5. Process separation and host API consent do not establish an operating-system sandbox. |

The suite pin currently includes shell v0.1.0, clipboard v0.1.2, walls v1.0.2, notify v0.1.0, tray v0.1.1 and lock v0.1.0, with terminal disabled [S2]. Read `internal/pin/pin.json` **at the installer tag being documented**, not only at main. The checked v0.1.1 tag reports suite release v0.1.1. Component HEADs can describe newer behavior than those assets; review each imported page against its pinned release.

### Recommended publication model

Start with one documentation edition for the current supported suite. Each component overview states whether its instructions describe the suite-pinned version, an independently released component, or development source. Record `repository`, full source commit, tag if applicable and verification date in a central provenance table. Link both the central documentation edit path and the component implementation reference.

Use the existing suite manifest as the input for the component-version/architecture table; do not invent a second suite lockfile. Keep imported prose in the central repository and record its provenance, licensing and reviewed differences. This gives reviewers stable content and avoids a production build cloning floating default branches. When a component release changes relevant behavior, its maintainer supplies a documentation PR or a source diff for review.

If a second supported suite release needs separate instructions, add an explicit archived snapshot under `/docs/releases/v<version>/` and a current/archive label. Scope search to current docs by default; label archive results if included. Retain old URLs or static redirect pages. Do not call Fumadocs navigation a version selector until that behavior is implemented and checked.

## 5. Reference boundaries and contributor guidance

**Manual guides** explain tasks, ownership and recovery: installation, upgrading, compositor/session setup, privilege prompts, lock security, clipboard privacy, plugin trust and troubleshooting. A CLI flag dump cannot replace these instructions.

**Generated tables** cover deterministic facts from reviewed source: suite versions/assets, published plugin listings, required commands and protocol versions. Keep generation as a small Node script in `docs-site/scripts/` if the table needs it; use built-in JSON parsing and existing content files. Check generated output into the repository or build it from checked-in inputs, then fail CI on drift. Never download plugin binaries or execute a plugin during a documentation build.

**Go API reference** links to `pkg.go.dev` for public modules/packages, at the selected version when available. Document short worked examples for sysc-wayland ownership/dispatch, metrics sample validity, launch activation/history, clipboard clients, tray presenter clients and `sysc-shell/plugin/v1`. Keep internal package internals in contributor architecture pages only where contributors need them. Do not generate a public API contract from `internal/` packages.

**Protocol reference** belongs to the repository that owns it. Reuse the lock protocol note and public plugin message/node definitions, with version and direction explained. For sockets/DBus, include endpoint discovery, identity checks, framing/bounds, supported verbs, timeouts/reconnect and error semantics. Mark private implementation interfaces as private; public documentation does not make them stable APIs.

Developer build instructions must honor repository-specific constraints. In particular, terminal's AGENTS guide limits local checks to named packages/tests and rejects broad/race runs on the laptop. Shell deployments use `scripts/deploy` and its stamped-build checks. A docs-only PR should run site checks without rebuilding the full desktop or connecting to a live Wayland session.

## 6. Plugin submission: existing rules and proposed review policy

### Existing technical requirements, verified from source

1. A plugin directory carries `manifest.json` and an executable speaking `sysc-shell/plugin/v1`. Authors add a library package and a `cmd/sysc-plugin-<name>/main.go` entry point, then register it in the Makefile/README [P1]. The manifest validator checks the host's accepted shape and capability names [P3]. Use that validator rather than duplicating its schema in prose.
2. Interactive nodes need IDs, accessible names/roles and events; unknown icons or malformed trees fail. Authors run `v1.Validate` and `plugin/lint.Tree` on representative views. Check horizontal bar 240×32, tooltip 280×200, manifest panel dimensions and narrow sidebar widths 32×32/64×32. Panels with host-included settings have a smaller usable box than their declared dimensions [P1]. Pin the SDK version and use its constants because geometry/protocol details can change.
3. Release tags use `<dir>-v<version>`, matching the committed manifest version. Current publishing guidance requires that version on `main` before tagging. Release jobs build amd64 and arm64, publish assets and open a catalog PR for human merge [P2/P4]. Calendar requires native-architecture CGO/libecal builds; a blanket “all plugins are static Go binaries” claim would be wrong.
4. Archives contain one plugin-ID root directory, unchanged manifest and the executable at the manifest exec path. Catalog metadata supplies required category/author and optional license/homepage/description/screenshot; manifest fields supply runtime name/description/version/protocol/capabilities/commands [P2]. License metadata is optional in the current documented technical shape; see the proposed reviewer requirement below.
5. `catalog validate` checks catalog metadata against the **tagged** manifest, not the working tree. `-fetch` checks release/media payload size and SHA256. `-community` requires a screenshot [P3]. The current catalog schema uses HTTPS URLs with a local HTTP exception for testing, lowercase SHA256 and bounded payloads; current source sets a 4 MiB catalog, 64 MiB asset and 256 KiB README limit.
6. Publishing guidance asks for a representative screenshot no larger than 2 MiB or 1920×1080; it makes screenshots optional for first-party entries and required for community entries [P2]. **Enforcement gap:** the inspected `validate -community` checks presence; `-fetch` uses the generic 64 MiB asset bound for screenshots and does not check image dimensions. Document these as author/reviewer requirements until an implementation enforces them. Do not claim CI proves the screenshot dimensions.
7. Third-party sources keep `catalog.json` at the default branch root and users add their repository URL in the shell's Plugins → Sources settings [P2]. Current built-in source points to `Nomadcxx/sysc-plugins` [P5]. This audit did not find a verified separate community catalog or established submission queue.

### Proposed submission page, requiring owner policy approval

Recommend a pull request to `sysc-plugins` for official-source inclusion. The owner must decide whether to accept third-party catalog entries there, require source in the repository, or create a separate community catalog. Do not publish an invented submission URL or promise an acceptance SLA.

Ask a submission to include: source repository and immutable release, plugin ID/version, supported shell/protocol version, required commands and distro packages, requested capabilities and why, credential/data handling, screenshot, license/asset attribution, focused protocol/geometry check output, release assets with hashes/sizes, and install/update/remove instructions. Require an explicit source license for reviewed inclusion as an editorial policy; this is stricter than today's optional catalog license field.

The maintainer review covers executable/manifest consistency, dependency and capability changes, user-visible failure behavior, accessible names and compact layouts, secrets in screenshots/logs, attribution, destructive actions, lifecycle cleanup and reproducible packaging. State what human review establishes and what remains the user's trust choice. Host capabilities constrain host calls; a same-user executable can still use resources outside those calls [P5]. Checksums establish content integrity, not a security endorsement.

The submission chapter must distinguish technical validation, catalog publication and editorial acceptance. Until the owner chooses a community route, explain how to publish an independent source and how to propose inclusion through the existing repository's pull requests; do not imply a registry exists.

## 7. Implementation sub-plans

The implementing engineer should create an isolated worktree from current `origin/main`, preserve unrelated local files and make reviewable commits per sub-plan. These are documentation work packages; they do not authorize runtime application changes or publication.

### A. Establish the source and route contract

**Files:** create `docs-site/README.md`, `docs-site/content/docs/reference/sources.md`, `docs-site/content/docs/reference/compatibility.md`; later create `docs-site/scripts/check-content.mjs` from the upstream checker.

1. Record the chosen upstream greet commit and lockfile SHA, latest deployed greet SHA, sysc tag and component release refs. Compare the local baseline with any selected later snapshot; require read-only PR checks and remote-image guards even if the copied source lacks them. Fetch/read immutable sources without checking out over dirty repositories.
2. Confirm the installer README matches the tagged suite; use it for the installer walkthrough. Compare local component source leads with the selected releases and list material discrepancies.
3. Write the route inventory from section 3 into navigation metadata. Make each project overview identify its role and owner before adding details.
4. Record license/attribution for copied prose/assets, including GPL-3.0 walls content. Prefer new factual instructions and links where copying would complicate licensing.

**Acceptance:** a reviewer can identify the version and source behind each support claim, all twelve requested projects have an intended route, and suite dependencies notify/gSlapper have a place. Missing release evidence remains labeled unknown.

### B. Reuse the existing site shell

**Files:** create `docs-site/` by copying reviewed greet source/package-lock, excluding `node_modules`, `.next`, `.source` and `out`; adapt `next.config.mjs`, `lib/shared.ts`, `lib/layout.shared.tsx`, `app/layout.tsx`, `app/docs/[[...slug]]/page.tsx`, `components/brand.tsx`, `components/greeter-header.tsx`, `components/docs-home.tsx`, `app/global.css`, `public/`, navigation and check scripts.

1. Keep Next/Fumadocs versions and static export. Set explicit `DOCS_BASE_PATH` for local empty path and Pages `/sysc`; reuse its public equivalent in browser/search paths. Do not derive repository identity from whether a build runs in CI.
2. Replace greet branding, title and source links with SYSC. Point central edit links to `main`; preserve component provenance links separately.
3. Build the instructional home and component sidebar using existing Fumadocs layout/link components. Retain search, article TOCs and copyable code. Give the site a pure black canvas, legible text, borders and visible focus.
4. Adapt content/export checks for SYSC routes and branding. Remove exact joke text, greet postinstall assertions and the inherited fixed chapter count.

**Smallest proof once implementation is requested:** `cd docs-site`, `npm ci`, `npm run check`; repeat with `DOCS_BASE_PATH=/sysc npm run check`. Check generated output contains all declared routes, local assets, source links and non-empty search data. The check script must use the same base-path setting as Next.

**Acceptance:** the site works at both paths with no network image fetch during the build; no `/sysc-greet` brand or source-branch references remain except intended links to that project.

### C. Write installation, support and recovery first

**Files:** `docs-site/content/docs/start/`, `guides/`, `troubleshooting/`, `reference/`, `components/sysc/`, `components/sysc-greet/`, `components/sysc-lock/`; update the associated `meta.json` files.

1. Write supported installation paths, normal-user/root boundaries and release checksum instructions from S1/S2. Include weather input/privacy, 80×24 controls and conflict choices.
2. Write first-launch checks and session setup, preserving the distinction between TTY/SSH enabling units and an active Niri session starting them.
3. Review greet import against its chosen release and add service activation/recovery; review lock standalone vs suite behavior and security claims.
4. Write upgrade/uninstall instructions from the actual stamp/restore logic. Explain recorded ownership, what remains, partial results and existing config preservation. Verify paths against XDG overrides and user unit templates.

**Acceptance:** someone can follow the install, verify, recover and remove journeys without following GitHub planning documents. No arm64/Fedora/Debian complete-suite support claim, sudo-prefixed user-service command or unlock bypass appears.

### D. Complete component manuals and integration guides

**Files:** remaining `docs-site/content/docs/components/<project>/`, shared theme/wallpaper/session guides and architecture/interface pages.

1. Use the inventory as a source checklist, compare each claim with its release and author the missing instructions. Preserve startup/environment/socket ownership boundaries.
2. Describe shell, tray, clipboard and notification handover together. Explain wallpaper ownership among shell, terminal, walls and gSlapper.
3. Add metrics/launch/Wayland worked examples and versioned public API links. Link upstream development policies instead of prescribing the same test/deploy commands to all repositories.
4. Add per-component read-only diagnostics, expected outputs, known limits and maintainer issue links. Check example data for secrets and usernames/host paths.

**Acceptance:** all requested components have substantive docs, libraries have working usage explanations, and the integration guides identify the process/configuration that owns each feature. Unqualified behavior has an explicit qualification boundary.

### E. Publish plugin authoring and submission guidance

**Files:** `docs-site/content/docs/plugins/`, provenance references, optional deterministic listing script under `docs-site/scripts/`.

1. Import/rewrite P1/P2 for the selected SDK and plugin catalog refs; include a small existing plugin as the example rather than designing a new framework.
2. Document manifests, handshake/capability errors, accessible trees and geometry checks using the SDK's current definitions/constants.
3. Write release/catalog steps, CGO exception, update consent and removal. Generate the published listing from the chosen catalog; identify source-only entries.
4. Have the owner choose the submission route/policy before presenting proposed requirements as mandatory. State the screenshot enforcement gap and link its responsible tools for a later separate code change if desired.

**Acceptance:** an author can move from source to validated views, tagged assets, catalog publication and a reviewable submission. The text distinguishes requirements enforced by code from maintainer review policy.

### F. Add documentation CI and review artifacts

**Files:** create `.github/workflows/docs.yml`; adapt `docs-site/scripts/check-content.mjs` and `check-export.mjs`; add only a small route/provenance check if existing checks cannot own it; update `docs-site/README.md`.

1. Run a stable `docs-check` job on pull requests and pushes to main. For a required branch-protection check, trigger on all PRs and use a small changed-files gate that succeeds when docs and their inputs did not change; this avoids required checks hanging on path-filtered PRs. Relevant inputs include `docs-site/**`, workflow, suite pin and any checked-in generation sources.
2. Give that job read-only contents permission, `persist-credentials: false`, a bounded timeout and cancellation for superseded PR builds. Install Node 22 and run `npm ci`; cache npm downloads keyed by the lockfile, never exported production output.
3. Run the explicit `/sysc` check, upload `docs-site/out` as `docs-preview-<PR-or-SHA>` with seven-day retention, and record SHA/base path/build commands in the job summary. Do not execute app installation commands or plugin assets.
4. Download/serve the artifact for review under its `/sysc` prefix. One documented local preview recipe: create a temporary preview root, extract output into its `sysc/` subdirectory and serve that root with `python3 -m http.server`; open `/sysc/docs/`. The author can use the existing `npm run dev` for a local root-path preview.
5. Check internal pages/assets/anchors against the export using a small stdlib script if Fumadocs does not already validate them. Keep external link availability checks scheduled/advisory with retries to avoid blocking edits on rate limits. Validate navigation references and provenance metadata; assert suite tables agree with the tagged/checked-in pin.

**Security invariant:** MDX and npm scripts execute code during a build. PR builds must have no deployment secrets or write tokens. Do not use `pull_request_target` to run PR source and do not deploy a downloaded PR artifact through a privileged `workflow_run`. Production must rebuild reviewed main with its locked inputs.

**Acceptance:** a docs PR has build/link/content evidence and a downloadable preview; a fork PR cannot publish the production site. Existing application CI and release workflows keep their current responsibilities. No new hosted preview service is required.

### G. Deploy, migrate links and prepare rollback

**Files/settings:** `.github/workflows/docs.yml` deployment job; repository GitHub Pages/environment/branch protection; optional static redirect pages; component README documentation links after publication approval.

1. Configure Pages for Actions builds. Restrict deployment to main and the `github-pages` environment; choose required reviewers if the owner wants a manual publishing gate. Use a single `pages` production concurrency group with `cancel-in-progress: false`.
2. Rebuild/check reviewed main, upload only `docs-site/out` with the Pages artifact action, then deploy. Grant `pages: write`/`id-token: write` only here; use full reviewed Action SHAs. Record documentation commit, provenance table and artifact digest in the deployment summary.
3. Perform a post-deploy read-only smoke check for home, one article per project, search payload and local assets at `/sysc`. Record the Pages deployment/run URL. Keep the prior site available until the new routes pass.
4. Link the umbrella site from sysc and component READMEs once public routes work. Keep greet's current URLs working; initially cross-link from its site. A later consolidation should add static redirect/fallback pages under the old greet origin, preserve path/query/fragment intent where possible and verify old deep links. Next server redirects cannot serve a static Pages export.
5. For rollback, revert the documentation change on main and rerun the same workflow with the same locked dependency inputs. If recovering an environment/build failure, redeploy a previously verified retained **trusted production** artifact through an explicitly approved rollback procedure. Keep a 30-day ordinary Actions artifact for trusted builds if artifact recovery is part of the policy; Pages upload alone does not guarantee that retention. Avoid force-pushing or moving release tags.

**Acceptance:** public routes work, old greet deep links still resolve, production publishes only trusted main, and the documented rollback identifies a known good commit/artifact. A docs rollback does not alter installed component binaries or their release pins.

### H. Establish maintenance ownership

**Files:** `docs-site/README.md`, optional `.github/CODEOWNERS` if the owner assigns actual reviewers, `docs-site/content/docs/developers/documentation.md` and central provenance table.

The owner appoints one site maintainer for dependency/build/deploy reliability and a reviewer per component for behavioral accuracy. Component owners review relevant chapters with release changes; plugin maintainers review protocol/submission text. Record real GitHub handles only after the owner chooses them. The documentation maintainer cannot infer component qualification from successful website CI.

At each suite release, read the pin from the tag, refresh the table, check affected installation/runtime/privilege chapters and record any support qualifications. At each SDK change, review manifest/protocol/geometry and plugin submission guidance. Review dependency/security updates in separate small PRs, using the lockfile and export checks. A monthly maintenance pass can run external-link checks and inspect deployment failures without running the whole desktop test suite.

Do not create a new cross-repository status tracker inside these docs. Link existing issue trackers and respect shell/terminal beads conventions. Register this sysc handover with the parent repository's existing plan index only if that is part of the parent's authorized documentation change; this commissioned file is the only write from this agent.

**Acceptance:** publishing, content review, dependency updates and release synchronization each have an assigned maintainer and a documented command/procedure. Unknown owners remain decisions, not fictional team names.

## 8. Decisions for the owner and recommended defaults

| Decision | Recommended default | When it blocks work |
|---|---|---|
| Site repository/host | `Nomadcxx/sysc/docs-site`, GitHub Pages `/sysc/`; reuse current greet stack. | Confirm before configuring hosting or publishing. Research/content can proceed. |
| Greet consolidation | Cross-link first; preserve its current site until migration routes and content pass. | Only blocks retiring/redirecting the old site. |
| Visual direction | Pure black canvas, existing RAMA accents/type, instructional home, static masthead. | Confirm screenshots during site implementation review; this plan does not redesign greet itself. |
| Version policy | One current suite edition, component provenance and stable URLs; add archives when another supported suite needs them. | No multi-version infrastructure required for initial site. |
| Community submissions | Start with source/release PR proposals to `sysc-plugins`; owner chooses first-party-only versus community entries and license policy. | Blocks calling proposed acceptance rules official and publishing a submit endpoint. |
| Preview hosting | Read-only PR build and downloadable static artifact; production uses reviewed main. | A hosted preview needs an approved destination/security model; artifact preview does not. |
| Additional components | Include notify/gSlapper integration pages and sysc-Go reference links. | Only full additional standalone manuals need expanded scope. |
| Maintenance | Owner names site/deployment maintainer and component reviewers. | Required before ongoing publication responsibility transfers. |

## 9. Completion proof for the future implementation

The site is ready for approval when all twelve requested projects have reviewed content; first-install, customize, troubleshoot, developer and plugin-author journeys have complete routes; support/privilege/version distinctions match source; internal pages/assets/anchors resolve; static search works under `/sysc`; and PR/production workflow permissions honor the trust boundary.

Review desktop and mobile navigation, keyboard-only search/menus/copy buttons, 200% zoom, reduced motion, focus visibility and code/table overflow. Include screenshots of home, a procedure, a component reference and plugin submission at desktop/mobile widths. Use existing browser tooling or focused manual evidence; add a browser-test dependency only if maintaining a repeatable failing interaction requires it.

Attach the exact documentation/source SHAs and site-check output. Record remaining unqualified distro/hardware combinations and any external-link failures. This research commission did not run component tests or privileged installs. Its limited existing-site observations include desktop, one 375px article and a successful search; they do not qualify the future site or replace the local export review described above.
