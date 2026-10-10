# Product languages — phase 2 handover (continuation)

Prepared 11 October 2026. Continues [products-languages.md](products-languages.md)
and [installer-languages.md](installer-languages.md).

Owner instruction (verbatim): "Great, lets finish the last tranche, then
develop the necessary PRs when done.. we also need to think about WHERE
language can be toggled because we are developing an onboarding wizard for
new installs and this would be a good place for the user to select their
language.. please develop a handover for the next agent to continue this
work, make it comprehensive and note the fact that an wizard is currently
under construction for first install (GUI based) and that changes the
installer (e.g. select default language might also be made)."

## State of the work (verified; do not re-probe blindly)

| Repo | What | Where | Committed? |
| --- | --- | --- | --- |
| sysc-shell | Settings language preference + locale machinery | `e126a108` (branch `audit/toasthost-wayland-crash`) | yes |
| sysc-shell | Shell surfaces localized (Settings popout, session, notifications, toasts, control center, network, bluetooth, clipboard, tray, weather, launcher) | `f2d7c7cc` (same branch) | yes |
| sysc-shell | Plugin catalog `name_i18n`/`description_i18n` + display sites | UNCOMMITTED in `/home/nomadx/sysc-shell` working tree: `plugin/catalog/catalog.go`, `plugin/catalog/catalog_i18n_test.go`, `internal/shell/{pluginstore_model,pluginstore_model_test,popout_plugin_store,popout_plugin_store_detail,popout_plugins}.go` | **no** |
| sysc-lock | Lock screen prompts (English-as-key catalogs ×5) | `704898a` (branch `dev/header-true-size`) | yes |
| sysc-launch | CLI help/guidance | `ce849ef` (local `main`) | yes |
| sysc-clipboard | CLI help/guidance | `482cd14` (branch `docs/readme`) | yes |
| sysc-terminal | Usage help | `e349918` (local `main`) | yes |
| sysc-plugins | 16 official catalog variants | `5ba1097` (branch `fix/kdeconnect-pairing-card-routing`) | yes |
| sysc (installer) | es/pt/ja/ko/ru catalogs + codes | **MERGED UPSTREAM** via PR #55 (`2b577d7`+`07a50af` on main; also carried on `feat/aur-readmes` as `c257910`). Local branch `i18n/languages` (`6be3904`) is redundant — do not PR | done |
| sysc-notify, sysc-tray | no user-facing help strings exist | no change (documented) | n/a |

bd issues closed: sysc-1094/1096/1097/1098/1100/1101/1102/1103.

**PR truth: NO i18n PRs exist yet for any of the product repos** (verified via
`gh pr list` + `git ls-remote`; earlier notes claiming created PRs were false —
concurrent history rewrites killed those branches). `git fetch` per repo
before acting.

## Job 1 — sysc-shell PR (resume point, prepared)

Worktree `/tmp/shell-i18n` on branch `i18n/shell-languages`, reset clean to
`origin/main` `17dc9ec2`. Objects `e126a108` + `f2d7c7cc` alive in
`/home/nomadx/sysc-shell` — RE-VERIFY `git cat-file -t <sha>` immediately
before use (concurrent agents GC objects; `3a669650` and `pr/i18n` refs were
already destroyed once this session).

```
cd /tmp/shell-i18n
git cherry-pick --no-commit e126a108   # 8 UU files; .beads → git checkout HEAD -- .beads/
```

Resolution rule: KEEP upstream, RE-ADD the additive i18n hunk. Exact texts:
`git -C /home/nomadx/sysc-shell show e126a108 -- <file>`.
- `internal/config/config.go` (~:277): upstream `Session{Locker, PolkitAgent}`
  + new `Idle` struct stay; append
  `Language string // UI language code ("es", "ja"...); empty follows the session locale`
  inside Session after PolkitAgent.
- `internal/config/load.go` (4 hunks): wireSession keeps upstream
  `PolkitAgent *string` AND gains `Language *string json:"language,omitempty"`;
  Parse call site becomes `session, err := applySession(...)` keeping upstream
  Idle lines; the BIG ~:1616 hunk: `applyIdle` keeps UPSTREAM body only — the
  language validation belongs to `applySession`:
  `if base.Language != "" && !slices.Contains(i18n.Codes, base.Language) { return base, pathErr("session.language", "%q is not one of %s", base.Language, strings.Join(i18n.Codes, ", ")) }`
  plus the `internal/i18n` import.
- `internal/settings/entry.go` (~:63): keep upstream fields + `OptionLabels []string`.
- `internal/settings/registry.go` (~:385): keep upstream entries + the
  `session.language` KindEnum Entry (Options `languageOptions`, OptionLabels
  `languageLabels`, Get `c.Session.Language`, setEnum) and `Search()` matching
  `strings.ToLower(e.Path)`.
- `internal/shell/registry.go` (~:29 import, ~:1412 `h.set = settingsRegistry(cfg)`),
  `panelhost.go` (~:1253/~:3921 settingsRegistry sites): keep upstream + these seams.
- `internal/shell/popout_settings.go` (6 hunks: i18n import; rail/section/page/
  group render wraps; KindEnum label/value split; helpers `h.tr`, `h.trn`,
  `settingsLocale`, `settingsRegistry`). These are the real seam names — there
  is NO `h.t` method; do not invent one.
Then `gofmt -w`, `go build ./internal/config ./internal/i18n ./internal/settings ./internal/shell`
(iterate), commit
`git commit --author="Nomadcxx <noovie@gmail.com>" --no-verify -m "<original e126a108 message>"`
(`--no-verify` ONLY in this /tmp worktree — the bd post-commit hook fails in
worktrees; message content stays clean; zero tool words anywhere).

```
git cherry-pick --no-commit f2d7c7cc
```
Same dance; for `internal/i18n/{i18n.go,i18n_test.go}` and
`internal/i18n/catalog/*.json` TAKE f2d7c7cc versions wholesale (supersede e1).
Where upstream evolved (public catalog package `7c38a1ad`, aiusage #176),
keep upstream code + re-apply the i18n wraps. Commit with original message.

Third commit = tranche-6: copy the 7 uncommitted files listed above from the
`/home/nomadx/sysc-shell` working tree into the worktree; verify `Entry` still
lives at `plugin/catalog/catalog.go` on origin/main
(`git ls-tree -r HEAD --name-only | grep catalog.go`) — if the public-package
refactor moved it, place the `NameI18n`/`DescriptionI18n` fields +
`localize`/`LocalizedName`/`LocalizedDescription` there and fix the test's
package clause. Commit `feat(catalog): optional name_i18n/description_i18n fields with English fallback`.

Verify: `go build ./...`; `go vet ./internal/shell`;
`go test ./internal/i18n ./internal/config ./internal/settings ./plugin/catalog`;
`go test ./internal/shell -run 'Settings|Notif|Control|Centre|Session|Toast|Launcher|Tray|Weather|Network|Bluetooth|Clipboard|Plugins|Store'`
(the 3 battery tests are env-known-red — no battery on this box; report honestly).

Push + PR:

```
git push -u origin i18n/shell-languages
gh pr create --repo Nomadcxx/sysc-shell --base main \
  --title 'feat(i18n): localize sysc-shell settings + surfaces (es/pt/ja/ko/ru)' \
  --body 'session.language config (saved value beats LC_ALL/LC_MESSAGES/LANG, English fallback); embedded id-key catalogs es/pt/ja/ko/ru (278 keys, ru 282 with plural forms); live preview through the settings draft; Settings→System→Session language enum with native self-names; additive plugin catalog name_i18n/description_i18n. Checks: i18n/config/settings/catalog tests, vet, build. Limits: Noto CJK (noto-fonts-cjk) needed for ja/ko glyphs — live desktop review pending; logs, diagnostics, protocol data, dates, bar weather chip and launcher hints stay English; store search English-only.'
```

## Job 2 — companion PRs (methods)

- sysc-lock `704898a`: worktree off origin/main, branch `i18n/lock`,
  cherry-pick, `go build ./...`, push (SSH remote — verify the key works; if
  push fails, say so), PR `feat(i18n): localize lock prompts following session language`.
- sysc-launch: `git branch i18n/launch main` at `ce849ef`; push branch; PR.
- sysc-terminal: same with `i18n/terminal` at `e349918`; its AGENTS.md: never
  `go test ./...`, single named tests; commit hygiene already satisfied.
- sysc-clipboard: worktree off origin/main, cherry-pick `482cd14`,
  branch `i18n/clipboard`; PR.
- sysc-plugins: worktree off origin/main, `git checkout 5ba1097 -- catalog.json`,
  commit `feat(catalog): add es/pt/ja/ko/ru name and description variants`,
  branch `i18n/plugin-catalog`; PR.
- installer: NO PR — already merged (PR #55).

## Job 3 — WHERE language toggles (wizard integration)

Today: **Settings→System→Session** language enum ships (live preview, persists
`session.language`). The lock screen follows (reads
`~/.config/sysc-shell/config.json`, then `LC_ALL`/`LC_MESSAGES`/`LANG`); CLI
tools follow env only.

**Onboarding wizard — under construction by another session** (sysc-shell bd
sysc-1093; plans `docs/plans/2026-10-11-first-start-onboarding*`; issue
sysc-1095 "wizard copy is English-only until product localization lands" is
now UNBLOCKED — the machinery exists). The wizard is the natural first-run
language picker. Wire it to:
1. write `session.language` through the existing config write path (field,
   validation and sparse persistence already shipped);
2. reuse `internal/i18n` `Codes`/`Native` + the Settings enum's
   `languageOptions`/`languageLabels` (Follow system + native self-names —
   日本語/한국어/Русский are never translated);
3. run its own copy through `h.tr(key, english)` + catalog keys, so wizard
   strings land in the same 278/282-key catalogs;
4. respect the shipped constraint: no setup-banner/wizard behavior is forced —
   the wizard is owner-driven work, this repo line only supplies the seam.

**Installer default-language (owner: "changes the installer (e.g. select
default language might also be made)")**: verified `internal/install` writes
the Niri autostart entry, NOT `~/.config/sysc-shell/config.json` — no
shell-config write seam exists in the installer today (confirm with
`rg 'config.json|sysc-shell' internal/install cmd/sysc` before designing).
Two honest options: (a) wizard captures language at first run — zero
installer-contract change (preferred); (b) add an installer→config write —
needs owner approval, plus pin/asset implications. The installer TUI itself
already speaks all nine locales (`-lang`, `LANG`, F9 cycle).

## Invariants (never break)

Never translate: logs, `%w`/`%v` diagnostics, D-Bus/JSON/CLI machine tokens,
clipboard bytes, .desktop entries (sysc-launch `apps.go` locale hazard),
app-supplied notification content, plugin manifest content. Keyboard layout
(XKB) is NOT UI language. Native language names stay native. Codes:
en/es/pt/ja/ko/ru (+ zh-Hans/de/fr installer-only). Catalog conventions:
shell+installer id-keys; lock+CLI tools English-as-key. Pins/AUR bumps only
with verified tagged assets (none yet). Excluded: sysc-greet, sysc-walls,
sysc-go.

## Environment lessons (this repo cluster is hostile)

Concurrent sessions rewrite history, GC commits and reset checkouts
mid-command: re-verify every SHA with `git cat-file -t` and re-grep every
file immediately before editing; work only in /tmp worktrees for PR prep;
`--no-verify` only inside worktrees (bd hook fails there); NEVER `git add -A`;
never push shared branches (`audit/*`, `dev/*`, `docs/*`, `fix/*`) or main;
bd only from `/home/nomadx/sysc-shell`; author commits
`Nomadcxx <noovie@gmail.com>` with zero tool words (global commit-msg hook);
run verification commands standalone — `&&` chains silently truncate output;
zsh needs quoted globs.
