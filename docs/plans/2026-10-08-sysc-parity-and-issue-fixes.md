# SYSC Issue Parity and Fixes Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix all 11 open issues (#18–#28) in three reviewable PRs: full installer-parity UI (#28, closing #18 #19 #20 #25 #26 #27), wizard page honesty (#21 #22), and uninstall CLI correctness (#23 #24).

**Architecture:** PR1 makes the TUI chrome match sysc-greet: banner-sized beams, monochrome palette, height-filled layout, page-accurate footers, and an in-TUI install flow driven by a new `install.Options.Progress` callback over a goroutine + `p.Send` messages. PR2 adds the reachable dark/light mode toggle and fixes wallpaper copy + README. PR3 gives uninstall a real confirmation prompt and a non-zero exit when a task fails.

**Tech Stack:** Go 1.26, bubbletea v1.3, bubbles (textinput), lipgloss v1. No new dependencies. No teatest — snapshot tests use direct `View()` string assertions.

**Repo:** `/home/nomadx/sysc`, base `main` @ `861fa9b`. Remote `https://github.com/Nomadcxx/sysc.git`.

**Gates (every PR):** `gofmt -l .` empty, `go vet ./...`, `go test -race -count=1 ./...`, `go build ./...`.

**Merge style:** `gh pr merge <n> --merge --delete-branch` (repo history uses merge commits).

**Commit identity:** Nomadcxx <noovie@gmail.com>. Use `git -c user.name=Nomadcxx -c user.email=noovie@gmail.com commit` if config differs.

**Do NOT touch (untracked sysc-lock artifacts):** `issue-21-spec.md`, `docs/plans/2026-10-06-sysc-lock-guided-installer.md`.

---

## PR1 — `fix/parity-installer-ui` (closes #18 #19 #20 #25 #26 #27)

### Task 1: Banner-sized beams canvas

**Files:**
- Modify: `internal/ui/banner.go`
- Modify: `cmd/sysc/main.go`
- Test: `internal/ui/banner_test.go` (create if missing)

**Step 1: Write the failing test** — `internal/ui/banner_test.go`

```go
package ui

import "testing"

func TestBannerHeight(t *testing.T) {
	if got := BannerHeight(); got != len(asciiHeaderLines)+2 {
		t.Fatalf("BannerHeight() = %d, want %d", got, len(asciiHeaderLines)+2)
	}
	if got := BannerHeight(); got != 11 {
		t.Fatalf("BannerHeight() = %d, want 11 for the current banner", got)
	}
}
```

**Step 2: Run and watch it fail** — `go test ./internal/ui/ -run TestBannerHeight` → undefined: BannerHeight.

**Step 3: Implement** — add to `internal/ui/banner.go`:

```go
// BannerHeight is the canvas height for the banner animation: the eight
// text lines, the cowboy underline, and one blank row above and below.
func BannerHeight() int { return len(asciiHeaderLines) + 2 }
```

In `cmd/sysc/main.go` `newModel`:

```go
m.beams = ui.NewBeamsTextEffect(m.width, ui.BannerHeight(), ui.Banner())
```

and in the `tea.WindowSizeMsg` case:

```go
if m.beams != nil {
	m.beams.Resize(msg.Width, ui.BannerHeight())
}
```

**Step 4: Verify** — `go test ./internal/ui/ -run TestBannerHeight` PASS; `go build ./...`.

**Step 5: Commit** — `fix(ui): size the beams canvas to the banner, not the terminal` (files: banner.go, banner_test.go if new, main.go).

---

### Task 2: Greet-parity chrome layout, palette, page-accurate footer

Closes #18, #26, #27 (nav part).

**Files:**
- Modify: `internal/ui/chrome.go`
- Modify: `cmd/sysc/main.go` (`View()` call site)
- Modify: `internal/i18n/catalog/{en,zh-Hans,de,fr}.json`
- Test: `internal/ui/chrome_test.go`

**Design:**
- Palette (greet monochrome): base bg `#1a1a1a`, fg `#ffffff`; box border `#ffffff`; muted `#666666`. Replace `#0a0a0a`/`#999999`/`#666666` border values in the style vars. Add a short comment naming the palette.
- `Nav` becomes page-aware: `func Nav(loc i18n.Locale, page Page, step Step) string`.
  - `StepInstalling` → `nav.wait` only (unchanged).
  - `StepDone`, `StepFailed` → `Enter <nav.close>`.
  - `StepWizard`:
    - `PageTheme`: `↑↓ <nav.preset>`
    - `PagePlugins`: `↑↓ <nav.toggle>`
    - `PageWeather`: `<nav.type>` first (q types into the input), then `Enter <nav.search>`
    - others: `Enter <nav.next>`
    - always `Esc <nav.back>` • `F9 <nav.language>`
    - `PageWeather` quit hint: `Ctrl+C <nav.quit>` (no dead `q Quit` — #27)
    - all other wizard pages: `q <nav.quit>`
  - Join with ` • `, render muted.
- `View` signature gains `page Page` before `step`: `View(loc, title, body string, page Page, step Step, width, height int, beams *BeamsTextEffect) string`.
- Layout: build `parts` (beams render if non-nil, title, box `boxStyle.Width(width-8)`), `content := lipgloss.JoinVertical(lipgloss.Center, parts...)`, split lines, pad with empty lines until `len(lines) == height-1`, append `Nav(...)` as the final line, then `baseStyle.Render(strings.Join(lines, "\n"))`. Result: top-aligned, exactly `height` lines, nav pinned bottom, never overflows when the beams canvas is banner-sized.

**New i18n keys (add to ALL FOUR catalogs with translations; `TestEveryKeyExistsInAllCatalogs` enforces parity):**

| key | en | de | fr | zh-Hans |
|---|---|---|---|---|
| `nav.preset` | Preset | Voreinstellung | Préréglage | 预设 |
| `nav.toggle` | Toggle | Umschalten | Basculer | 切换 |
| `nav.search` | Search | Suchen | Rechercher | 搜索 |
| `nav.type` | Type a city | Stadt eingeben | Saisir une ville | 输入城市 |
| `nav.install` | Install | Installieren | Installer | 安装 |
| `nav.close` | Close | Schließen | Fermer | 关闭 |

**Step 1: Write failing tests** — update `internal/ui/chrome_test.go`:

```go
func TestNavFollowsLocale(t *testing.T) {
	en := Nav(i18n.EN, PageTheme, StepWizard)
	if !strings.Contains(en, "Language") {
		t.Fatalf("en nav = %q", en)
	}
	zh := Nav(i18n.ZH, PageTheme, StepWizard)
	if strings.Contains(zh, "Quit") {
		t.Fatalf("zh nav still uses the English word Quit: %q", zh)
	}
	if !strings.Contains(zh, i18n.T(i18n.ZH, "nav.quit")) {
		t.Fatalf("zh nav missing the zh quit label: %q", zh)
	}
}

func TestNavPageAccurate(t *testing.T) {
	theme := Nav(i18n.EN, PageTheme, StepWizard)
	if !strings.Contains(theme, "Preset") {
		t.Fatalf("theme nav = %q", theme)
	}
	wall := Nav(i18n.EN, PageWallpaper, StepWizard)
	if strings.Contains(wall, "↑↓") {
		t.Fatalf("wallpaper nav advertises dead arrows: %q", wall)
	}
	weather := Nav(i18n.EN, PageWeather, StepWizard)
	if !strings.Contains(weather, "Ctrl+C") {
		t.Fatalf("weather nav must advertise the real quit key: %q", weather)
	}
	if strings.Contains(weather, "q Quit") {
		t.Fatalf("weather nav still advertises q Quit: %q", weather)
	}
	confirm := Nav(i18n.EN, PageConfirm, StepWizard)
	if !strings.Contains(confirm, "Install") {
		t.Fatalf("confirm nav = %q", confirm)
	}
	done := Nav(i18n.EN, PageTheme, StepDone)
	if !strings.Contains(done, i18n.T(i18n.EN, "nav.close")) {
		t.Fatalf("done nav = %q", done)
	}
}
```

Keep `TestNavWaitHasNoCancel` (update the call to `Nav(i18n.EN, PageTheme, StepInstalling)`).

**Step 2: Run and watch them fail** — `go test ./internal/ui/`.

**Step 3: Implement chrome changes + catalogs + main call site** (`ui.View(m.w.Locale, m.w.Title(), body, m.w.Page, ui.StepWizard, m.width, m.height, m.beams)`).

**Step 4: Verify** — `go test ./internal/ui/ ./internal/i18n/`.

**Step 5: Commit** — `fix(ui): banner-height chrome, greet palette, page-accurate footers`.

---

### Task 3: Confirm summary, plugins none-state, stale copy

Closes #20, #25.

**Files:**
- Modify: `internal/ui/wizard.go`
- Modify: `internal/i18n/catalog/{en,zh-Hans,de,fr}.json`
- Test: `internal/ui/wizard_test.go`

**Design:**
- New key `plugins.blurb` (all catalogs): en `"Recommended plugins are on by default; toggle them off for a bare bar."` de `"Empfohlene Plugins sind standardmäßig aktiv; schalte sie für eine reine Leiste aus."` fr `"Les plugins recommandés sont activés par défaut ; désactivez-les pour une barre nue."` zh `"默认启用推荐插件；关闭它们即可获得干净的栏位。"`
- New key `plugin.none` (all catalogs): en `none`, de `keine`, fr `aucun`, zh `无`.
- New confirm keys (all catalogs):
  - `confirm.preset` / `confirm.mode` / `confirm.wallpaper` / `confirm.plugins` / `confirm.location` / `confirm.files`: en `Preset` `Mode` `Wallpaper` `Plugins` `Location` `Files`; de `Voreinstellung` `Modus` `Hintergrundbild` `Plugins` `Ort` `Dateien`; fr `Préréglage` `Mode` `Fond d'écran` `Plugins` `Lieu` `Fichiers`; zh `预设` `模式` `壁纸` `插件` `位置` `文件`.
- `Body()` `PagePlugins`: join plugins; if empty show `plugin.none`.
- `Body()` `PageConfirm` (default case): one labelled line per value plus footprint:

```go
plugins := strings.Join(w.Plugins, ", ")
if plugins == "" {
	plugins = i18n.T(w.Locale, "plugin.none")
}
body := i18n.T(w.Locale, "confirm.blurb") + "\n\n" +
	i18n.T(w.Locale, "confirm.preset") + ": " + w.Preset + "\n" +
	i18n.T(w.Locale, "confirm.mode") + ": " + w.Mode + "\n" +
	i18n.T(w.Locale, "confirm.wallpaper") + ": " + w.WallpaperDir + "\n" +
	i18n.T(w.Locale, "confirm.plugins") + ": " + plugins + "\n" +
	i18n.T(w.Locale, "confirm.location") + ": " + w.Location + "\n" +
	i18n.T(w.Locale, "confirm.files") + ": ~/.local/bin, ~/.config/systemd/user"
if w.PlainNiri {
	body += "\n\n" + i18n.T(w.Locale, "warn.niri_session")
}
```

**Step 1: Failing tests** — extend `TestConfirmWarnsPlainNiriSession` asserts all five labels + `~/.local/bin`; add:

```go
func TestPluginsNoneState(t *testing.T) {
	w := NewWizard(i18n.EN, nil)
	w.Page = PagePlugins
	if !strings.Contains(w.Body(), i18n.T(i18n.EN, "plugin.none")) {
		t.Fatalf("plugins body = %q, want the none label", w.Body())
	}
}
```

**Step 2: fail** → **Step 3: implement** → **Step 4: `go test ./internal/ui/ ./internal/i18n/`** → **Step 5: commit** `fix(ui): confirm summary and plugins copy` .

---

### Task 4: `install.Options.Progress` task-list callback

Closes the #19 data path (part of #28d).

**Files:**
- Modify: `internal/install/install.go`
- Test: `internal/install/progress_test.go` (new; mirror the stub style of `audit_install_test.go`)

**Design:** add to `Options`:

```go
// Progress receives a detached snapshot of the task list every time it
// changes: initial rows (pending/skipped), done-marking after swap, the
// gSlapper row, and the final list. Nil disables progress reporting.
Progress func(tasks []Task)
```

In `Run`, emit `append([]Task(nil), current...)` (a copy!) at these points:
1. after the row-building/download loop, before the swap (rows still pending, skipped rows final),
2. after `res.Tasks = append(res.Tasks, rows...)` / done-marking,
3. after the gSlapper row is appended,
4. immediately before every `return res, nil` (final state).

Implement a local helper `emit := func(tasks []Task) { if opts.Progress != nil { opts.Progress(append([]Task(nil), tasks...)) } }` and a `current()` helper that concatenates `res.Tasks` + `rows`. `Run` semantics (statuses, errors, stamping) must not change.

**Step 1: Failing test** — stubbed `Options` following `audit_install_test.go` (temp home, fake `Download`/`Swap`/`LookPath`/`Systemctl`, `Pin` with one enabled component + `GSlapper`). Capture snapshots in `Progress`; assert at least: first snapshot has the component with `Status == ""` (pending), last snapshot has `Status == Done`, no snapshot shares backing memory with a later one (mutating a captured slice must not change another). Also assert `Progress == nil` runs clean.

**Step 2: fail** → **Step 3: implement** → **Step 4: `go test ./internal/install/`** → **Step 5: commit** `feat(install): report task-list progress to callers`.

---

### Task 5: In-TUI install, done, and failed screens

Closes #19 (UI part), #28d/#28e.

**Files:**
- Modify: `cmd/sysc/main.go`
- Modify: `internal/i18n/catalog/{en,zh-Hans,de,fr}.json`
- Test: `cmd/sysc/main_test.go` (extend) or new `cmd/sysc/install_flow_test.go`

**New i18n keys (all four catalogs, with translations):**

| key | en | de | fr | zh-Hans |
|---|---|---|---|---|
| `install.title` | Installing | Installation | Installation | 安装中 |
| `done.title` | Done | Fertig | Terminé | 完成 |
| `failed.title` | Failed | Fehlgeschlagen | Échec | 失败 |
| `install.blurb` | Installing the SYSC suite. This can take a few minutes. | Die SYSC-Suite wird installiert. Das kann einige Minuten dauern. | Installation de la suite SYSC. Cela peut prendre quelques minutes. | 正在安装 SYSC 套件，可能需要几分钟。 |
| `done.blurb` | Install complete. | Installation abgeschlossen. | Installation terminée. | 安装完成。 |
| `failed.blurb` | Install failed. | Installation fehlgeschlagen. | Échec de l'installation. | 安装失败。 |
| `install.log` | Log | Protokoll | Journal | 日志 |

**Model changes (`cmd/sysc/main.go`):**

```go
type progressMsg struct{ tasks []install.Task }
type installDoneMsg struct {
	res install.Result
	err error
}
```

- model gains: `step ui.Step`, `tasks []install.Task`, `installRes install.Result`, `installErr error`, `logPath string`, `frame int`, `send func(tea.Msg)`, `installer func(seed.Answers, func([]install.Task)) (install.Result, error)`.
- `newModel` sets `step: ui.StepWizard`, `logPath: "/tmp/sysc-installer.log"`.
- `main` builds the program from the model, then wires `m.send = prog.Send` and `m.installer = func(a seed.Answers, progress func([]install.Task)) (install.Result, error) { opts := installOptions(home, p, a, false, loc); opts.InNiriSession = inNiri; opts.Progress = progress; return install.Run(ctx, opts) }` before `Run()`.
- `enter` on `PageConfirm`: set `m.step = ui.StepInstalling`, launch `go m.runInstall()`, return without quitting. `runInstall` creates/truncates `m.logPath`, passes a progress func that logs each snapshot (`name: status (reason)`) and `m.send(progressMsg{...})` with copies, then `m.send(installDoneMsg{...})`; on error write `error: <err>` to the log before closing.
- `Update`: `tickMsg` increments `frame` (spinner), still updates beams. While `m.step == ui.StepInstalling`, ignore all key messages. `progressMsg` replaces `m.tasks`. `installDoneMsg` sets result/err and `step = ui.StepDone` or `ui.StepFailed`. On `StepDone`/`StepFailed`, `enter`, `q`, `ctrl+c` quit. `f9`/`esc`/arrows only in `StepWizard`.
- `View`: step switch — wizard body as today; `StepInstalling` → title `install.title`, body `install.blurb` + task list; `StepDone` → `done.title` + `done.blurb` + task counts + `Log: <path>`; `StepFailed` → `failed.title` + `failed.blurb` + failed task lines + error + `Log: <path>`. All call `ui.View(..., m.w.Page, m.step, ...)`.
- Task list rows (pure helper `taskLines(tasks []install.Task, frame int) string`, spinner frames `⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏`): pending `spinner + " " + name`, Done `"✓ " + name`, Skipped `"- " + name + " (" + reason + ")"`, Failed `"✗ " + name + " (" + reason + ")"`.
- `main` after `Run()`: if `fm.installErr != nil` → `fmt.Fprintln(os.Stderr, err)`, `os.Exit(1)`. Delete the old post-run `install.Run` + `printTasks` block. `printTasks` stays for `--yes` and uninstall.

**Tests (`cmd/sysc/install_flow_test.go`, package main):**
- `TestConfirmStartsInstallInTUI`: model from `newModel`, `Page = ui.PageConfirm`, `Update(tea.KeyMsg{Type: tea.KeyEnter})` → returned model's `step == ui.StepInstalling` and **not** a quit (assert `m.step`), with `installer` injected to a fake that blocks on a channel so no goroutine races; close channel before test end.
- `TestTaskLines`: pending/Done/Skipped/Failed render distinct glyphs and reasons.
- `TestDoneAndFailedStepsQuit`: set `step = ui.StepDone` → `Update("q")` returns `tea.Quit`; same for `StepFailed`.
- `TestInstallingIgnoresKeys`: `step = ui.StepInstalling` → `q`/`esc` do not quit and do not change step.

Use `installer` injection for determinism; never call the real `install.Run` in tests.

**Step 6: verify** — `go test ./cmd/sysc/ ./internal/...` then full gates.
**Step 7: commit** — `feat(ui): install inside the TUI with live tasks, done and failed screens`.

---

### Task 6: One-screen snapshot tests at 80x24 and 120x40

Closes #28 acceptance (b) and (i).

**Files:**
- Test: `internal/ui/snapshot_test.go` (new)

**Design:** for every page (Theme, Wallpaper, Plugins, Weather, Confirm) with `StepWizard`, and for `StepInstalling`/`StepDone`/`StepFailed`, at both 80x24 and 120x40:

```go
b := NewBeamsTextEffect(width, BannerHeight(), Banner()) // no Update: deterministic blank rows
out := View(i18n.EN, "T", "body", page, step, width, height, b)
lines := strings.Split(out, "\n")
if len(lines) != height { fail }                    // exactly one screen, filled
for _, l := range lines {
	if lipgloss.Width(l) > width { fail }           // never wider than the terminal
}
joined := out
if !strings.Contains(joined, "╭") || !strings.Contains(joined, "╰") { fail }  // rounded box
if !strings.Contains(joined, Nav(i18n.EN, page, step)) { fail }               // footer present
```
Additionally assert the beams region: `BannerHeight()` blank lines precede the title (title is the first non-empty line) — this is the deterministic pre-Update state.

Run with `go test ./internal/ui/`. Commit `test(ui): lock the one-screen chrome at 80x24 and 120x40`.

---

### Task 7: Full gates, push, PR, review, merge (PR1)

```bash
gofmt -l . && go vet ./... && go test -race -count=1 ./... && go build ./...
git push -u origin fix/parity-installer-ui
gh pr create --title "Installer UI parity: one-screen chrome, honest copy, in-TUI install" --body "$(cat <<'EOF'
Closes #18, #19, #20, #25, #26, #27.

- Beams canvas is banner-sized (11 rows), resized to banner height, so the chrome stops overflowing 80x24 (#18).
- Greet-parity monochrome palette and height-filled layout; footer pinned to the last line (#28b/c).
- Confirm stays in the TUI: live task list, done and failed screens with a log pointer (#19, #28d/e).
- Confirm summarises preset, mode, wallpaper dir, plugins, location plus install files (#20).
- Plugins copy reflects on/off with an explicit none (#25).
- Page-accurate footers: no dead arrows, Weather advertises Ctrl+C, not q (#26, #27).
EOF
)"
```

Then dispatch a review subagent on the PR diff (spec compliance vs this task list, then code quality), fix findings on-branch, re-run gates, push. Merge with `gh pr merge <n> --merge --delete-branch` after LGTM. Verify issues #18 #19 #20 #25 #26 #27 auto-close; if not, close them referencing the PR.

---

## PR2 — `fix/wizard-page-honesty` (closes #21 #22)

Branch from updated `main`.

### Task 8: Reachable dark/light mode toggle

**Files:** `internal/ui/wizard.go` (add `CycleMode`), `cmd/sysc/main.go` (`left`/`right` keys on `PageTheme`), `internal/ui/chrome.go` (Theme nav hint), all four catalogs, tests.

- Add key `nav.mode`: en `Mode`, de `Modus`, fr `Mode`, zh `模式`.
- `Nav` `PageTheme` becomes `↑↓ <nav.preset> • ←→ <nav.mode> • Enter ...`.
- `main` `Update` key case: `"left", "right"` → if `m.w.Page == ui.PageTheme { m.w = m.w.CycleMode() }`; keep up/down on Theme for Preset.
- `Wizard.CycleMode()` toggles `Mode` between `"dark"` and `"light"`.
- Tests: `TestCycleModeToggles` (wizard), `TestThemeArrowsChangeMode` (cmd/sysc: `Update("left")` flips mode to light, `"right"` back), update nav assertion.
- Commit `feat(ui): reachable dark/light mode on the theme page`.

### Task 9: Wallpaper page and README honesty

**Files:** `internal/i18n/catalog/*` (`wallpaper.blurb`), `README.md` lines 7–9, tests.

- `wallpaper.blurb` en: `"SYSC uses wallpapers from this directory. Your other wallpaper tools stay installed."` de `"SYSC verwendet Hintergrundbilder aus diesem Verzeichnis. Deine anderen Wallpaper-Werkzeuge bleiben installiert."` fr `"SYSC utilise les fonds d'écran de ce dossier. Vos autres outils de fond d'écran restent installés."` zh `"SYSC 使用此目录中的壁纸。其他壁纸工具仍会保留。"`
- README: `walks first-run defaults: theme, wallpaper directory, recommended bar plugins, and weather location.`
- Test: assert `wallpaper.blurb` in every catalog does not contain `engine`/`Engine` (and translated equivalents are covered by the visible directory wording); simplest deterministic test: `TestWallpaperCopyMentionsDirectory` asserting `strings.Contains(Body(), WallpaperDir)` and `!strings.Contains(blurb, "engine")` for EN.
- Commit `fix(ui): wallpaper page copy matches what it configures`.

### Task 10: Gates, push, PR, review, merge (PR2)

Same gate block, branch `fix/wizard-page-honesty`, title `Wizard page honesty: mode toggle and wallpaper copy`, body `Closes #21, #22.` Review, fix, merge.

---

## PR3 — `fix/uninstall-cli` (closes #23 #24)

Branch from updated `main`.

### Task 11: Uninstall confirmation prompt

**Files:** `cmd/sysc/main.go`, `cmd/sysc/audit_cli_test.go` (call sites), new tests.

- Signature: `func runUninstall(args []string, in io.Reader, out io.Writer, home string) int`; `main` passes `os.Stdin`.
- When `!o.Yes`: print what will be removed — `"This removes the SYSC suite binaries and user units."`, plus `"The user config under ~/.config/sysc-shell is removed too (--purge)."` when `o.Purge`, plus `"gSlapper is removed with the AUR helper (--remove-gslapper)."` when `o.RemoveGSlapper` — then `"Proceed? [y/N] "`, read one line with `bufio.NewScanner(in)`, accept only `y`/`Y`/`yes` (case-insensitive, trimmed). Anything else (including EOF): `"Aborted."`, return 1.
- `--yes` skips both the summary and the prompt.
- Commit `feat(cli): uninstall asks before removing anything`.

### Task 12: Uninstall exits non-zero on failed tasks

**Files:** `cmd/sysc/main.go`.

- Add `func anyFailed(tasks []install.Task) bool` (Done/Skipped pass, Failed fails).
- In `runUninstall`, after `printTasks`: if `err == nil && anyFailed(res.Tasks)` → print `"uninstall finished with failed tasks"` and return 1; keep `err != nil` → 1.
- Tests: `TestAnyFailed`; prompt-abort test (`runUninstall([]string{}, strings.NewReader("n\n"), &out, home)` → 1 and output contains `Aborted.`); `--yes` empty-home test still returns 1 with "no SYSC installation found". Update every existing `runUninstall(...)` call site for the new `in` parameter.
- Commit `fix(cli): uninstall exits 1 when a task fails`.

### Task 13: Gates, push, PR, review, merge (PR3)

Branch `fix/uninstall-cli`, title `Uninstall CLI: confirmation prompt and failure exit code`, body `Closes #23, #24.` Review, fix, merge.

---

## Final verification

- All three PRs merged, all 11 issues closed (verify `gh issue list --state open` shows none of #18–#28).
- `main` green on all four gates.
- No sysc-lock artifacts committed.