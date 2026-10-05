# SYSC Installer Audit Report — 2026-10-05

Auditor: independent adversarial review per commission
(`docs/plans/2026-10-05-sysc-installer-audit-commission.md`). Target commit: `2de99b5`.
All audit tests live on branch `audit/installer-2026-10-05` as
`internal/*/audit_*_test.go` + `cmd/sysc/audit_main_gap_test.go` and are throwaway
harnesses, not production code. No source file was modified; the real `$HOME` was
never touched (every test used `t.TempDir()`; verified `~/.local/state/sysc` absent
after all runs).

## 1. Verdict

**NOT READY to ship.** The architecture is sound and most of the security posture
holds under attack (fail-closed checksums, fail-closed distro/arch gates, idempotent
niri include, sacred first backups, unit invariants, i18n parity). But the hero path
is broken end-to-end: `curl | sh install.sh` can never complete because its checksum
verification targets a filename the script never creates (AUD-01), the advertised
`uninstall` capability is unreachable from the CLI (AUD-06), reinstall silently
destroys the hotkeys the installer itself installed (AUD-02), and a start failure
leaves an unrecoverable half-installed state (AUD-03). Sixteen claims-tested
behaviors include four outright refutations (claims 11, 13, 14, 15) and one partial
(claim 8). The seed-vs-shell contract check proves the installer's missing input
validation can brick the bar on first launch (AUD-04, confirmed against the real
`sysc-shell` config loader). None of this requires a redesign — every fix is local —
but nothing here should be announced as installable until AUD-01..06 are closed.

## 2. Findings

Severity-ordered. `REPRO` commands run from the repo root (tests) or are described
for script/CLI checks against `/tmp/audit-script-harness` fixtures.

### AUD-01 (High) — `install.sh` can never succeed: checksum check names a file that does not exist

`install.sh:18-21`:

```sh
curl -fsSL "$BASE/$ASSET" -o "$TMP/sysc"
curl -fsSL "$BASE/SHA256SUMS" -o "$TMP/SHA256SUMS"
grep " $ASSET\$" "$TMP/SHA256SUMS" > "$TMP/check"
(cd "$TMP" && sha256sum -c check)
```

The download is saved as `$TMP/sysc`, but `SHA256SUMS` (release.yml: `sha256sum sysc-linux-*`)
records the name `sysc-linux-amd64`/`sysc-linux-arm64`. `sha256sum -c` therefore
looks for a file that was never created.

- Consequence: the single advertised install path fails with exit 1 on every arch,
  even when the bytes are a perfect match. Fails closed (no bad binary is executed),
  but the product is uninstallable via its hero command.
- REPRO: local fixture with correct hashes (see §5 harness): `sh /tmp/audit-script-harness/run.sh`
  or manually stage `SHA256SUMS` containing the binary's real digest under name
  `sysc-linux-amd64` while the file on disk is `sysc`.
- Observed: `sysc-linux-amd64: FAILED open or read` + `sha256sum: WARNING: 1 listed file could not be read`, exit 1, with a provably-matching digest.
- Fix: `curl -o "$TMP/$ASSET"` then `mv`/exec that path, or `sed "s/ $ASSET\$/ sysc/"` when writing `check`. Add a CI smoke test that runs `install.sh` against a fixture server — this class of bug is invisible to `sh -n` and unit tests.

### AUD-02 (High) — re-running the install strips the hotkeys the installer itself added

`internal/niri/niri.go:117` (`occupiedKeys` scans the config **and every include it
matches**, including the freshly-ensured `include "sysc.kdl"` line, so run 1's own
sidecar counts as "occupied"):

- Consequence: second `sysc` run (reinstall/upgrade) rewrites `sysc.kdl` with **zero**
  binds — `Mod+Space` launcher and `Mod+Comma` settings silently vanish after upgrade.
- REPRO: `go test ./internal/niri -run TestAuditApplyTwiceKeepsBinds` (two `Apply` calls with the same options over one temp HOME).
- Observed: FAIL — `run2 BindsSkipped=[Mod+Space Mod+Comma]`, sidecar contains no `bind` block.
- Fix: exclude `opts.SidecarPath` from the occupied-keys include scan; a SYSC-generated
  sidecar is not foreign occupancy.

### AUD-03 (High) — start failure (or SIGKILL) after enablement leaves an unrecoverable orphan install

`internal/install/install.go:182-201`: units are enabled, seeded, niri-applied, then
started; `stamp.Write` runs only after a successful start (line 201), and a start
error returns before it. `Uninstall` begins at `install.go:214` with:

```go
st, err := stamp.Read(opts.stateDir())
if err != nil {
    return res, fmt.Errorf("no SYSC installation found: %w", err)
}
```

- Consequence: if `systemctl --user start` fails (or the process dies) after enable,
  the machine has enabled units, swapped binaries, seeded config, patched niri — and
  `Uninstall` refuses to touch any of it. There is no recovery path in the tool.
- REPRO: `go test ./internal/install -run TestAuditStartFailureStateIsRecoverable`.
- Observed: FAIL — enabled units + binaries on disk; `Uninstall` → `no SYSC installation found: open .../installed.json: no such file or directory`.
- Fix: write the stamp before `StartAll` (with `started:false`) and let start failure
  be a reported, recoverable state; this preserves "stamp after enable" while closing
  the orphan window. Same window applies to §5.6's SIGKILL drill — real-kill was not
  tested, the code path is identical.

### AUD-04 (High) — installer accepts weather input the shell hard-rejects; seeded bar won't start

`internal/seed/seed.go:44-47` validates only zero-coords / empty-label; no ±90/±180
range check. `cmd/sysc/main.go:117` `in.CharLimit = 80` is 80 **runes**, while
sysc-shell caps `weather.location` at 80 **bytes** (`internal/config/config.go:458`)
and `internal/config/load.go:284` sets `DisallowUnknownFields` — any invalid value
means the bar fails startup (`cmd/sysc-shell/main.go:80-86`).

- Consequence: `--lat 95` or a 30-hanzi city name installs "successfully" and the
  panel never launches again — with zh-Hans a first-class locale, CJK overrun is a
  likely user path, not a curiosity.
- REPRO: `go test ./internal/seed -run 'TestAuditSeedAccepts'` then load the emitted
  JSON with the real shell (§5.4 contract run).
- Observed: installer accepts lat 95, lon 200, 90-byte label; real `config.Load`
  rejects all three verbatim: `weather.latitude: 95 is outside -90 through 90`,
  `weather.longitude: 200 is outside -180 through 180`, `weather.location: is 90 bytes, over the 80-byte limit`.
- Fix: mirror the shell's range + byte-limit validation in `seed.ConfigJSON` (and
  post-guess/search labels), reject before writing.

### AUD-05 (High) — XDG_CONFIG_HOME / XDG_STATE_HOME ignored; seeded config invisible to the shell

`internal/install/install.go:65-67` (`configPath` = `Home + "/.config/..."`), same
hardcoding for state/bin/unit dirs. The shell's `config.DefaultPath()`
(`sysc-shell/internal/config/load.go:255`) uses `os.UserConfigDir()`, which honors
`XDG_CONFIG_HOME`.

- Consequence: for any user with `XDG_CONFIG_HOME` set, the install writes a config
  the bar will never read (bar runs on defaults), and uninstall "purge" deletes the
  wrong directory while the real one survives.
- REPRO: `go test ./internal/install -run TestAuditSeedHonorsXDGConfigHome` (temp HOME + `XDG_CONFIG_HOME` override).
- Observed: FAIL — config absent at `$XDG_CONFIG_HOME/sysc-shell/config.json`.
- Fix: use `os.UserConfigDir()`/`os.UserStateDir()` equivalents (respect env, fall back to `~/.config`, `~/.local/state`) in the one place dirs are computed.

### AUD-06 (High) — uninstall is unreachable from the CLI; `--help` and unknown flags print nothing; `sysc uninstall` opens the install wizard

`cmd/sysc/main.go:37` `fs.SetOutput(io.Discard)`; flag set is `--yes --city --lat --lon --lang` only.
`internal/install.Uninstall` (`install.go:210`, fully implemented and tested) has zero
call sites outside tests.

- Consequence: the capability the code and commission treat as shipped cannot be
  invoked; a user typing `sysc uninstall` gets the positional silently ignored and is
  dropped into the **install wizard** (further re-config on a broken install). Help
  is invisible, so flag discovery is impossible. Family precedent: `sysc-greet`'s
  installer ships a working uninstall mode (`cmd/installer/main.go:235-236`).
- REPRO: build (`go build -o /tmp/audit-sysc-bin ./cmd/sysc`); `/tmp/audit-sysc-bin --help`; `... --bogus`; `echo | /tmp/audit-sysc-bin uninstall`.
- Observed: `--help` → no output, exit 2. `--bogus` → no output, exit 2. `uninstall`
  → alt-screen wizard launched (escape sequences on stdout), exit 0. No file written
  to the real `$HOME`.
- Fix: wire `uninstall [--purge]` subcommand to `install.Uninstall`; give `flag.FlagSet`
  a real output (stderr); README flags table must list it once wired.

### AUD-07 (Medium) — `Run` never stops running units before swapping binaries (claim 13 refuted)

`internal/install/install.go:130-133` (download → swap per component); `StopAll`
appears only inside `Uninstall` (`install.go:217`).

- Consequence: upgrading while the bar runs overwrites a live binary (Linux: ETXTBSY
  or inode-swap leaving the old process current); the new version is inert until
  session restart with no user-visible hint.
- REPRO: `go test ./internal/install -run TestAuditRunStopsUnitsBeforeSwap`.
- Observed: FAIL — recorder shows zero `stop` calls before three swaps.
- Fix: `units.StopAll` before the swap phase (Run already knows how, order exists in `units.go:32`).

### AUD-08 (Medium) — the Go installer never checks host architecture (claim 14 refuted)

Zero occurrences of `runtime.GOARCH` in `cmd/` or `internal/`. `install.sh:10-14` maps
`aarch64|arm64 → sysc-linux-arm64` and refuses unknown archs **by name**, but invoking
the binary directly (`.install` path, `go run ./cmd/sysc`) on arm64 downloads amd64
assets from `pin.json`'s `Assets["amd64"]` (`install.go:125`) with no refusal.

- REPRO: `go test ./internal/install -run TestAuditRunStopsUnitsBeforeSwap` companion
  assertion — grep proof: `grep -rn runtime.GOARCH cmd internal` → no matches.
- Observed: `install.Run` builds every asset list from the literal key `"amd64"`.
- Fix: refuse up-front when `runtime.GOARCH != "amd64"`, naming the host arch; or
  generalize pin assets and select by GOARCH.

### AUD-09 (Medium) — uninstall removes unit files but never `disable`s or reloads; wants symlinks dangle

`install.go:210-253` StopAll + `os.Remove` of unit paths only; no `systemctl --user
disable` and no `daemon-reload` string exists repo-wide.

- Consequence: `~/.config/systemd/user/graphical-session.target.wants/sysc-*.service`
  symlinks survive uninstall, pointing at deleted units; the next session logs
  failures, and reinstall behavior around stale wants is undefined.
- REPRO: `go test ./internal/install -run TestAuditUninstallDisablesUnits`.
- Observed: FAIL — "uninstall removed unit files but never ran systemctl disable/daemon-reload; graphical-session.target.wants symlinks dangle".
- Fix: `disable --now` before file removal, `daemon-reload` after.

### AUD-10 (Medium) — per-component download→swap interleaving + rollback can't undo a fresh install (claim 4 partial)

`install.go:130-133` swaps immediately after each component's download instead of
the design's download-all-then-swap; `internal/fetch/fetch.go:123` — `Rollback`
silently `continue`s when a `.bak` doesn't exist, and on a first install none do
(`fetch.go:105` only backs up an existing dst).

- Consequence: component 2 download failing after component 1 swapped leaves the new
  binary in `~/.local/bin` with no stamp and no rollback — same orphan family as
  AUD-03. (`TestAuditRollbackOrphanOnFreshInstall` FAIL: orphan `binDir/one` present
  after error; no `.new` litter — that part is clean.)
- REPRO: `go test ./internal/install -run TestAuditSecondComponentFailureLeavesFirstSwapped`
  and `go test ./internal/fetch -run TestAuditRollbackOrphanOnFreshInstall`.
- Observed: FAIL both — first binary swapped and still there, no stamp.
- Fix: download+verify all staged assets before any swap (the staging dir already
  makes this a small loop reorder); on rollback, remove swapped-in files lacking a `.bak`.

### AUD-11 (Medium) — pin validation gaps enable path traversal and un-verified transport

`internal/pin/pin.go` `Decode` validates the documented eight fields but not: binary
name safety, URL scheme, SHA hex format/length, unit membership, duplicate IDs, tag
emptiness, or gslapper package emptiness. Combined with `fetch.go:76`
(`rename(tmp.Name(), filepath.Join(staging, a.Name))`) and `fetch.go:111` (rename
into `binDir + name`), a name like `../../.bashrc` writes outside both dirs.

- REPRO: `go test ./internal/pin -run TestAuditDecodeAcceptsHostileValues` (FAIL —
  all eight hostile values accepted); `go test ./internal/fetch -run 'TestAudit.*Traverses'` (FAIL — bytes land outside `binDir`).
- Observed: `'../../.bashrc'`, `http://…`, non-hex `sha`, short `dead`, bogus unit,
  duplicate IDs, empty tag, empty package — all pass `Decode`.
- Impact today is limited because pin is compiled in via `go:embed` and the checked-in
  values are sane; it becomes High the moment the design's "published pin JSON" path
  is wired (then pin is untrusted input). Trust-boundary validation belongs here.
- Fix: reject names with separators/`..`, require `https://`, `^[0-9a-fA-F]{64}$`,
  unit ∈ `units.All`, unique IDs.

### AUD-12 (Medium) — wizard "wallpaper engine" choice is a placebo; terminal option shown despite disabled component (claim 15 refuted)

`cmd/sysc/main.go:229` cycles `model.Engine` ∈ stills/gslapper/terminal; `internal/ui/wizard.go:114,125` render it.
No consumer: `seed.Answers` has no Engine field, `install.Run` decides gSlapper
purely from `LookPath` + helper (`install.go:139-149`). And `terminal` stays selectable
though `pin.json` ships sysc-terminal disabled ("no release assets yet"), violating
the design rule "hide terminal-effects engine while sysc-terminal disabled".

- REPRO: grep proof — `grep -rn "\.Engine" cmd internal --include='*.go'` shows only
  assignment + display; wizard run-through needs no input about engines to change output.
- Observed: any engine choice yields byte-identical install results.
- Fix: either consume the choice (gate gslapper on it, error on terminal) or remove
  the row. Removing is honest and smaller.
- Related placebo surface: `seed.Answers.Mode`/`WallpaperDir` exist but neither wizard
  nor `--yes` flags ever set them — design's wallpaper-dir/mode answers are
  unreachable at the CLI. Dead knobs should go or be wired.

### AUD-13 (Medium) — uninstall does not restore niri autostart (asymmetric to install)

`internal/niri/niri.go:135-150` (`Remove` deletes the sidecar and strips the include)
never un-comments the `spawn-at-startup "sysc-shell"` lines that `Apply` commented
(`niri.go:48` `spawnRe` path).

- Consequence: after uninstall, the user's pre-existing shell-autostart stays off and
  no unit exists to start it — bar silently gone from their session with no pointer.
- REPRO: `go test ./internal/niri -run TestAuditRemoveRestoresSpawnLine` (FAIL — line
  remains `// spawn-at-startup "sysc-shell"`).
- Fix: the comment was written by SYSC (marker style), so uncomment the lines it
  commented; if too fragile, README/printTasks must tell the user to re-enable autostart.

### AUD-14 (Low) — component without a matching unit is silently installed, stamped, and reported Done

`install.go:156-157` — `unitFor(c.ID)` miss → `continue` with no task, no warning.
`TestAuditComponentWithoutUnitIsSilent` FAIL (observed: enabled+stamped with zero unit
files). Latent: all three current pin IDs match `units.All` names exactly.

### AUD-15 (Low) — gSlapper skip reason doesn't name the package (claim 8 half-refuted)

`install.go:143`: `"no AUR helper found"`; design/claim want the SKIP reason to name
`gslapper` (the package the user should install manually). PATH-skip and
never-replace-existing both hold.

### AUD-16 (Low) — primary `--yes` refusal bypasses i18n

`install.go:94` returns hardcoded `"weather coordinates are required"` while all four
catalogs carry an unused `refuse.weather` key (dead key; only code-key miss found in
an otherwise-perfect parity check). A `--lang de` user gets English here, and
`printTasks` statuses/reasons from `install.Run` are English throughout.

### AUD-17 (Low) — `install.sh` litter: `exec` skips cleanup, `$TMPDIR/sysc-install.$$` keeps the downloaded binary forever

`install.sh` `mkdir -p "$TMP"` then `exec`s; no trap/`rm`. Same pattern in `.install`
(that one does `rm -rf` at the end).

### AUD-18 (Info)

- `fetch.go:72` compares digests with `strings.EqualFold` — uppercase SHAs accepted (correct-equivalent, not fail-closed-reject as one might assume).
- `fetch.download` has no size cap — a hostile/compromised asset fills staging/disk (test: 4 MiB unbounded success). Fail-closed on truncation: PASS (`Content-Length` mismatch errors).
- `swapOne` replaces a symlinked `binDir` target with a regular file without comment (`TestAuditSwapAllReplacesSymlinkedBinary` FAIL if refusal expected) — deliberate-looking, flag in docs if kept.
- Staged seed/stamp files end up `0600` (CreateTemp default) — fine, just noting.
- `niri.Apply` appends the include with bare LF into a CRLF config; `i18n.Match` maps `zh-TW` to Simplified.
- release.yml builds `sysc-linux-arm64` (grep-compatible two-space SHA256SUMS format) — the arm64 story only fails on the Go side (AUD-08) and hero path (AUD-01).

## 3. Claim verdicts (§6, verbatim list — 15 claims, not 20)

| # | Claim (abbrev.) | Verdict | Evidence |
|---|---|---|---|
| 1 | pin Decode rejects 8 documented cases | CONFIRMED (scope note: accepts everything else, AUD-11) | `TestAuditDecodeDocumentedRejectionsHold` PASS |
| 2 | distro gate: 4 families pass; fedora/debian/ubuntu refused by name; empty ID unknown | CONFIRMED | author `distro_test.go` table + `preflight_test.go:19`; harness refusal `unsupported architecture`-style naming verified |
| 3 | 4-catalog key parity + LANG match rules + F9 cycle | CONFIRMED | parity grep 22/22/22/22; `TestAuditLangMatchTable` PASS; `i18n.Next` cycles all four |
| 4 | checksum mismatch writes nothing; swap failure restores every .bak; no .new remains | REFUTED (partial) | mismatch/404/abs-name fail-closed PASS; but fresh-install rollback orphans swapped binary (AUD-10), no `.new` litter holds |
| 5 | stamp only after units enabled; records release/versions/gslapper/started=false on SSH | CONFIRMED | `TestAuditEnableFailureLeavesNoStamp` PASS; `TestAuditSSHSessionEnablesOnly` PASS (`started:false`, zero start calls); orphan caveat AUD-03 |
| 6 | seed refuses empty weather; full bar-right + weather item; never overwrites | CONFIRMED (validation gap is AUD-04, outside claim text) | `TestAuditSeedNeverOverwritesExisting` PASS; coords/location empties rejected both directions PASS; defaultRight byte-equivalent to shell `Default()` |
| 7 | `--yes` without location refuses naming `--city/--lat/--lon` | CONFIRMED | `TestAuditLatLngPairNamesFlags` PASS; `main.go` guess-fail message names both flag pairs |
| 8 | gSlapper PATH-skip; no-helper SKIP **naming the package**; existing never replaced | PARTIALLY REFUTED | PATH-skip ✓, replacement ✓, reason wording ✗ (AUD-15) |
| 9 | disabled components skipped by name, zero fetches | CONFIRMED | `TestAuditDisabledComponentZeroFetch` PASS (no terminal/lock asset fetched; `res.Tasks` carries reason "no release assets yet") |
| 10 | uninstall keep leaves config / purge removes / gSlapper only if stamped installed | CONFIRMED in code+tests — but UNREACHABLE from CLI (AUD-06) | author `TestUninstallKeepsConfig:169`; `TestAuditUninstallPurgeRemovesConfig` PASS; gate `install.go:232` |
| 11 | niri: include idempotent, sysc-shell.kdl untouched, spawn commented not deleted, occupied binds skipped, first .sysc.bak never overwritten | REFUTED | per-item PASS controls all green, but double-Apply treats own sidecar as occupancy and strips its binds (AUD-02) |
| 12 | stop shell-first / start clipboard-first; start only when graphical-session active; WantedBy + `%h/.local/bin` ExecStart | CONFIRMED | recorder test (runtime stubs, not constants); templates inspected — no `default.target`, no `/usr/local` |
| 13 | `install.Run` stops running units before swapping | REFUTED | AUD-07 |
| 14 | `install.Run` refuses unsupported host arch by name | REFUTED | AUD-08 (script refuses; binary does not check at all) |
| 15 | wizard hides terminal-effects engine while sysc-terminal disabled | REFUTED | AUD-12 |

## 4. Adversarial test log (§5.3 → tests, verdicts)

Convention: `FAIL` = the test asserts the claimed invariant and the code breaks it
(i.e., finding reproduced); `PASS` = invariant held. Run pattern:
`timeout 90s env GOMAXPROCS=2 go test -count=1 ./PKG -run NAME`.

| Area | Test | Verdict | Links |
|---|---|---|---|
| fetch | `audit_fetch_test.go`: `TestAuditAssetNameTraversesOutOfStaging` | FAIL | AUD-11 |
| fetch | `TestAuditSwapAllNameTraversesOutOfBinDir` | FAIL | AUD-11 |
| fetch | `TestAuditRollbackOrphanOnFreshInstall` | FAIL | AUD-10 |
| fetch | `TestAuditSwapAllReplacesSymlinkedBinary` | FAIL | AUD-18 |
| fetch | `TestAuditRollbackLeavesNoNewFile` / uppercase-sha / short / non-hex / 404 / truncated / absolute-name (gap file) | PASS | fail-closed confirmed |
| fetch | `TestAuditUnboundedDownloadSize` | PASS (4 MiB accepted) | AUD-18 |
| pin | `TestAuditDecodeAcceptsHostileValues` | FAIL | AUD-11 |
| pin | `TestAuditDecodeDocumentedRejectionsHold` | PASS | claim 1 |
| pin | embedded placeholder-zeros Load | PASS (fails downstream at download) | §7 accepted |
| seed | lat 95 / lon 200 / 90-byte CJK label / dir-path silent nil | FAIL ×4 | AUD-04 |
| seed | never-overwrite / key shape / coord-or-location pairing (gap file) | PASS | claim 6 |
| niri | `TestAuditApplyTwiceKeepsBinds` | FAIL | AUD-02 |
| niri | `TestAuditRemoveRestoresSpawnLine` | FAIL | AUD-13 |
| niri | CRLF bare-LF include | FAIL (asserted Info-level) | AUD-18 |
| niri | foreign spawn untouched / foreign sidecar FirstBak / missing config refusal / trailing-comment idempotent / sysc-shell.kdl untouched (gap) / commented-include re-add (gap) / comment-only key not occupied (gap) / first bak sacred across applies (gap) | PASS ×8 | claim 11 items |
| install | `TestAuditRunStopsUnitsBeforeSwap` | FAIL | AUD-07 |
| install | `TestAuditSecondComponentFailureLeavesFirstSwapped` | FAIL | AUD-10 |
| install | `TestAuditStartFailureStateIsRecoverable` | FAIL | AUD-03 |
| install | `TestAuditComponentWithoutUnitIsSilent` | FAIL | AUD-14 |
| install | `TestAuditUninstallDisablesUnits` | FAIL | AUD-09 |
| install | `TestAuditSeedHonorsXDGConfigHome` | FAIL | AUD-05 |
| install | `TestAuditGslapperSkipReasonNamesPackage` | FAIL | AUD-15 |
| install | `TestAuditEnableFailureLeavesNoStamp` / `TestAuditSSHSessionEnablesOnly` | PASS | claims 5, 12 |
| install | corrupt-stamp refusal / unknown-stamped-component ignored / purge removes / disabled zero-fetch (gap file) | PASS ×4 | claims 10, 9 |
| units | start/stop order via recording stub (gap file) | PASS | claim 12 |
| i18n | `Match` table incl. `zh_CN.UTF-8`, `zh-Hans-CN`, `de_DE`, `fr_FR`, `ja_JP`, empty (gap file) | PASS | claim 3 |
| cmd | `--lat` w/o `--lon` (and symmetric) refuses, names flags, no network (gap file) | PASS | claim 7 |

Script/CLI harness (no Go test): `--help`/unknown-flag silence + `uninstall`→wizard
(AUD-06); `install.sh` hero-path mismatch bug + fail-closed cases + `ppc64le` refusal
by name (AUD-01/18); `.install` missing-go/git guards print and exit 1 before any
`mktemp`.

## 5. Contract check vs real sysc-shell (§5.4)

Method: scratch `git worktree` of `/home/nomadx/sysc-shell` @ `8140efc2` (detached,
`/tmp/sysc-shell-audit`), throwaway test calling `config.Load(path)` on installer-emitted
JSON; worktree removed after (`git worktree remove --force`); shell repo untouched.

- `audit-seed-happy.json` → **LOADS**, weather bound, right bar = seeded.
- `lat 95` → rejected: `weather.latitude: 95 is outside -90 through 90`.
- `lon 200` → rejected: `weather.longitude: 200 is outside -180 through 180`.
- 90-byte CJK `location` → rejected: `weather.location: is 90 bytes, over the 80-byte limit`.
- Amplifier: `decoder.DisallowUnknownFields()` (load.go:284-285) + startup fail on
  invalid config → every seed-content bug is bar-killing, which promotes AUD-04 to High.
- Key-by-key agreement otherwise: `theme.preset`, `theme-gen.source="wallpaper"`,
  `theme-gen.mode="dark"`, `weather{latitude,longitude,location}`,
  `bar.items.right`, `plugins.enabled`, `wallpaper.image_directory`, item `id:"weather"`
  — all valid shell fields; `defaultRight` is currently byte-equivalent to shell
  `Default()` right section (drift risk remains: hardcoded copy, seed.go:29-38).
- Path mismatch is the other contract break: shell reads `os.UserConfigDir()`
  (honors `XDG_CONFIG_HOME`), installer writes hardcoded `~/.config` → AUD-05.
- Unit ExecStart names match pin binaries (`sysc-walls-daemon` ✓), user-scoped `%h` paths ✓.

## 6. Failure drills (§5.6) mapping

| Drill | Status |
|---|---|
| download fails mid-run after swap 1 | reproduced — AUD-10 (FAIL test) |
| enable fails | clean abort, no stamp — PASS control |
| start fails after partial starts | unrecoverable orphan — AUD-03 |
| niri mid-write on read-only/disk-full | NOT dynamically tested (no root to mount ro); statically: `writeAtomic` rename error propagates, Run aborts pre-stamp — same orphan class as AUD-03; mark unverified |
| SIGKILL between swap and stamp | identical state to AUD-03 (no stamp yet); recovery absent — code-path evidence, real kill not exercised |
| uninstall with stamp but missing binaries | `os.Remove` errors ignored (`install.go:229-231`) → Task Done anyway; benign |

## 7. UX / i18n honesty (§5.7)

- Catalog parity verified mechanically (22 keys × 4); `Match`/F9 per claim 3.
- Refusal paths mostly honor `--lang`/`$LANG` via `i18n.T` **except** the main `--yes`
  weather guard (AUD-16) and all `install.Run` task reasons (English).
- Wizard is honest about what it does **not** offer except the engine row, which is a
  placebo that silently does nothing (AUD-12) — the single UX lie found.
- `--yes` answers == wizard answers structurally (both feed one `seed.Answers`), minus
  the unreachable `Mode`/`WallpaperDir` (AUD-12 tail).
- Help: nonexistent (`io.Discard`) — AUD-06. README flags table matches real flags;
  README never promises uninstall, so the doc is silent-honest while the commission
  treats it as shipped.

## 8. Docs & release chain (§5.8)

- README:18 hero one-liner fetches the raw binary with **no checksum step at all** —
  inconsistent with install.sh's (broken) verify and with README:65's honest "do not
  curl until first release" banner; both README install instructions fail today
  (releases have no assets — §7 known).
- README:22 describes install.sh as a working fetcher — contradicted by AUD-01.
- release.yml: tag `v*`, `CGO_ENABLED=0`, amd64+arm64 assets, SHA256SUMS two-space
  format — the only grep-compatible consumer is install.sh, which can't complete
  (AUD-01). No publish of `pin.json` as an artifact (embed-only), matching §7 notes.
- `.install`: guards verified live (missing go/git → clean named refusal, exit 1, no
  litter before checks); unpinned `git clone --depth 1 main` stands as §7-accepted
  residual risk; `exec </dev/tty 2>/dev/null || true` can kill a strict non-interactive
  dash before the `|| true` applies (special-builtin redirection failure semantics) —
  Low, only bites `curl | sh` in terminal-less environments, which is exactly where
  `--yes` auto-append lives.

## 9. Residual risks / not covered by this audit

- §7 known-accepted gaps confirmed, not re-litigated: placeholder-zero SHAs (download
  fails closed — PASS), no component release assets, sysc-terminal disabled with
  reason, gSlapper AUR-only, `--version` absent, same-origin trust of install.sh/.install.
- Never executed against a live systemd user session or real niri — unit semantics
  (`is-active --quiet graphical-session.target` under a real session, Wants symlink
  lifecycle) verified by contract + recording stubs only.
- No real SIGKILL/read-only-mount races (see §6); no real arm64 hardware; bubbletea
  wizard exercised through model methods + headless view, not a human keystroke session.
- Real GitHub release endpoint untouched (no network); all download paths via httptest/file fixtures.

## 10. Appendix — per-file checklist (§4, one-line verdicts)

| File | Verdict |
|---|---|
| cmd/sysc/main.go | flags/i18n/preflight/pin wiring solid; help silenced + uninstall unwired + rune-limit input (AUD-04/06/12) |
| internal/install/install.go | spine order correct per design except download-all-before-swap + stop-before-swap + stamp window + XDG + arch (AUD-03/05/07/08/10/14) |
| internal/install/install_test.go | author tests genuine, green; don't cover failure-order windows this audit found |
| internal/fetch/fetch.go | verify-before-rename ✓ within staging; rollback incomplete for fresh installs; name unsanitized (AUD-10/11/18) |
| internal/pin/pin.go + pin.json | 8 documented rejections ✓; hostile extras accepted (AUD-11); assets/units/tag consistency ✓ |
| internal/niri/niri.go | include idempotent ✓ sacred first bak ✓; self-occupancy bind-strip + asymmetric Remove (AUD-02/13) |
| internal/seed/seed.go | never-overwrite ✓ full bar ✓; no range/byte validation (AUD-04); dead Mode/WallpaperDir plumbing (AUD-12) |
| internal/stamp/backup/units/geo/distro/preflight | clean against design contract; units templates exactly per spec |
| internal/i18n + catalogs | 4×22 parity ✓; `refuse.weather` dead, one hardcoded English refusal (AUD-16) |
| internal/ui (beams/chrome/wizard/banner) | FG+BG ✓ min-size ✓ page order ✓; engine placebo (AUD-12) |
| install.sh | arch map + fail-closed grep ✓ but hero path can't complete (AUD-01), TMP litter (AUD-17) |
| .install | guards verified live; unpinned clone accepted residual; dash tty-quirk Low |
| README.md | honest "not ready" banner ✓; two install instructions both non-functional (AUD-01 + no assets) |
| .github/workflows/release.yml | mechanics consistent with install.sh expectations; moot until AUD-01 fixed |
