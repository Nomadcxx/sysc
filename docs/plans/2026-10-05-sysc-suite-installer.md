# SYSC Suite Installer Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Ship a guided TUI in `/home/nomadx/sysc` that downloads a pinned SYSC desktop (shell, clipboard, terminal, walls; gSlapper if missing) into a Niri user session, with greet-family chrome and en/zh-Hans/de/fr.

**Architecture:** One static `cmd/sysc` binary. Domain packages (`pin`, `distro`, `fetch`, `stamp`, `i18n`, `seed`) have no Bubble Tea. The TUI collects answers then calls those packages. Download every asset and verify SHA256 before any rename; restore `.bak` on mid-swap failure; stamp only after units are up.

**Tech Stack:** Go 1.26, charmbracelet bubbletea + bubbles + lipgloss, `golang.org/x/text/language` for LANG matching. No CGO. Tests: `timeout 90s env GOMAXPROCS=2 go test -count=1 <pkg> -run <Name>` only. Never `go test ./...` or `-race`.

**Design:** `/home/nomadx/sysc-shell/docs/plans/2026-10-05-sysc-suite-installer-design.md` (copy in this repo under `docs/plans/`, gitignored).

**Workdir:** `/home/nomadx/sysc`. Commit there, not in sysc-shell. Commit messages: conventional, no `agent`/`cursor`/`llm`/`both`. `gofmt -w` on touched Go files before each commit.

---

### Task 1: Module and pin file

**Files:**
- Create: `go.mod`
- Create: `internal/pin/pin.go`
- Create: `internal/pin/pin_test.go`
- Create: `internal/pin/testdata/ok.json`
- Create: `internal/pin/testdata/missing-sha.json`

**Step 1: Write the failing test**

`internal/pin/pin_test.go`:

- `TestDecodeRequiresAssetsAndSHA` loads `testdata/ok.json` (shell, clipboard, terminal, walls with amd64+arm64 URLs and SHA256; gslapper arch slot; lock `disabled`; recommended plugin ids) and asserts `Decode` returns them.
- Load `missing-sha.json` and assert error.

**Step 2: Run test to verify it fails**

Run: `timeout 90s env GOMAXPROCS=2 go test -count=1 ./internal/pin -run TestDecodeRequiresAssetsAndSHA`

Expected: FAIL (package or Decode missing).

**Step 3: Write minimal implementation**

`go mod init github.com/Nomadcxx/sysc` with `go 1.26`. `Pin` struct + `Decode([]byte) (Pin, error)` that rejects missing SHA, missing arch asset, enabled lock, or empty recommended list. `ok.json` uses placeholder URLs/hashes.

**Step 4: Run test to verify it passes**

Run: `timeout 90s env GOMAXPROCS=2 go test -count=1 ./internal/pin -run TestDecodeRequiresAssetsAndSHA`

Expected: PASS.

**Step 5: Commit**

```bash
gofmt -w internal/pin/pin.go internal/pin/pin_test.go
git add go.mod internal/pin/pin.go internal/pin/pin_test.go internal/pin/testdata/ok.json internal/pin/testdata/missing-sha.json
git commit -m "$(cat <<'EOF'
feat: decode the suite pin file

EOF
)"
```

---

### Task 2: Distro gate

**Files:**
- Create: `internal/distro/distro.go`
- Create: `internal/distro/distro_test.go`

**Step 1: Write the failing test**

Table: ID `arch` / `cachyos` / `manjaro` / `endeavouros` → proceed; `fedora` / `debian` / `ubuntu` → named refusal containing the ID; empty ID → unknown refusal.

**Step 2: Run**

`timeout 90s env GOMAXPROCS=2 go test -count=1 ./internal/distro -run TestFamilyGate`

Expected: FAIL.

**Step 3: Implement**

`Family(id string) error` using a closed Arch-family set. Do not exec pacman here; ID comes from `/etc/os-release` ID/ID_LIKE in a later wiring test with a fake file.

**Step 4: Pass** the same `-run TestFamilyGate`.

**Step 5: Commit** `feat: refuse non-Arch distros by name`

Add `TestParseOSRelease` with testdata snippets (ID + ID_LIKE=arch) in the same commit if still small; otherwise next micro-commit.

---

### Task 3: Catalogues (en, zh-Hans, de, fr)

**Files:**
- Create: `internal/i18n/i18n.go`
- Create: `internal/i18n/i18n_test.go`
- Create: `internal/i18n/catalog/en.json`
- Create: `internal/i18n/catalog/zh-Hans.json`
- Create: `internal/i18n/catalog/de.json`
- Create: `internal/i18n/catalog/fr.json`

**Step 1: Failing tests**

- `TestEveryKeyExistsInAllCatalogs` — union of keys; each locale has every key, non-empty.
- `TestMatchLANG` — `zh_CN.UTF-8` → `zh-Hans`; `de_DE` → `de`; `fr_FR` → `fr`; `en_US` and `ja_JP` and empty → `en`.

v1 keys at least: `nav.next`, `nav.back`, `nav.quit`, `nav.language`, `nav.wait`, `weather.title`, `weather.blurb`, `theme.title`, `wallpaper.title`, `plugins.title`, `refuse.distro`, `refuse.root`, `refuse.niri`, `refuse.weather`.

**Step 2: Run** `... ./internal/i18n -run 'TestEveryKeyExistsInAllCatalogs|TestMatchLANG'` — FAIL.

**Step 3: Implement** embed JSON via `go:embed`, `Match(lang string) Locale`, `T(locale, key) string`. Cycle order `en → zh-Hans → de → fr → en`.

**Step 4: PASS** those tests.

**Step 5: Commit** `feat: installer catalogues for four locales`

---

### Task 4: Fetch, checksum, stage, rollback

**Files:**
- Create: `internal/fetch/fetch.go`
- Create: `internal/fetch/fetch_test.go`

**Step 1: Failing tests**

- `TestChecksumMismatchDoesNotWrite` — httptest server, wrong SHA, dest dir empty.
- `TestSwapRestoresBakOnFailure` — two components; second rename fails (inject); first dest restored from `.bak`, no `.new` left.

**Step 2: Run** `... ./internal/fetch -run 'TestChecksumMismatchDoesNotWrite|TestSwapRestoresBakOnFailure'` — FAIL.

**Step 3: Implement**

`DownloadAll` to a staging dir; SHA256; `Swap(binDir, name, staged)` writes `name.new`, keeps `name.bak`, rename. `Rollback(binDir, names)` copies bak back. Never overwrite a running file in place: remove dest then rename, or rename over after bak.

**Step 4: PASS.**

**Step 5: Commit** `feat: verify assets then swap with bak rollback`

---

### Task 5: Stamp

**Files:**
- Create: `internal/stamp/stamp.go`
- Create: `internal/stamp/stamp_test.go`

**Step 1: Failing test**

`TestWriteStampOnlyWhenUnitsUp` — fake `unitsUp` false → no file; true → `installed.json` names release, component versions, `GSlapperInstalled bool`.

**Step 2: Run** `... ./internal/stamp -run TestWriteStampOnlyWhenUnitsUp` — FAIL.

**Step 3: Implement** `Read`/`Write` under a given state dir (tests pass a temp dir). Caller supplies “units up”.

**Step 4: PASS.**

**Step 5: Commit** `feat: write the suite stamp after units are up`

---

### Task 6: Shell config seed (theme, weather, plugins, bar)

**Files:**
- Create: `internal/seed/seed.go`
- Create: `internal/seed/seed_test.go`

Do **not** import sysc-shell as a module in v1 unless a tiny JSON round-trip is easier; hand-built JSON must still load as the shell’s `Default()` overlay: theme preset, ThemeGen wallpaper, weather lat/lon/location, plugins.enabled, bar.items including a `weather` widget.

**Step 1: Failing test**

`TestSeedAddsWeatherToDefaultBar` — given answers (standard, dark, stills dir, plugin ids, place), output JSON has `weather.latitude`, `weather.location`, and a bar item `"id":"weather"`. `TestSeedRejectsEmptyWeather` — no coordinates → error.

**Step 2: Run** `... ./internal/seed -run 'TestSeedAddsWeatherToDefaultBar|TestSeedRejectsEmptyWeather'` — FAIL.

**Step 3: Implement** merge onto a baked copy of shell `Default()` JSON (paste the default bar structure from sysc-shell `internal/config/config.go` Default(); keep weather off the baked default, add it only after coordinates).

**Step 4: PASS.**

**Step 5: Commit** `feat: seed shell config with weather on the bar`

---

### Task 7: Install pipeline (no TUI)

**Files:**
- Create: `internal/install/install.go`
- Create: `internal/install/install_test.go`

**Step 1: Failing tests**

- `TestYesNeedsWeather` — `--yes` without city/lat/guess → error.
- `TestSkipGSlapperWhenOnPATH` — fake PATH with `gslapper`; installer does not call the package hook.
- `TestUninstallKeepsConfig` vs `TestUninstallPurge` — stamp present; keep leaves XDG file; purge removes it; `gslapper` binary not removed unless stamp.GSlapperInstalled.

Use temp HOME, fake PATH, stub fetch.

**Step 2: Run** the named tests — FAIL.

**Step 3: Implement** `Run(opts)` building the task list from answers: fetch SYSC-owned, skip-or-install gSlapper, write units under systemd/user, seed config, start (stubable), stamp. Uninstall reads stamp.

**Step 4: PASS.**

**Step 5: Commit** `feat: run install and uninstall from answers`

---

### Task 7b: Niri sidecar, units, backups

**Files:**
- Create: `internal/niri/niri.go`
- Create: `internal/niri/niri_test.go`
- Create: `internal/backup/backup.go`
- Create: `internal/backup/backup_test.go`
- Create: `internal/units/units.go`
- Create: `internal/units/units_test.go`

**Step 1: Failing tests**

- `TestIncludeIdempotent` — two applies → one `include "sysc.kdl"`; `sysc-shell.kdl` include untouched.
- `TestCommentShellSpawnOnly` — `spawn-at-startup "sysc-shell"` commented; cliphist/polkit lines unchanged.
- `TestSkipOccupiedBind` — existing `Mod+Comma` consume-window → sidecar does not bind `Mod+Comma`.
- `TestFirstBakNotOverwritten` — mutate twice; `config.kdl.sysc.bak` bytes still match the original; a second timestamped copy appears under a fake state dir.
- `TestSeedDoesNotClobberExistingShellConfig` — existing `config.json` left intact.
- `TestStopOrderShellFirst` — fake systemctl records stop of shell before walls/clipboard.

**Step 2: Run** those tests — FAIL.

**Step 3: Implement** backup helper (`FirstBak` + timestamped state copies, cap five timestamps, never delete `*.sysc.bak`). Niri merge as in the design. Units: `WantedBy=graphical-session.target`; start only if graphical-session is active (injectable).

**Step 4: PASS.**

**Step 5: Commit** `feat: niri sidecar, user units and first-run config backups`

---

### Task 8: TUI chrome (banner, beams, nav)

**Files:**
- Create: `internal/ui/banner.go` (asciiHeaderLines copied from greet installer)
- Create: `internal/ui/beams.go` (BeamsTextEffect, monochrome, 50ms tick — port from `sysc-greet/cmd/installer/animations.go`, keep it in this module)
- Create: `internal/ui/chrome.go`
- Create: `internal/ui/chrome_test.go`
- Create: `cmd/sysc/main.go`

**Step 1: Failing tests**

- `TestBannerContainsCowboyLine` — joined header contains `SEE YOU IN SPACE COWBOY`.
- `TestNavFollowsLocale` — chrome help for `en` contains `Language`; for `zh-Hans` does not use the English word `Quit` (uses the zh catalog value).
- `TestNavWaitHasNoCancel` — installing step help is the wait key, not Esc/Quit cancel.

**Step 2: Run** `... ./internal/ui -run 'TestBannerContainsCowboyLine|TestNavFollowsLocale|TestNavWaitHasNoCancel'` — FAIL.

**Step 3: Implement** View layout: beams, title, rounded body, bottom nav. Lipgloss FG+BG on every style. 80×24 minimum. `cmd/sysc` wires tea.NewProgram alt-screen; `--yes` skips tea.

**Step 4: PASS.**

**Step 5: Commit** `feat: greet-family banner, beams and bottom nav`

---

### Task 9: Wizard screens

**Files:**
- Create: `internal/ui/wizard.go`
- Create: `internal/ui/wizard_test.go`

**Step 1: Failing tests**

- `TestWizardOrder` — theme → wallpaper → plugins → weather → confirm.
- `TestWeatherBlocksConfirm` — empty location, Enter on weather does not advance.
- `TestF9CyclesLocale` — locale changes, copy keys resolve.

**Step 2: Run** those tests — FAIL.

**Step 3: Implement** collect-then-execute; page blurbs from catalogs; F9 cycle; Esc back except during install.

**Step 4: PASS.**

**Step 5: Commit** `feat: collect theme wallpaper plugins and weather`

---

### Task 10: Preflight, flags, fetcher script

**Files:**
- Modify: `cmd/sysc/main.go`
- Create: `cmd/sysc/main_test.go` (flag parse only, if testable without tea)
- Create: `internal/preflight/preflight.go`
- Create: `internal/preflight/preflight_test.go`
- Create: `install.sh`

**Step 1: Failing tests**

- `TestPreflightRefusesRoot` (geteuid fake via param).
- `TestInstallShPicksAmd64` — script test with stub uname, asserts URL suffix `sysc-linux-amd64` (see sysc-Go unmerged install_sh_test pattern: run bash with mocked curl).

**Step 2: FAIL those tests.**

**Step 3: Implement** `--yes`, `--city`, `--lat`, `--lon`, `--lang`. `install.sh` detects arch, downloads release asset, sha256sum -c, exec. Non-TTY: exec `sysc --yes`. Comment in README: do not curl until first tag.

**Step 4: PASS.**

**Step 5: Commit** `feat: preflight flags and release fetcher script`

---

### Task 11: Release workflow (binary only)

**Files:**
- Create: `.github/workflows/release.yml`

On tag `v*`: build `sysc` CGO_ENABLED=0 linux amd64 and arm64, `sha256sum` to `SHA256SUMS`, upload `sysc-linux-amd64`, `sysc-linux-arm64`, pin JSON if not fully embedded. Do not `CGO_ENABLED=0` any walls daemon here — SYSC only ships this installer; component releases stay in their repos.

No test beyond `go build`. Commit `ci: attach installer binaries on version tags`

---

### Task 12: README one-liner (still “not ready” until first tag)

**Files:**
- Modify: `README.md`

Document the intended curl, `--yes`, languages, Arch-family v1, and “do not curl until a release exists.” Commit `docs: describe the public install path`

---

## Notes for the implementer

- Beams source of truth: `/home/nomadx/sysc-greet/cmd/installer/animations.go` and `asciiHeaderLines` in that installer `main.go`. Copy, do not import greet.
- Shell Default() bar: `/home/nomadx/sysc-shell/internal/config/config.go` `Default()`. Weather validation: `requireWeatherWhenUsed` in `load.go`.
- gSlapper package names: fill Arch slot when cutting the first pin; other distro slots stay empty until expansion.
- First pin URLs can be placeholders until those component repos have matching GitHub release assets; fetch tests use httptest, not the network.
- Owner `scripts/deploy` stays out of this binary.
