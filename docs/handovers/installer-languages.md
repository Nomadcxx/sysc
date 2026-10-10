# Handover: expand Go installer languages

Prepared 11 October 2026. Source baseline: `cac86d0` on `Nomadcxx/sysc` main.

## Objective

Add Spanish, Portuguese, Japanese, Korean and Russian to the Go installer's existing translation system. Preserve English, Simplified Chinese, German and French. Keep the installer layout, keyboard controls and install behavior intact.

This document hands off implementation; it does not add translations. The companion [product handover](products-languages.md) covers sysc-shell and the other programs.

## Checkout and project context

- Repository: https://github.com/Nomadcxx/sysc
- Working checkout used for this task: `/home/nomadx/sysc/.worktrees/aur-readmes/sysc`, branch `main`.
- Primary `/home/nomadx/sysc` contains unrelated work and independent AUR repositories. Inspect status before editing; do not stage the outer `aur/` directory.
- Read applicable AGENTS.md instructions. Use existing packages and patterns, avoid new dependencies, and leave a small runnable check for nontrivial logic.
- Git identity: `Nomadcxx <noovie@gmail.com>`. Preserve hooks and omit attribution trailers. Ask before using external review tools.
- Beads tracking belongs in `/home/nomadx/sysc-shell`; do not initialize another database in the installer checkout. The previous audit/guidance tasks `sysc-1091` and `sysc-1092` are closed.
- Docs: https://nomadcxx.github.io/sysc/docs/

The previous task hardened installer ownership and uninstall recovery, fixed bootstrap dispatch, enabled sysc-terminal without a service, enabled tray presentation in fresh configs, and removed unsupported plugin defaults. Preserve these changes. Current source skips the empty plugin page. The released v0.1.1 binary predates these fixes; a main-branch push does not update that binary.

## Current implementation

| File or area | Current behavior / next action |
|---|---|
| `internal/i18n/i18n.go` | Embeds `catalog/*.json`; defines Locale constants and the explicit `order` slice; loads only locales in that slice. Extend these existing structures. |
| `internal/i18n/catalog/en.json` | English source catalog, currently 100 keys. Copy its key set for each new catalog; re-count at implementation time. |
| `internal/i18n/i18n.go: Match` | Trims input, removes encoding/modifier suffixes, converts underscores to hyphens, parses with existing `golang.org/x/text/language`, matches base language and falls back to English. |
| `internal/i18n/i18n.go: T` | Uses the requested catalog, then English. Preserve fallback. Do not silently accept incomplete finished catalogs. |
| `internal/i18n/i18n.go: Next` | Implements the F9 cycle using `order`. Preserve the existing four-language order and append new languages. |
| `internal/i18n/i18n_test.go` | Checks nonempty/key-complete catalogs, locale matching, docs guidance and Niri command spelling. Some loops enumerate only four locales; cover the expanded order. `ja_JP` currently expects English and must change. |
| `cmd/sysc/main.go` | Both flag parsers advertise four values for `--lang`. Installation selects explicit `--lang`, otherwise `LANG`. Uninstall currently matches only its explicit flag. F9 refreshes input placeholders. |
| `internal/ui/wizard.go` | Locale flows through translated titles, controls, review and advice. F9 should preserve answers and the current page. |
| `internal/ui/chrome.go` | Existing guidance panel has charcoal `#161616`; the rest of the installer stays black. Use the current wrapping and terminal-width helpers. |
| README and `docs-site/content/docs/` | Search language lists, `--lang`, F9 and screenshots/captions; update claims that describe supported languages. |

Useful searches:

```sh
rg -n 'Locale|Match\(|Next\(|--lang|F9|zh-Hans' internal cmd README.md docs-site/content
rg -n 'fmt\.(Print|Sprint)|errors.New|Reason:' cmd/sysc internal/install internal/preflight
```

The catalog does not cover all CLI output and low-level errors. Record remaining English surfaces after translating the existing catalog. Translate installer-owned user guidance at its responsible layer; retain raw OS/tool diagnostics as useful details. Do not change install-engine semantics to localize messages.

## Language decisions

| Language | Proposed catalog code | Display name | Examples to match |
|---|---|---|---|
| Spanish | `es` | Español | `es_ES.UTF-8`, `es_MX`, `es-419` |
| Portuguese | `pt` | Português | `pt_BR.UTF-8`, `pt_PT`, `pt-BR` |
| Japanese | `ja` | 日本語 | `ja_JP.UTF-8`, `ja-JP` |
| Korean | `ko` | 한국어 | `ko_KR.UTF-8`, `ko-KR` |
| Russian | `ru` | Русский | `ru_RU.UTF-8`, `ru-RU` |

Use base-language catalogs for this first implementation. Portuguese regional vocabulary needs a conscious translation choice: document the chosen register and have a fluent reviewer check both Brazilian and European comprehension. Do not label one regional translation as two independently reviewed variants. Separate regional catalogs can follow a concrete request.

Keep `C`, `POSIX`, empty, invalid and unsupported locale values falling back to English. Keep existing Chinese behavior unchanged. The installed x/text package already handles tag parsing; no new locale parser is needed.

If improving environment selection, use explicit `--lang` before `LC_ALL`, `LC_MESSAGES`, then `LANG`, and use the same rule for install and uninstall. Treat this as a small, separately checked change. Existing source currently reads only LANG on install; do not document richer precedence until implementing it.

## Translation rules

- Translate complete user-facing sentences in context, including refusal, failure and recovery guidance.
- Preserve JSON keys, command names, paths, package names, URLs, flags and config values. Keep `sysc-shell`, `niri-session`, Debian, Fedora and the docs URL recognizable where existing checks require them.
- Preserve formatting arguments. Four English keys currently contain `%s`: `conflicts.dbus_failed`, `warn.conflict_kept`, `warn.conflict_failed`, `confirm.system`. Recheck the source catalog. Do not turn a literal percent into a printf argument.
- Preserve newline intent and keyboard symbols; use valid UTF-8 JSON.
- Keep concise UI labels, especially navigation and the advice panel. Do not shrink the global font, add screens or expand the wizard to fit a translation.
- Avoid flag icons as language selectors. Language and nationality are different choices.
- Maintain the current guidance: shell companions install together; plugins come from the shell catalog; full Debian/Fedora support remains planned.
- Identify translations requiring fluent review. Machine-generated text alone does not establish translation quality.

## Implementation sequence

1. Inspect current main and catalog changes since the baseline. Check workspace status.
2. Add five Locale constants, append them to `order`, and extend `Match` using the existing switch.
3. Add five complete JSON catalogs with the exact English key set. Keep translation work separate from code behavior changes.
4. Extend existing i18n checks for matching, complete nonempty catalogs, formatting arguments and cycle membership. Prefer a table-driven check in the existing file.
5. Update both `--lang` help strings and language documentation. Check F9 on wallpaper/weather inputs and confirmation, including skipped optional pages.
6. Inspect existing hardcoded user-facing messages; localize necessary guidance with existing catalog keys/patterns and record any deliberate remaining English diagnostics.
7. Review compact and tall terminal layouts. Check Japanese/Korean cell widths and Russian/Portuguese wrapping, not byte lengths.
8. Report completion, checks and fluent-review gaps. Commit/push only with authorization applicable to that future task. A release needs its own binary build/checksum process.

## Small verification plan

Start with the affected packages, not new frameworks or a matrix of duplicated tests:

```sh
env SHELL=/bin/sh GOMAXPROCS=2 go test -p 1 ./internal/i18n ./internal/ui ./cmd/sysc
gofmt -l cmd internal
git diff --check
```

Use the existing race suite once if locale state or UI behavior changes. If website content changes, run its existing content check; run the full docs gate before publishing a site change when practical. The gate is `DOCS_BASE_PATH=/sysc npm run check` from `docs-site`.

In this environment the Go cache and fixture sockets needed sandbox approval, and the Next.js TypeScript subprocess returned empty output under the sandbox; the approved runs passed. Request the normal tool escalation if these restrictions recur. Keep local output concise.

Manually inspect existing terminal sizes around 80x24 and a larger window with every new language. Check advice wrapping, visible actions, F9 answer preservation, no missing glyphs, and navigation back across the skipped plugin page. The user's terminal font controls CJK glyph availability; Go's font preflight concerns the installed graphical shell, not the terminal renderer.

## Acceptance conditions

- Nine installer locales load; the five requested languages match common environment and CLI spellings.
- Existing languages and English fallback keep working; catalog keys and printf arguments agree.
- F9 visits each supported locale once per cycle and keeps user input, selections and page state.
- Translation changes preserve the black UI, charcoal advice panel, controls and install/uninstall behavior.
- Language help/docs describe source and released binaries accurately.
- The final handoff lists checks performed, unresolved translation review needs and any English-only user surfaces.
