# `sysc update` design

Kind: design. Scope: the Update entry the owner-approved suite-installer design already carries, implemented on the existing stop-swap-stamp pipeline. This document is a design only: it authorizes no product code, no release, and no tag.

Baseline: `sysc` origin/main `466106d`. Ledger: `sysc-shell/docs/plans/2026-10-14-pleamar-improvement-ledger.md` REC-ddist-01 (bd `sysc-1225`, gh Nomadcxx/sysc#63). The plan that implements it is `2026-10-15-sysc-update.md`.

## Evidence and responsible owners

| Fact | Where it lives |
|---|---|
| `Run` is the only pipeline: lock, ownership, plan files, download-all-then-swap, stamp persist, restart-on-failure, seed, niri, conflicts, dbusact, start | `internal/install/install.go:202` |
| Stop before swap, `wasActive` restart defer | `internal/install/install.go:407`, `:467` |
| Stamp persist closure, written only after a swap is undoable | `internal/install/install.go:398`, `:472`, `:494` |
| `.bak` swap with symlink refusal | `internal/fetch/fetch.go:142`, `:168` |
| `.bak` trust chain on a rerun | `internal/install/ownership.go:91` |
| Single state marker and lock | `internal/stamp/stamp.go:47`, `:73`; `internal/install/ownership.go:21` |
| The pin is embedded, so an installer only knows its own pin | `internal/pin/pin.go:139`, `internal/pin/pin.json` |
| Stamp is the source of truth; "already at this pin" skips | `2026-10-05-sysc-suite-installer-design.md:102` |
| `config.json` is never overwritten on update | `2026-10-05-sysc-suite-installer-design.md:155`, `internal/seed/seed.go:113` |
| "Uninstall (welcome: Install / Uninstall; Update when a stamp exists)" | `2026-10-05-sysc-suite-installer-design.md:172` |
| Documented one-liner: download a release asset, verify SHA256, run it | `2026-10-05-sysc-suite-installer-design.md:176`, `docs-site/content/docs/start/install.mdx` |
| No update or check verb exists | `git grep -n 'update\|check' -- cmd internal` on origin/main |
| Releases published: v0.1.0 … v0.1.3 (latest v0.1.3, 2026-10-14) | `gh release list -R Nomadcxx/sysc` |

Owners: `internal/install` keeps owning the pipeline, `internal/fetch` keeps owning verified downloads and the `.bak` swap, `internal/stamp` keeps owning the one state marker, `internal/pin` keeps owning which versions exist. This design adds no owner and takes none away.

Release and tag publication stays owner-gated (`sysc-1158`). `sysc-744`'s report-only newer-release check stays the separate owner of "is there something newer"; update consumes that answer implicitly (the fetched installer is the answer) and does not reimplement a check.

## Approaches considered

1. **Proposed — parent fetches the current release installer, hands the work to it.** `sysc update` downloads `releases/latest/download/sysc-linux-<arch>` plus `SHA256SUMS`, verifies, runs it with `--apply`, and the fetched installer runs the one pipeline against its own pin and prints the delta. One pipeline, one stamp, one rollback story, and the embedded-pin trap is structurally impossible because the applying binary is never the stale one.
2. **Rejected — run the local pipeline and fetch only the new pin.** The pin has no published asset. Publishing `pin.json` as a release artifact would add a second release contract, a second checksum story, and a way for the two to disagree; that is a new distribution mechanism, not a verb.
3. **Rejected — self-replacement.** The installer overwrites its own binary and continues. Rejected on the deploy-guard stance already recorded for the shell (`sysc-744`), and it needs its own versioning, resume and rollback story.
4. **Rejected — a TUI welcome page offering Install / Uninstall / Update.** The approved design mentions a welcome page; the shipped TUI (`internal/ui/wizard.go`) has no welcome page today, so that would be new UI in this change. The verb delivers the capability first; the page can come later.

## D1. The verb and its flags

`sysc update [--yes] [--lang LANG] [--apply]`, dispatched beside `uninstall` before `parseFlags`, added to the usage text:

```
Usage: sysc [flags]            install
       sysc uninstall [flags]  remove an installation
       sysc update [flags]     update the installed suite
```

`--yes` skips the confirmation, matching uninstall's `Proceed? [y/N]` contract; `--lang` matches the existing flag families; `--apply` is the delegate mode (D2) and is documented but not advertised in the usage summary. Exit codes: `0` done or nothing to do, `1` refusal or failed task, `2` usage error, unchanged from `parseFlags` and `runUninstall`.

## D2. Two roles, one pipeline

**Parent** (the binary the user typed, which only needs this verb to be present):

1. Read the stamp at `$XDG_STATE_HOME/sysc/installed.json`. Absent → refusal, exit 1.
2. Arch check; non-amd64 keeps the named refusal (`refuse.distro`) because the pin has amd64 assets only.
3. Download `releases/latest/download/sysc-linux-<arch>` and `releases/latest/download/SHA256SUMS` into a fresh temp dir through the existing `fetch` client, which verifies the sha256 from the checksum line for that asset.
4. `chmod 0700` and run `<temp>/sysc update --apply [--yes]`, streaming its stdout and stderr.
5. Propagate the child's exit status, then remove the temp dir. A temp dir left behind by a crash is named in the refusal for the next run and removed then.

**Child** (`--apply`, i.e. the freshly fetched installer, the only party that knows the new pin):

1. Read the stamp (absent → refusal).
2. Print the delta (D3). Empty delta → `already at <release>` and exit 0 without touching a binary.
3. Confirm `continue? [y/N]`, or proceed under `--yes`.
4. Recover `seed.Answers` from the user's config (D4).
5. Call `install.Run` with the existing options builder, no conflicts, and the same lock, then `printTasks`.

The parent never swaps a binary, never writes the stamp, and never learns the new pin. That is what keeps this from becoming a second installer.

An installer older than the release that first ships this verb cannot update itself; the documented download-and-run route in `docs-site/content/docs/start/install.mdx` remains the path for those installs, which is already what that page tells users to do.

## D3. The delta

`internal/update.Delta(prev stamp.Stamp, p pin.Pin) []Row`, one row per enabled component in the pin or recorded in the stamp, sorted by component id:

| Row | Meaning | Shape |
|---|---|---|
| changed | stamp version differs from the pin tag | `~ sysc-shell  v0.1.0 -> v0.2.0` |
| added | component enabled in the pin, absent from the stamp | `+ sysc-terminal  v0.2.0` |
| removed | recorded in the stamp, disabled or absent in the pin | `- sysc-notify  v0.1.1` |
| unknown | stamp has no version for it (legacy stamp) | `? sysc-tray  unknown -> v0.1.2` |

Unchanged components are omitted. Header `SYSC update v0.1.1 -> v0.1.3` above the rows, then one line naming the installer that will run: `applying with the v0.1.3 installer`. Versions only: no dates, no release notes, no changelog, because the repo cannot produce one and a delta that reads like a changelog is a promise nobody keeps. A disabled component in the pin is skipped for rows but still counted in the "no amd64 asset" refusal path inside `install.Run`.

## D4. Answers for the pipeline

`install.Run` validates `seed.ConfigJSON(opts.Answers)` before anything else, and that requires weather coordinates, even though `seed.Write` never overwrites an existing config — so update needs answers it does not intend to write. They come from the user's own config: a new `seed.AnswersFromConfig(path)` reads only `weather.latitude`, `weather.longitude` and `weather.location` and returns them as `Answers`. Every other field stays zero: `ConfigJSON` defaults `preset` and `mode`, and the wallpaper, plugin and tray keys it would emit are never written because the config exists.

No config, or a config without coordinates → refusal naming `sysc` install, exit 1. Guessing a location during an update would silently change the user's weather widget the first time a config was ever rewritten, and it is not this verb's job.

## D5. What update does not repeat

- **Conflicts.** No `conflict.Detect`, no `conflict.Defaults`, no `ConflictEnv`. Handovers happened at install time, `stamp.HandedOver` is carried into the new stamp by `install.Run`, and re-running a takeover against a service the user has since reconfigured is a new risk with no update benefit.
- **gSlapper, dbusact, units.** Re-run through their existing idempotent paths: the gSlapper row skips as "already installed by SYSC", unit writes back up foreign units, dbusact re-installs its file.
- **niri.** Re-run, and it is idempotent by construction: an already-commented `spawn-at-startup "sysc-shell"` line does not match `spawnRe`, the `include "sysc.kdl"` line is matched by `includeRe`, and sidecar binds are only added when the key is free. The plan proves this with a test that applies twice against a temp config and asserts the second write is byte-identical; if that test fails, update gains an option that skips `niri.Apply` rather than the test being weakened.
- **Preflight.** Kept: `refuse.root`, `refuse.distro` and the runtime checks run on the delegate, so a root invocation or an unsupported distro is refused before any download.

## D6. Refusals and failure paths

| Situation | Output | Exit | What changed |
|---|---|---|---|
| No stamp (parent or `--apply`) | `No SYSC install stamp at <path>. Run sysc to install the suite.` | 1 | nothing |
| Non-Arch | `refuse.distro` | 1 | nothing |
| Download or checksum failure | fetch client text, e.g. `checksum mismatch for sysc-linux-amd64: got <a> want <b>` | 1 | nothing; temp dir removed |
| Another install or uninstall running | `another SYSC install or uninstall is in progress` | 1 | nothing |
| Component binary is pacman-managed | existing `install.CheckOwnership` refusal | 1 | nothing |
| Stamped binary changed on disk | existing `planFile` refusal | 1 | nothing |
| Untrusted `.bak` present | existing `preserving existing rollback backup …` refusal | 1 | nothing |
| User declines the prompt | `Aborted.` | 1 | nothing |
| A task fails inside `install.Run` | existing task list plus the wrapped error | 1 | whatever the pipeline's rollback undoes, which is its existing behaviour |
| Child exits nonzero | child output, verbatim | child's code | parent's fetch only |

Nothing on the failure path writes outside the pipeline's own rollback. The parent writes nothing at all except its temp dir.

## D7. Ownership, state, and what is deliberately not replaced

`sysc update` adds no file, no marker, no lock. State stays in `installed.json`, `install.lock`, `staging/` and `installer.log`, all owned by the existing packages.

The installer binary at `~/.local/bin/sysc` is **not** replaced. Update runs the fetched binary from a temp dir and leaves the installed one alone; after success it prints one line naming the documented install one-liner for moving the installer itself forward. Replacing a running binary is self-replacement by another name, and no user has asked for it. If the repeated "run the one-liner again" turns out to be the common complaint, that is the moment to design installer self-replacement with its own rollback, not to smuggle it in here.

## D8. Localisation

The delta rows and the task list are untranslated, exactly like the existing `printTasks` output. Two new keys are the whole localisation cost: `refuse.stamp` and `update.current`. `TestEveryKeyExistsInAllCatalogs` means each key lands in all nine catalogues; that cost is the reason the surface stays at two keys.

## D9. Tests and the live matrix

Installer rule: table tests, one package, a named `-run`. Never `go test ./...`, never `-race` (`2026-10-05-sysc-suite-installer-design.md:187`).

- `internal/update`: `TestDelta` (unchanged, changed, added, removed, legacy-unknown, disabled component, empty stamp), `TestChecksumFor` (matching line, missing asset, short or non-hex digest, CRLF, extra names), `TestRunRefusesWithoutStamp`, `TestRunRefusesChecksumMismatch` (injected download; asserts the child was never started and the temp dir is gone), `TestRunPropagatesChildExit`.
- `cmd/syc`: `TestRunUpdateUsage`, `TestRunUpdateAlreadyAtPin` (empty delta → exit 0, `install.Run` never called, using an injected runner seam), `TestRunUpdateRefusesWithoutStamp` (exact line and exit code), `TestRunUpdateDeclined`.
- `internal/seed`: `TestAnswersFromConfig` (full, partial, missing weather block, malformed JSON, absent file).
- `internal/niri`: `TestApplyTwiceIsByteIdentical`.

Live Niri matrix, this machine: usage, `--help`, and the no-stamp refusal against a temp `XDG_STATE_HOME` are runnable. The swap path is **not** exercised here — running it would replace the owner's suite binaries — and the table tests stand in for it. That limit is recorded rather than papered over.

## D10. Rollout, rollback, stop condition

The verb becomes available when the next installer release ships it; installs older than that keep the documented route. Rollback is a revert: the stamp format, the lock and the pipeline are untouched, so there is nothing to migrate. Stop condition: if the parent/child split cannot be built from the fetch and exec seams named in D2, stop and design update around a published pin artifact instead of growing a second installer.

## Non-goals

No background or scheduled check (`sysc-744` owns the report-only check). No self-replacement. No downgrade verb. No pacman/AUR path. No arm64 install: the installer binary is published for arm64, but the pin carries amd64 assets only, so arm64 keeps the named refusal. No Nix packaging (that is the shell's flake, not the installer's). No changelog or release-notes surface. No plugin update path.

## Local design/plan audit

Findings against the 2026-08-26 development-orchestration gates, applied here:

- **Owners and data flow.** Named per package; the parent/child split and "the parent never writes the stamp" remove the only place a second owner could appear.
- **Error and shutdown behaviour.** D6 tables every refusal with its exit code and what changed; the temp dir and the child's exit status are the only shutdown paths.
- **Tests and live hardware matrix.** D9 states which live rows are unrunnable on this machine and why, instead of claiming a swap that was never performed.
- **Explicit non-goals.** Listed, including the two that a reader would otherwise assume in (installer self-replacement, Nix).
- **Stop or reconsider condition.** D10.
- **Scope added.** One new package (`internal/update`), one new exported helper in `internal/seed`, two catalogue keys, one verb. No new dependency: the download reuses `internal/fetch`, the process spawn reuses the existing `exec` pattern in `runPackage`.
- **Architecture review.** This adds no goroutine owner, no new persisted schema, no UI primitive and no exported repository split; the stamp schema is unchanged. It does add a child process, which is why D2 fixes the exact argv and D6 fixes what happens when it fails.