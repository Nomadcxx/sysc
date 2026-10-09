# SYSC docs coverage and UI review

**Reviewed:** 2026-10-10

**Scope:** SYSC documentation source and routes, the static site at 390/720/1440 px, and the public Noctalia and DankMaterialShell documentation entry points.

## Coverage result

The agreed SYSC scope has a route for each of its 15 projects: 14 component references and the `sysc-plugins` authoring section. The site also covers installation, configuration, recovery, development, plugin submission, compatibility, privileges, paths, and source provenance. I found no missing project route in the approved scope.

| Reader task | SYSC coverage | Result |
|---|---|---|
| Install and verify | Requirements, install, wizard, verification, compatibility, and suite pins | Covered |
| Configure and recover | Service integration, Niri startup, themes, wallpapers, conflicts, upgrade, uninstall, and troubleshooting | Covered |
| Understand the projects | Component references state each project's role and whether SYSC installs it | All in-scope projects have a page |
| Develop and extend | Architecture, builds, interfaces, releases, docs workflow, plugin catalog, protocol, UI rules, testing, publishing, and submissions | Covered |
| Check sources and limits | Reviewed source commits, exact SYSC v0.1.1 pins, module versions, privileges, paths, and compatibility statements | Present |

The scope intentionally gives `sysc-notify` and gSlapper integration guidance and treats sysc-Go as a dependency reference. Those pages are not full standalone manuals. The plugin section covers `sysc-plugins` as an authoring and catalog workflow. These route depths match the approved scope.

The compatibility page leaves machine-specific qualification unclaimed because the reviewed SYSC source does not establish it. The install guidance names the Arch-family and Niri constraints and handles other projects' standalone support separately.

## Competitor comparison

I reviewed the public navigation and first-use paths on [Noctalia](https://docs.noctalia.dev/noctalia/) and [DankMaterialShell](https://danklinux.com/docs/) on 2026-10-10. I did not test their installers or audit every page.

| Pattern | Noctalia | DankMaterialShell | SYSC assessment |
|---|---|---|---|
| First use | Installation, running the shell, then configuration; a separate NixOS route | Quick Start, Getting Started, install and install management | SYSC's Requirements → Install → Wizard → Verify path gives the equivalent task sequence |
| Main navigation | Compositor setup, configuration, IPC, bar and widget reference, desktop features, services, themes, automation, plugins, and templates | Shell setup, compositor configuration, themes, CLI tools, plugins, greeter, companion utilities, contributing, support, and changelog | SYSC uses five task groups: Start, Guides, Components, Developers, and Plugins. This matches the smaller source scope |
| Platform breadth | Pages for Niri, Hyprland, Sway/Scroll, Umbriel, Mango, Labwc, and KDE | Multiple compositor and distribution guides, plus separate tools and greeter docs | Do not add these topics as SYSC support claims. The SYSC source and compatibility page define a narrower supported path |
| Versions | The overview identifies the current Noctalia release as v5+ | The version selector exposes several documentation paths, including 1.2 through 1.7 | SYSC documents the current v0.1.1 suite. Add an archive or selector when another supported suite release needs separate instructions |
| Contribution and support | Plugin development and template guidance | Contributing, registry contribution, support, and changelog routes | SYSC documents component development and plugin submissions. A single support-routing page could help readers choose the right repository, but needs maintainer policy and ownership first |

Their compositor lists, settings, utility applications, plugins, and release histories describe those products. Keep SYSC guidance tied to SYSC source and use the competitor sites as navigation references.

## UI and heading review

The shared header and home hero now use the SYSC logo from `sysc-greet/assets/logo.png`, with “Documentation” below it. The brand no longer says “Desktop.” Article titles render at 32–48 px, section headings at 20–24 px, and third-level headings at 18–20 px. At 720 px, the SYSC article title and section heading render at 32 px and 20 px. The competitor captures measured 35/29 px on Noctalia and 48/32 px on DankMaterialShell at the same width; SYSC gives section headings more separation from the page title.

The slash frames remain around section headings at a smaller size and weight. The 390 px browser check showed 45 section labels wrapping, including the sysc-shell headings “Open panels and change settings” and “Companion service failures.” I shortened those labels across the component, guide, developer, plugin, reference, start, and troubleshooting pages. The final browser check found no wrapped section headings across the article set at 390 px.

The foreground `#f1f1f1`, muted text `#a3a3a3`, and RAMA link accent `#ff6678` have contrast ratios of 18.59:1, 8.33:1, and 7.42:1 against black. The browser check also confirms the mobile navigation opens with Enter and closes with Escape, and that the site has no horizontal overflow at 390 px. It checks representative component, guide, start, plugin, reference, and troubleshooting pages at 390, 720, and 1440 px.

Screenshots are saved beside this report:

- `home-desktop.png`, `home-tablet.png`, `home-mobile.png`
- `sysc-shell-desktop.png`, `sysc-shell-tablet.png`, `sysc-shell-mobile.png`
- `plugin-ui-desktop.png`, `plugin-ui-tablet.png`, `plugin-ui-mobile.png`

## Follow-up items

1. When a component's default branch changes, update the affected guidance and record the revision behind it. Keep exact component release tags in installation steps for the SYSC suite pin.
2. Consider a central support-routing page if maintainers choose an owner and repository policy. Current pages link to project sources and issue trackers.
3. Add versioned documentation only when another supported SYSC suite release needs its own instructions.

## Audit limits

The competitor review covered public landing pages, navigation, and first-use flow. It did not test their installers or audit every page. The local browser check covers layout, heading hierarchy, keyboard operation of the mobile navigation, branding, and contrast tokens. Screen-reader testing and a full 200% zoom review remain open.
