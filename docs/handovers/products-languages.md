# Handover: localize sysc-shell and companion products

Prepared 11 October 2026. Related installer baseline: `Nomadcxx/sysc` main at `cac86d0`.

## Objective

Add Spanish, Portuguese, Japanese, Korean and Russian to SYSC product interfaces, beginning with sysc-shell. Preserve existing behavior, accessibility and English fallback. Follow the [installer handover](installer-languages.md) for the installer-specific implementation.

This is a staged implementation brief, not a claim that the products already expose a language setting. Targeted source searches found the installer's i18n package but no equivalent product-wide translation layer in the inspected shell/config and companion code. Reinspect each repository before choosing implementation details.

## Repositories and ownership

Local sibling repositories are under `/home/nomadx/`; use their own git status, branch and applicable AGENTS.md. Current sandbox write roots cover `/home/nomadx/sysc` and `/tmp`, so editing siblings may need the normal approval or an authorized checkout. Do not bypass workspace restrictions.

| Repository | First user-facing scope / source landmarks |
|---|---|
| `sysc-shell` | Settings, launcher-owned controls, quick settings, power/session actions, notification controls, clipboard UI, tray drawer, weather/calendar labels, plugin catalog controls. Start at `internal/shell`, `internal/config`, `internal/ui`, `internal/render`. |
| `sysc-lock` | Lock screen prompts/status, CLI help and setup guidance. Inspect `internal/lockd/view.go`, `internal/lockd/text.go`, `internal/input`, `internal/config`, `internal/options`, `cmd`. |
| `sysc-notify` | SYSC-owned help/errors and daemon controls. Notification text/actions supplied by applications retain their source language. Inspect `cmd` and `internal`. |
| `sysc-clipboard` | CLI help/errors and persistence guidance. Shell owns the graphical clipboard panel. Stored clipboard bytes retain their contents. Inspect `cmd/sysc-clipboard/main.go`, `internal/daemon`, `internal/store`. |
| `sysc-tray` | SYSC-owned daemon guidance. Shell owns the tray drawer/menu presentation; application-owned labels come through the tray protocol. Inspect `internal/menu`, `internal/presenter`, `internal/app`. |
| `sysc-terminal` | Help, mode descriptions and SYSC-owned on-screen hints, if any. Preserve art content, mode IDs and terminal geometry. Read its AGENTS.md first. |
| `sysc-launch` | Standalone help/status and SYSC-owned labels. Inspect `apps.go`: it already notes process-global locale hazards in the desktop-entry dependency. Preserve application-provided localized names and Exec semantics. |
| `sysc-plugins` | SYSC-owned catalog descriptions and official plugin UI as a later pass; inspect the manifest/protocol before changing presentation fields. |

The user excluded sysc-greet, sysc-walls and sysc-go from the original new-package work. Keep them out of this initial localization scope unless requested. sysc-wayland and sysc-metrics are libraries; they need no user language selector.

Record each product's current commit and release before editing. Independent repository changes need independent review and releases; an installer commit cannot update the companion binaries.

## Shared user experience

- Use the same proposed codes as the installer: `es`, `pt`, `ja`, `ko`, `ru`; preserve English and any product locales already present.
- Use native language names: Español, Português, 日本語, 한국어, Русский. Document the Portuguese register and seek fluent review.
- Offer one language preference in the shell's existing Settings layout. Follow the current control style and configuration lifecycle; do not add a setup wizard or recurring banners.
- Default to the user's session language, with English fallback. An explicit saved product preference takes precedence over environment detection.
- Design companion inheritance after inspecting existing shared configuration readers. Lock already has shell-following configuration coverage; extend a fitting path instead of adding a separate service or IPC system.
- Keep the installer language independent of saved desktop language unless a deliberate, documented seed change is requested. Preserve existing user config on reruns.
- Decide whether language changes apply live or require restart based on current reload behavior. Tell users only when they need to take an action.
- Keep language selection separate from keyboard layout, IME, timezone, number/date formats and units. Do not change those as an incidental effect of translation.

## Architecture: choose the smallest fitting change

First find an existing responsible catalog/config layer. If none exists, use a small product-owned translation package with embedded catalogs, stable message keys and English fallback. The installer demonstrates this pattern with stdlib embed/JSON and an existing language parser.

Check each product's installed dependencies before adding x/text. Avoid a new cross-repository dependency solely to share a five-case switch. Keep locale identifiers and translation terminology aligned through documentation until actual duplication warrants a shared library.

Resolve locale at the application's existing state boundary. Pass or store it through the current view model/config ownership. Do not mutate global process environment to switch UI language, especially in a concurrent daemon or the launcher.

Add an optional language config field with a default that keeps old files valid. Preserve unknown or unsupported values through a safe fallback and follow the repository's config-write rules. Check that old binaries' readers can tolerate the new field before calling the change compatible.

Use stable keys for labels and states. Continue using enum values, route IDs and protocol names for logic; never compare translated strings to select an action. Translate visible strings near the view layer instead of translating errors used for programmatic decisions.

## Content ownership and safety

Translate SYSC's own buttons, headings, placeholders, empty states, confirmation questions and recovery instructions. Preserve:

- Application names, desktop-entry localized names, notification summaries/bodies/action labels and tray menu labels supplied by other applications.
- Clipboard text, filenames, paths, SSIDs, device names, artist/title metadata, user plugin content and geocoder-provided place names.
- D-Bus names/signatures, JSON field names, CLI flags, service names, plugin IDs and package identifiers.
- PAM responses and authentication behavior. Localize SYSC-owned authentication prompts/status without exposing passwords or turning an operational error into an authentication success.

Preserve formatting placeholders and plural meaning. Start with the quantities the UI actually displays. Use an existing plural helper if present; add plural handling only where counts need it, including Russian forms. Do not build a general translation service.

Logs and raw external diagnostics can remain in their original form when useful for support. Pair a diagnostic with translated user guidance where the user must act. List intentional English-only surfaces in the final report.

## Rendering and input checkpoints

The shell already uses a shaped text renderer and font map. Inspect `internal/render/fontmap.go`, `text.go`, `paint.go`, and existing text/truncation tests before changing rendering. Reuse font fallback and measured layout; do not substitute rune or byte counts for glyph advances.

Japanese and Korean require script coverage in installed fonts. Russian requires Cyrillic. A generic usable-font check does not establish these scripts are available. Document relevant optional system fonts, such as Noto CJK, after confirming renderer support; avoid bundling font assets or adding packages without a need and license review.

Check labels, multiline hints, ellipsis, tooltips, popout sizing and text fields at supported scale factors. Spanish/Portuguese/Russian text may be longer than English; Japanese/Korean have different line-break behavior. Preserve control target sizes and existing design tokens. Check missing-glyph output with real fonts.

UI translation does not establish Japanese/Korean text-entry support. Inspect the existing Wayland text-input/IME path and state any limitations. sysc-lock's `internal/lockd/keymap.go` builds an XKB compose table from locale; do not replace that keyboard/input locale with the UI language or promise IME support without evidence.

## Suggested delivery sequence

1. Inventory shell-owned visible strings and current config/reload behavior. Write down the English source keys and a short glossary (lock, suspend, launcher, tray, clipboard history, wallpaper, plugin).
2. Implement shell locale resolution, fallback and the existing Settings control. Translate one representative Settings/panel slice to establish the pattern without altering service/protocol behavior.
3. Complete shell-owned surfaces in the table above for the five requested languages. Audit visible dynamic statuses and plurals. Check that external content remains untouched.
4. Extend lock prompts/status and its established configuration-following path. Treat authentication and input behavior as invariants.
5. Localize companion help and actionable guidance. Leave backend-only surfaces alone unless users see them.
6. Address official plugin presentation only after deciding whether catalog/manifest schemas already support localized text. Preserve protocol compatibility and third-party fallback.
7. Update each product's docs with language selection, fallback, fonts and remaining limitations. Rebuild/release affected products through their own workflows, then update installer/AUR pins only when verified assets exist.

Avoid changing packaging, language support, protocols and authentication in one undifferentiated commit. Finish a coherent product slice before proceeding to the next repository.

## Minimal proof and completion report

Use each repository's current checks. A small catalog parity/placeholder check and locale/config fallback regression cover the new invariants; avoid copying tests for every string. Add a rendering regression only for behavior you change. Run the affected package checks first and the repository's release gate before publication.

Manually review Settings, a long-label panel, notifications, clipboard, tray menus and lock prompts for all five languages with real fonts. Check compact windows, scaling, focus, text entry and destructive confirmations. Keep live lock/auth checks under the repository's approved hardware workflow; do not interrupt an active desktop to collect evidence.

For each product report:

- Commit, branch and publication/release status.
- Translated surfaces, locale precedence, saved-preference behavior and fallback.
- Checks run and actual font/IME/hardware coverage.
- Remaining English strings, fluent-review needs and unresolved layout/input limits.

Keep the docs honest about released binaries versus current source. The Go installer remains the preferred setup route, AUR follows it, and full Debian/Fedora suite support remains planned.
