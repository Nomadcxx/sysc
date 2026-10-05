# Commission: root-and-branch audit of the SYSC suite installer

- **Commissioned:** 2026-10-05
- **Target repository:** `github.com/Nomadcxx/sysc` (local checkout `/home/nomadx/sysc`)
- **Target commit at commission time:** `2de99b5377475ea16a81f1ab8b437ce9cacc9ae8` (`2de99b5 docs: center the banner and refresh its cache`)
- **Auditor:** an independent agent with no authorship of this code
- **Deliverable:** one audit report, `docs/plans/2026-10-05-sysc-installer-audit-report.md`, committed on a scratch branch (see §9)
- **Status:** commissioned, not started

---

## 1. Mandate

Audit the installer root and branch. The author's tests describe intent; they are
not evidence of correctness. Assume every claim in §6 is false until you have
reproduced it yourself from the code and from execution. Your job is to find
what is wrong, what is missing, what is dishonest, and what will hurt a real
user on a real Niri machine — not to confirm the author's story.

Two failure classes matter most:

1. **A user's machine is damaged or left half-installed.** Data loss, clobbered
   config, a running shell swapped out from under the session, a stamp that
   claims success when units are down.
2. **The installer lies.** A screen, README, flag, or stamp that says something
   the run did not do.

Style, naming, and micro-optimization are out of scope unless they cause one of
the above.

## 2. Target and environment

- Go 1.27.1 linux/amd64 is installed. The module declares `go 1.26`.
- One output, `DP-1` 3440×1440 scale 1.0. Niri has no runtime virtual output, so
  two-output behavior is unrunnable here; say so rather than guessing.
- Live Niri environment, if you need it:
  ```bash
  export NIRI_SOCKET=$(ls /run/user/1000/niri.wayland-*.sock | head -1)
  export WAYLAND_DISPLAY=wayland-1
  export XDG_RUNTIME_DIR=/run/user/1000
  ```
- Do **not** run the installer against the real `$HOME`. Every dynamic check
  uses a temp `HOME`/`XDG_*` tree. See §9.
- The component repositories are out of scope except where the pin references
  their release assets. Their state at commission time:
  - `sysc-shell`: no tags, no release workflow, no release assets.
  - `sysc-clipboard`: tags `v0.1.0`, `v0.1.1`; no release workflow; no assets.
  - `sysc-walls`: tags `v1.0.0`, `v1.0.1`; a release workflow exists but no
    GitHub release has been published; its workflow builds amd64 only.
  - `sysc-terminal`: docs-only; no code, no tags, no assets.
  - `sysc-lock`: not shipped.
  - `gslapper`: AUR package `gslapper` 1.5.1 only.

## 3. Ground truth

Read these before touching code. They are the contract the implementation
claims to satisfy:

- `docs/plans/2026-10-05-sysc-suite-installer-design.md` — approved design.
- `docs/plans/2026-10-05-sysc-suite-installer.md` — implementation plan.
- `docs/plans/2026-10-05-sysc-suite-installer-research.md` — prior art and the
  pitfall checklist the design was written against.

The primary copies live in `/home/nomadx/sysc-shell/docs/plans/` (gitignored
there). The copies in this repository are the committed ones. If they differ,
the committed copies are the target.

## 4. Scope

**In scope:** every tracked file in the repository at the target commit —
`cmd/sysc/`, all of `internal/` (`backup`, `distro`, `fetch`, `geo`, `i18n`,
`install`, `niri`, `pin`, `preflight`, `seed`, `stamp`, `ui`, `units`),
`install.sh`, `.install`, `.github/workflows/release.yml`, `README.md`,
`assets/banner.svg`, `go.mod`, `go.sum`, and the docs in `docs/plans/`.

**Out of scope:** fixing anything (see §9), component repository internals,
Niri compositor behavior beyond what the sidecar writes, and the owner's
`scripts/deploy` machinery in sysc-shell.

## 5. Method

### 5.1 Static review

Read every file end to end. For each claim in §6, locate the code that is
supposed to implement it and decide whether it does. Record `file:line` for
every finding. Do not skim `internal/install/install.go` — it is the spine and
deserves line-by-line reading, including error paths.

### 5.2 Dynamic verification

- `gofmt -l .` must be empty; `go vet ./...`; `go build ./...`.
- Run the existing tests with the repository's convention:
  `timeout 90s env GOMAXPROCS=2 go test -count=1 <pkg> -run <Name>`.
  The plan forbids `go test ./...` and `-race`; you may run `-race` on a single
  package if it adds signal, and say that you did.
- Then write **new** adversarial tests on your scratch branch. The author's
  tests are the floor, not the ceiling. §5.3 lists the ones that must exist.

### 5.3 Adversarial tests (new, required)

Write these as throwaway tests on the scratch branch. Each must fail or pass
with a clear verdict; report the verdict even when the code is correct.

**Pin and trust boundary**
- A pin whose binary `name` is `../evil` or `/tmp/evil`: does `fetch.SwapAll`
  or `install.Run` write outside the bin directory?
- A pin with an uppercase SHA256, a short SHA256, or a non-hex SHA256: does
  verification fail closed?
- A pin with `disabled: true` and an empty reason: rejected?
- A pin with an enabled `sysc-lock`: rejected?
- A pin with no `amd64` asset but an `arm64` one: rejected on amd64?

**Fetch and swap**
- Checksum mismatch: nothing written under the destination, no temp left.
- 404, truncated body, and a body larger than any sane limit: behavior?
- `SwapAll` on a fresh install where the destination does not exist: does the
  `.bak` copy path handle `ENOENT`, or does the first install fail?
- `SwapAll` failure mid-list: every already-swapped binary restored from
  `.bak`, no `.new` left, and the failed one untouched.
- `Rollback` when a `.bak` is missing: error or silent corruption?
- Symlink at the destination path: followed, replaced, or refused?

**Seed and shell contract**
- `seed.Write` when `config.json` exists: byte-identical afterwards?
- `seed.Write` when the config path is a directory or unreadable: error?
- `seed.ConfigJSON` with coordinates but no location, location but no
  coordinates, and out-of-range coordinates: all rejected?
- The seeded JSON must load in the real shell. Do the contract check in §5.4.

**Niri**
- `config.kdl` with CRLF line endings, no trailing newline, `include "sysc.kdl"`
  already present with different spacing, and a commented-out include: does
  `Apply` stay idempotent and does `Remove` restore the original bytes?
- `spawn-at-startup` with different quoting/spacing for `sysc-shell`: is it
  commented, and are `cliphist`/`polkit`/`gsettings` lines untouched?
- A bind key that appears only inside a comment: is it treated as occupied?
- A bind key that appears in an included file: skipped?
- `sysc-shell.kdl` (the shell's own theme include): never written or removed?
- Missing `config.kdl`: named refusal, no sidecar created?

**Backups**
- Mutate twice: the first `*.sysc.bak` bytes are still the original.
- More than five state copies: rotation keeps five and never deletes a
  `*.sysc.bak`.
- A foreign `sysc.kdl` without the marker: backed up before overwrite?

**Units and systemd**
- Stop order is shell → walls → clipboard; start order is clipboard → walls →
  shell. Verify with a recording stub, not by reading the constants.
- Start is skipped when `graphical-session.target` is not active.
- Unit templates: `WantedBy=graphical-session.target`, `ExecStart=%h/.local/bin/...`,
  no `/usr/local`, no `WantedBy=default.target`.
- `systemctl enable` failure: does the run stop before stamping?
- Uninstall: are the `graphical-session.target.wants` symlinks removed, or do
  dangling symlinks remain? Is `daemon-reload` called after unit removal?

**Stamp**
- Units not enabled: no `installed.json`.
- Enabled but not started (SSH/TTY): `started: false`, file written.
- Corrupt `installed.json`: `Read` errors, `Uninstall` refuses by name.
- A stamp listing an unknown component: uninstall behavior?

**Install pipeline**
- `--yes` with no weather flags and a failing guess: refuses, names the flags.
- A disabled component: `SKIP` with its reason, zero fetches for it.
- gSlapper on `PATH`: package hook never called.
- gSlapper absent and no AUR helper: `SKIP` naming the package, suite still
  completes.
- **Does `Run` stop running units before swapping binaries?** The design's
  replace order is stop shell → stop walls/clipboard → swap. Verify against
  the code and report the actual order.
- **Does `Run` check the host architecture?** The pin requires `amd64`; on an
  `arm64` host, is the amd64 asset used anyway, or is the host refused by name?
- Partial failure: download fails for the second component — is the first
  component's binary left swapped, and is the stamp absent?
- `Uninstall` keep vs purge: keep leaves `config.json`, purge removes it;
  gSlapper is removed only when the stamp says SYSC installed it.

**Preflight and flags**
- Root, foreign distro, and Wayland-without-Niri each refuse by name.
- SSH/TTY (neither `WAYLAND_DISPLAY` nor `NIRI_SOCKET`): allowed.
- `--lat` without `--lon` and vice versa: refused.
- `--help`: what does the user actually see? (`fs.SetOutput(io.Discard)` is
  suspicious.)
- Unknown flag: exit code and message.

**install.sh and .install**
- Unsupported architecture: named refusal, no download.
- Checksum mismatch: non-zero exit, nothing executed.
- Non-TTY: `--yes` is added; TTY: it is not.
- `.install` with no `go` or no `git`: named failure, temp cleaned.
- `.install` with no TTY: does it still run, and with `--yes`?

**i18n and UI**
- Every key present in all four catalogs, non-empty, and no key used by code
  missing from a catalog. Grep for `i18n.T(` and `T(loc,` call sites and check
  each key.
- `LANG=zh_CN.UTF-8` → `zh-Hans`; `de_DE` → `de`; `fr_FR` → `fr`;
  `en_US`/`ja_JP`/empty → `en`.
- F9 cycles every screen and the nav strings follow the locale.
- The installing step's help has no cancel key.
- Every lipgloss style sets FG **and** BG (the plex2jellyfin lesson).
- Below 80×24: the enlarge page appears, nothing panics.
- CJK strings: do they render at the right width, or do they overflow the box?

### 5.4 Contract check against the real shell

The seed hand-builds JSON. A field-name typo would be silently ignored by the
shell's decoder. Prove the seeded file actually configures the shell:

1. Copy `/home/nomadx/sysc-shell` to a scratch directory (or use a git
   worktree) — do not modify the primary checkout.
2. Generate a seeded `config.json` with `seed.ConfigJSON` (or `seed.Write` into
   a temp `XDG_CONFIG_HOME`).
3. In the scratch copy, write a throwaway test that calls the shell's config
   loader on that file and asserts: weather is configured, the weather item is
   on the bar, the theme preset and wallpaper engine are applied, and the
   plugin ids are enabled.
4. Report the result, including any field the shell ignored.

Also check whether the shell's decoder rejects unknown fields. If it does not,
say so: it means every future seed typo is silent.

### 5.5 Security review

- **Integrity:** SHA256 comparison — length checked before compare? Constant
  time is not required for integrity, but a short/empty hash must not pass.
- **Transport:** TLS verification is on by default; confirm no `InsecureSkipVerify`.
  Redirects: how many, and to where?
- **Path handling:** every place a pin-supplied or stamp-supplied string becomes
  a filesystem path (`Binary.Name`, component `ID`, unit name). Traversal,
  absolute paths, empty names.
- **TOCTOU:** checksum verified on the staged file, then renamed. Can the
  staged file change between verify and rename? Is the staging directory
  user-owned and mode-checked?
- **Permissions:** binaries `0755`, configs `0644`, state dirs `0755`; nothing
  world-writable.
- **Command execution:** AUR helper and `systemctl` invocations use fixed
  argument vectors, never a shell. Package names come from the pin; confirm no
  interpolation into a shell string.
- **Environment trust:** `HOME`, `XDG_CONFIG_HOME`, `XDG_STATE_HOME`,
  `TMPDIR`, `PATH`. Does the installer honor `XDG_CONFIG_HOME` for the shell
  config, or does it hardcode `~/.config` while the shell honors XDG? A
  mismatch means the seed lands where the shell never reads it.
- **Root:** the refusal lives in `preflight`/`main`. `install.Run` itself has no
  root check — note whether a direct caller could install as root.
- **curl|sh chain:** `install.sh` fetches the binary and `SHA256SUMS` from the
  same origin. State the residual risk plainly; it is accepted, not a finding,
  but the report must say so.
- **`.install`:** clones `main` unpinned and builds it. State the residual risk.

### 5.6 Failure-mode drills

For each, describe the user-visible end state and whether it is honest:

- Download fails after the first component swapped.
- `systemctl enable` fails for one unit.
- `systemctl start` fails for the shell after clipboard and walls started.
- Niri config is read-only or the disk is full mid-write.
- The process is killed (SIGKILL) between swap and stamp.
- Uninstall runs with a stamp but missing binaries.

### 5.7 UX and i18n honesty

- Does the complete screen read run state, or intent? Trace `printTasks` and
  `Result.Tasks` back to what actually happened.
- Are task names and statuses localized, or English-only? The design says the
  installer UI is localized; decide whether the task list is UI.
- Does the wizard hide options that need disabled components? The design says
  the terminal-effects wallpaper engine is hidden while `sysc-terminal` is
  disabled. Check `main.go`'s engine cycle against the pin.
- Does the wizard expose dark/light mode and the wallpaper directory, or only
  the preset and engine? Compare against the design's wizard section.
- Does the weather page enforce the ≤80-byte place label, or only 80
  characters?
- Does `--yes` produce the same answers the wizard defaults would?

### 5.8 Docs and release chain

- README claims vs actual flags and behavior, line by line.
- `install.sh` asset names vs the release workflow's outputs vs the README
  one-liner: all three must agree.
- The release workflow: tag trigger, `CGO_ENABLED=0`, both architectures,
  `SHA256SUMS` format, asset names. Does `install.sh`'s `grep " $ASSET\$"`
  match the format `sha256sum` actually writes?
- The embedded pin (`internal/pin/pin.json`) vs the published-pin story in the
  design: is the pin actually embedded, and is there any path that reads a pin
  from disk?
- The banner: no byline, centered in the README, renders without mangling.

## 6. Claims to verify

Confirm or refute each. A refuted claim is a finding; a confirmed claim is
evidence the report must still show.

1. Pin decode rejects: empty release, empty recommended list, disabled row
   without reason, enabled `sysc-lock`, component with no binaries, missing
   `amd64` asset, empty URL/SHA256.
2. Distro gate: `arch`, `cachyos`, `manjaro`, `endeavouros` proceed; `fedora`,
   `debian`, `ubuntu` refuse by name; empty ID refuses as unknown.
3. Every catalog key exists in all four catalogs; `Match` maps
   `zh_CN.UTF-8`→`zh-Hans`, `de_DE`→`de`, `fr_FR`→`fr`, everything else→`en`.
4. Checksum mismatch writes nothing; swap failure restores every `.bak`; no
   `.new` remains.
5. Stamp is written only after units are enabled; it records release, component
   versions, `gslapper_installed`, and `started` (false from SSH/TTY).
6. Seed refuses empty weather; writes the full bar right section plus the
   weather item; never overwrites an existing `config.json`.
7. `--yes` without a weather location refuses and names `--city`/`--lat`/`--lon`.
8. gSlapper on `PATH` is skipped; no AUR helper means `SKIP` naming the package;
   an existing gSlapper is never replaced.
9. Disabled components are skipped by name with zero fetches.
10. Uninstall keep leaves `config.json`; purge removes it; gSlapper is removed
    only when the stamp says SYSC installed it.
11. Niri include is idempotent; `sysc-shell.kdl` is never touched;
    `spawn-at-startup "sysc-shell"` is commented, not deleted; occupied binds
    are skipped; the first `*.sysc.bak` is never overwritten.
12. Units stop shell first and start clipboard first; start only when
    `graphical-session.target` is active; templates use
    `WantedBy=graphical-session.target` and `%h/.local/bin`.
13. `install.Run` stops running units before swapping binaries.
14. `install.Run` refuses an unsupported host architecture by name.
15. The wizard hides the terminal-effects engine while `sysc-terminal` is
    disabled.
16. The wizard exposes preset, dark/light mode, wallpaper directory, plugin
    toggles, and weather; the task list is localized.
17. `--help` prints useful usage.
18. The seeded JSON loads in the real shell and configures weather, theme,
    wallpaper, and plugins.
19. `install.sh` and `.install` behave as documented, including non-TTY.
20. The release workflow produces exactly the assets `install.sh` expects.

## 7. Known gaps (confirm, do not re-report as novel)

These are known and accepted at commission time. Confirm they are still true
and describe their user-visible effect; do not spend the report budget
"discovering" them:

- `internal/pin/pin.json` carries placeholder SHA256 zeros. A real install
  fails at checksum until the first pin is cut. Confirm it fails **closed**.
- No component release assets exist, so the hero path cannot complete. The
  first pin is tracked as `sysc-1003`, blocked by gates `sysc-999`–`sysc-1002`
  in the sysc-shell bd graph.
- `sysc-terminal` is disabled in the pin with reason "no release assets yet".
- gSlapper is AUR-only; no helper means skip.
- No component binary supports `--version`; the stamp is the source of truth.
- `install.sh` and `.install` fetch from the same origin they trust; accepted
  for v1.

## 8. Deliverable

One report at `docs/plans/2026-10-05-sysc-installer-audit-report.md`, committed
on the scratch branch, plus a register row in `docs/plans/README.md`.

Structure:

1. **Verdict** — one paragraph: would you let this install on your machine?
2. **Findings** — ordered by severity. Each finding:
   - ID (`AUD-01`…), severity, title
   - `file:line`
   - What the code does (with the exact snippet)
   - Why it matters (user-visible consequence)
   - Reproduction: exact command and observed output
   - Suggested fix (described, not applied)
3. **Claim verdicts** — a table of §6 claims: confirmed / refuted / unverified,
   with evidence.
4. **Adversarial test log** — each §5.3 test: what you wrote, verdict, output.
5. **Contract check result** — §5.4, including ignored fields.
6. **Residual risks** — accepted risks restated, plus anything you could not
   test on this hardware.
7. **Appendix** — per-file checklist with a one-line verdict per file.

Severity taxonomy:

- **Critical** — data loss, machine damage, security hole, or a stamp/screen
  that lies about a broken install.
- **High** — install fails or leaves a broken session in a common path; a
  documented behavior is absent.
- **Medium** — wrong behavior in an uncommon path; misleading docs; missing
  localization.
- **Low** — cosmetic, style, or robustness nit with no user-visible failure.
- **Info** — observation, accepted risk, or question for the owner.

Evidence standard: every finding must be reproducible from the report alone.
No "looks like", no "probably", no finding without a command and its output.
If you could not verify something, mark it unverified and say why.

## 9. Rules of engagement

- Work on a scratch branch `audit/installer-<date>` in a worktree or a clone.
  Never commit to `main`, never push to `main`, never force-push.
- **Do not fix anything.** The report is the deliverable. Fixes are a separate
  commission.
- Never run the installer against the real `$HOME`. Use temp `HOME`,
  `XDG_CONFIG_HOME`, `XDG_STATE_HOME`, and `TMPDIR` for every dynamic check.
- Never run `install.Run` with real network downloads. Use `httptest` or the
  injectable seams (`Download`, `Swap`, `LookPath`, `InstallPkg`, `Systemctl`).
- Do not touch `.beads/`, `docs/plans/README.md` rows other than your own, or
  any file another session has modified. The primary sysc-shell checkout has
  other sessions' uncommitted work; leave it alone.
- The machine `commit-msg` hook rejects ordinary English matching `agent`,
  `cursor`, `codex`, `llm`, `bot`, and similar. Screen commit messages before
  committing on the scratch branch.
- If you need bd, run it from `/home/nomadx/sysc-shell`, never from a worktree.
- Timebox is not fixed; depth is the point. Stop when §10 is met, not when the
  report is long.

## 10. Exit criteria

- Every file in §4 read; the appendix checklist is complete.
- Every claim in §6 has a verdict with evidence.
- Every adversarial test in §5.3 exists and has a verdict.
- The contract check in §5.4 has a result.
- The report is committed on the scratch branch with a register row.
- No unverified assertion is presented as fact.

## Appendix A: per-file checklist

| File | Read | Verdict |
|---|---|---|
| `cmd/sysc/main.go` | | |
| `cmd/sysc/main_test.go` | | |
| `internal/backup/backup.go` + test | | |
| `internal/distro/distro.go` + test + testdata | | |
| `internal/fetch/fetch.go` + test | | |
| `internal/geo/geo.go` + test | | |
| `internal/i18n/i18n.go` + test + 4 catalogs | | |
| `internal/install/install.go` + test | | |
| `internal/niri/niri.go` + test | | |
| `internal/pin/pin.go` + test + testdata + pin.json | | |
| `internal/preflight/preflight.go` + test | | |
| `internal/seed/seed.go` + test | | |
| `internal/stamp/stamp.go` + test | | |
| `internal/ui/banner.go`, `beams.go`, `chrome.go`, `wizard.go` + tests | | |
| `internal/units/units.go` + test + 3 templates | | |
| `install.sh` | | |
| `.install` | | |
| `.github/workflows/release.yml` | | |
| `README.md` | | |
| `assets/banner.svg` | | |
| `go.mod`, `go.sum` | | |
| `docs/plans/*` (design, plan, research) | | |

## Appendix B: exact commands

```bash
# static
cd /home/nomadx/sysc
gofmt -l .
go vet ./...
go build ./...

# existing tests, repository convention
timeout 90s env GOMAXPROCS=2 go test -count=1 ./internal/pin -run TestDecodeRequiresAssetsAndSHA
timeout 90s env GOMAXPROCS=2 go test -count=1 ./internal/distro -run 'TestFamilyGate|TestParseOSRelease'
timeout 90s env GOMAXPROCS=2 go test -count=1 ./internal/i18n -run 'TestEveryKeyExistsInAllCatalogs|TestMatchLANG'
timeout 90s env GOMAXPROCS=2 go test -count=1 ./internal/fetch -run 'TestChecksumMismatchDoesNotWrite|TestSwapRestoresBakOnFailure'
timeout 90s env GOMAXPROCS=2 go test -count=1 ./internal/stamp -run TestStampAfterEnable
timeout 90s env GOMAXPROCS=2 go test -count=1 ./internal/seed -run 'TestSeedAddsWeatherToDefaultBar|TestSeedRejectsEmptyWeather|TestSeedDoesNotClobberExistingConfig'
timeout 90s env GOMAXPROCS=2 go test -count=1 ./internal/backup -run 'TestFirstBakNotOverwritten|TestStateCopyRotates'
timeout 90s env GOMAXPROCS=2 go test -count=1 ./internal/niri -run 'TestIncludeIdempotent|TestCommentShellSpawnOnly|TestSkipOccupiedBind'
timeout 90s env GOMAXPROCS=2 go test -count=1 ./internal/units -run 'TestStopOrderShellFirst|TestWriteBacksUpForeignUnit'
timeout 90s env GOMAXPROCS=2 go test -count=1 ./internal/geo -run 'TestGuess|TestSearch'
timeout 90s env GOMAXPROCS=2 go test -count=1 ./internal/install -run 'TestYesNeedsWeather|TestSkipGSlapperWhenOnPATH|TestDisabledComponentSkipped|TestUninstallKeepsConfig|TestUninstallPurge|TestUninstallGSlapperGatedOnStamp'
timeout 90s env GOMAXPROCS=2 go test -count=1 ./internal/preflight -run TestPreflight
timeout 90s env GOMAXPROCS=2 go test -count=1 ./internal/ui -run 'TestBannerContainsCowboyLine|TestNavFollowsLocale|TestNavWaitHasNoCancel|TestWizardOrder|TestWeatherBlocksConfirm|TestF9CyclesLocale'
timeout 90s env GOMAXPROCS=2 go test -count=1 ./cmd/sysc -run 'TestParseFlags|TestInstallShPicksAmd64'

# shell syntax
sh -n install.sh
sh -n .install

# banner render (optional)
rsvg-convert -o /tmp/banner.png assets/banner.svg
```
