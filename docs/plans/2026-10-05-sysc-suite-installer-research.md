# SYSC suite installer — prior-art research

Date: 2026-10-05.
Kind: research. Not a design. Decisions wait for the design conversation.

This document records what the existing Nomadcxx Go TUI installers actually do,
what they share, what they get wrong, and what a suite installer for the SYSC
desktop should take or refuse. It is the evidence base for
`YYYY-MM-DD-sysc-suite-installer-design.md`, which does not exist yet.

## Scope

Read-only study of:

| Project | Checkout | Installer |
|---|---|---|
| sysc-Go | `/home/nomadx/Documents/sysc-Go` | `cmd/installer/main.go` (618 lines on master) |
| sysc-greet | `/home/nomadx/sysc-greet` | `cmd/installer/` (~2.7k LOC) |
| moonbit | `/home/nomadx/moonbit` | `cmd/installer/main.go` (~650 LOC) |
| plex2jellyfin | `/home/nomadx/Documents/plex2jellyfin` | `cmd/installer/` + shared `plugininstall` engine |
| smolbot | `/home/nomadx/Documents/smolbot` | `cmd/installer/` + design docs |
| sysc-walls | `/home/nomadx/.config/superpowers/worktrees/sysc-walls/feature-sysc-911-walls` | `cmd/installer/main.go` (~1223 lines) |

Also checked, because SYSC would install them:

- `sysc-shell` (this repo): plugin store, default bar, systemd user unit, `scripts/deploy`
- `sysc-clipboard`: user-local binary + systemd user unit, no TUI installer
- `sysc-terminal`: wallpaper engine (not a terminal emulator), Stage 3, no installer
- `sysc-lock`: unfinished; design lives in `2026-10-05-sysc-lock-design.md`

Not in the suite list the owner named: sysc-greet, sysc-Go (as a user-facing CLI), gSlapper, moonbit, plex2jellyfin, smolbot. Those are prior art only.

## Naming traps (read this first)

| Name | What it actually is |
|---|---|
| **sysc-walls** | Kitty/Wayland **screensaver**, not a wallpaper engine. No gslapper, no wallpaper dirs. |
| **sysc-terminal** | Niri **background-layer wallpaper** that paints sysc-Go effects. Not a terminal emulator. |
| **sysc-Go** | Terminal animation CLI/TUI. Dependency of sysc-terminal. Has its own installer. |
| **gSlapper** | External video wallpaper process. Shell talks to its socket; it is not a SYSC binary. |
| **sysc-lock** | Session locker (PAM + ext-session-lock). Not ready to ship. |
| **SYSC** | In this conversation: the **guided suite installer**. Elsewhere in the tree it also means the bar wordmark / brand. |

A suite installer that treats “walls” as wallpaper and “terminal” as a term will mis-wire the desktop.

## The family pattern

Every installer in this set is a Charm stack TUI (`bubbletea` + `bubbles` + `lipgloss`) with the same skeleton:

1. Welcome (Install / Uninstall, sometimes Update)
2. A small number of choices
3. Sequential task list with spinner and OK / FAIL / SKIP
4. Complete screen

That skeleton came from sysc-Go / jellywatch / greet and was copied forward. smolbot's `installer-ui-fix.md` is explicit: an earlier Catppuccin layout was thrown out in favour of the monochrome “sysc-family” look.

Shared mechanics:

- `installTask{name, description, execute, optional, status}`
- Required failure stops the pipeline; optional failure becomes SKIP
- 200 ms sleep per task so the spinner is visible
- `install.sh`: clone repo (usually `main`/`master`), `go build ./cmd/installer`, run TUI
- Root required except **smolbot** (user-local) and packaged postinstall scripts that defer setup
- Compile-on-target is the primary TUI path; GitHub Releases / AUR / nFPM exist beside it and often install a *different* layout
- Almost no `--yes` / non-interactive mode on the shipping branches
- Almost no transactional rollback

This is a brand, not a library. None of the installers import a shared installer package. Theme colours, task runners, and privilege helpers were copied, then drifted.

## Per-project notes

### sysc-Go — smallest useful wizard

Installs `syscgo` + `syscgo-tui` + assets to `/usr/local`. Install vs Uninstall only. No config, no systemd, no first-run.

Useful: tiny task table, explicit uninstall, consumer-side search of `/usr`, `/usr/local`, and XDG.

Broken on master (fixes exist on unmerged branches):

- `curl|bash` has no TTY → stuck on welcome (fix: `--yes`)
- `installAssets` flattens `assets/*` into `/usr/local/share/syscgo/`; the TUI looks for `.../syscgo/assets/`
- Copies showcase GIFs into system share
- `getProjectRoot` assumes the binary lives at `cmd/installer/<name>`
- Builds as root, leaves root-owned artifacts in the source tree
- PKGBUILD / `.SRCINFO` version+sha desync
- Zero installer tests on master

Lesson: even a “just copy binaries” installer needs a share layout that matches what the app searches, a headless flag, and tests for project-root discovery.

### sysc-greet — do not copy the session takeover

This is the most dangerous installer in the set. It compiles the greeter, writes `/etc/greetd/*`, **overwrites** `config.toml`, writes polkit, **deletes** `display-manager.service`, and enables greetd as the DM. Cage/niri/sway/hyprland session files are embedded as Go strings, drifted from `config/` and Nix.

Useful: platform detection (`os-release`, systemd vs runit, package-name maps), optional tasks, persistent installer log, Nix/nfpm as a *separate* path from the wizard, Void static verify scripts.

Refuse for SYSC:

- Anything that becomes a display manager
- Always-overwrite of system config
- Requiring the compositor binary already on PATH while advertising auto-install
- Fake `SYSC_COMPOSITOR` that still needs Enter and can be clobbered
- curl|bash clone of `master` as the blessed path
- 674 lines of installer-only animation on the critical path

Package `postinstall.sh` is *safer* than the Go wizard (only writes greetd.toml if missing). The wizard is the aggressive one.

### moonbit — timers that do not match the CLI

One binary + optional systemd *system* timers. Schedule screen (daily / weekly / manual). Config is created later by the app.

Useful: optional vs critical split (units can fail, binary cannot); Arch packaging that does **not** auto-enable timers (safer than the TUI default).

Refuse / fix:

- `moonbit-clean.service` calls `--force`, which the CLI does not define → timer likely broken
- `moonbit scan` is interactive; oneshot services need a non-interactive entrypoint
- `ProtectHome=read-only` plus `/root`-biased `ReadWritePaths` means automated clean cannot touch the user's home
- `configureSchedule` is a no-op; daily vs weekly is only which timers get enabled
- `install.sh` prints “Installation complete!” even after uninstall or a failed TUI
- Config under sudo lands in `/root/.config/moonbit`

Lesson: advertised flags, units, and CLI must be the same program. Do not enable a user-session service from a root TUI without proving the unit's `User=` / `XDG` / `PATH` story.

### plex2jellyfin — richest guided setup, and the only real postmortem

Collect-then-execute wizard: Paths → Sonarr → Radarr → Jellyfin → AI → Permissions → Service → Web → Confirm → tasks → optional scan → post-scan plugin/systemd.

This is the closest analogue to “pick defaults, then install the suite.”

Steal:

- Consent toggles with “Enter through” yielding the full correct path
- Shared plugin-install **engine** reused by TUI/CLI/web (`internal/jellyfin/plugininstall`)
- Two-phase setup stamp: write config with `completed = false`, mark complete only after listeners start
- Honest complete screen driven by what the pipeline actually wrote (`serviceRunState`)
- Package postinstall that **refuses to start unconfigured services** and prints next steps
- goreleaser multi-binary + nFPM
- Staging + checksum + one-step rollback in the *plugin* engine (same idea as sysc-shell's plugin store)
- Regression tests after the 2026-07-12 incident (~47 installer tests)

The 2026-07-12 incident (`docs/audit-2026-07-12-post-rebrand-wizards.md`) is the most important failure in this family:

1. systemd units never written (post-scan gated on non-empty library paths)
2. `[setup] completed=true` stamped before services existed → web wizard would not recover
3. Plugin verify hit 412 because nothing was listening on :5522
4. Complete screen advertised units that were never installed
5. Lipgloss FG-only styles punched holes in the background under sudo / lost TrueColor

Also: Bubble Tea copies the model on every `Update`. Task goroutines must capture fields up front and write through pointers, or they mutate a dead copy.

Refuse: compile-on-target as the primary path; hand-rolled TOML that silently drops fields (use one typed emitter); gating service install on optional later steps; TUI-only with no `--yes`.

### smolbot — user-local done right, then grown too fat

The only installer that refuses root. Everything under `$HOME`: `~/.local/bin`, `~/.smolbot/config.json` mode 0600, systemd **user** unit with the installer's `PATH` captured (fnm/nvm).

Steal:

- User-local + systemd --user
- Token *files* referenced by path, not copied into config (Telegram/Discord)
- OAuth tokens in a separate 0600 file with atomic write
- Config backup on upgrade (`config.json.backup.<timestamp>`)
- Optional bundle that soft-fails (hybrid-memory needs Node; missing Node skips, core still installs)
- Binary replace: remove dest first to avoid “text file busy”
- Stop service / orphan processes before overwrite

Pitfalls:

- `install.sh` clones into `mktemp` and traps `rm -rf` on exit, then hybrid-memory config stores an absolute path into that temp tree
- Prerequisites screen does not block Continue when Go is missing
- `enableService` toggle still writes and enables the unit; only *start* is gated
- Welcome Uninstall is unreachable with ↓ (off-by-one `maxOptions`)
- Uninstall always deletes config/workspace; design's keep/purge choice never landed
- WhatsApp QR: Bubble Tea value-receiver + goroutine mutations + mutex deadlock. Never drive long interactive side-channels inside the installer model
- Channel/OAuth wizard belongs in `smolbot onboard`, not in the suite installer

Design docs here are unusually good (`2026-03-21-installer-design.md` and the UI-fix). Code drifted from them (no Reconfigure, no Retry/Skip/Abort).

### sysc-walls — closest component installer, still not composable

Screensaver, three binaries (`daemon`, `display`, `client`), user unit, default `daemon.conf`, ASCII assets.

Steal:

- `resolveUserHome` via `getent passwd` under sudo (not `/home/$SUDO_USER`)
- Atomic binary replace (`dst.new` then rename)
- Config keep-by-default with non-clobbering backups (`daemon.conf.backup`, `.backup.1`, …)
- Required vs optional: `import-environment WAYLAND_DISPLAY` is required; enable can skip
- Tests for the home/backup bugs that already shipped (#44/#45)

Pitfalls:

- No `--yes`, no library API — a suite cannot call this without a PTY
- Root + compile-on-target; needs Go and Wayland CGO headers
- GitHub Releases build with `CGO_ENABLED=0` while idle detection is CGO — release binaries likely degrade
- Config-exists prompt uses `os.UserHomeDir()` as root → `/root`, so the real user's config may never trigger Keep/Override
- Uninstall still uses `/home/$SUDO_USER` after install was fixed to `getent`
- ASCII “preserve existing art” was lost when inlined
- AUR installs to `/usr/bin` + `/usr/lib/systemd/user`; TUI installs to `/usr/local/bin` + `~/.config/systemd/user` — dual layouts fight
- `WAYLAND_DISPLAY` import fails off a graphical session (SSH/headless)
- Makefile omits client/unit/config that the TUI installs

Do not shell out to `go run cmd/installer` from a suite. Extract task functions into an importable package, or reimplement against a shared spec.

## Cross-cutting lessons

### 1. Split “put files on disk” from “configure the product”

plex2jellyfin packages write binaries + units and defer setup. smolbot's `onboard` is a second wizard. sysc-greet merges both and takes over the login stack.

SYSC should:

1. Install selected component binaries + user units
2. Seed XDG config (theme, bar plugins, wallpaper dirs) as a *user* identity
3. Enable/start only what was selected
4. Stamp setup-complete only after those units are actually up

Package postinstall must not start an unconfigured shell.

### 2. User-local is the default; root is a special case

sysc-shell already deploys to `~/.local/bin` with a stamp and a guard (`scripts/deploy`, `sysc-shell-guard`). clipboard's documented install is the same prefix + systemd --user.

Root TUI (Go, greet, moonbit, walls) exists because those tools historically copied into `/usr/local`. For a Niri user session, user units + `~/.local` match how the desktop actually runs. Privilege escalation, if any, is a later opt-in for system-wide installs — not the first path.

### 3. Do not compile on the user's machine as the hero path

Every TUI installer requires Go (and often git, meson, Node, CGO headers). That fails offline, on a fresh Arch laptop without `base-devel`, and it ignores stamped-release discipline.

Prefer: pinned GitHub release assets + SHA256, the same way the plugin store already installs plugins (`Download` + checksum + stage + rename, previous version kept as one-step rollback). Compile-from-source is a `--from-source` escape hatch for developers.

`scripts/deploy` already refuses dirty trees, unstamped binaries, and hand-copies. A public installer cannot use that script as-is (it is owner-machine specific), but it must not invent a second “copy a binary into `~/.local/bin`” story that the guard will undo.

### 4. One layout contract, written down

Every project has at least two layouts (TUI `/usr/local` vs AUR `/usr` vs user `~/.local`). Apps then search a pile of paths. sysc-Go's installer and its TUI disagree about `share/syscgo/assets`.

SYSC should pick **one** user-session layout and stick to it:

```
~/.local/bin/<component>
~/.config/<component>/
~/.config/systemd/user/<component>.service
~/.local/share/<component>/   # assets
~/.local/state/<component>/   # stamps, refused binaries
```

AUR/system packages, if they exist later, are a second distribution channel with `/usr` + `/usr/lib/systemd/user`, not a second TUI destination.

### 5. Guided defaults are a catalog, not a free-form editor

plex2jellyfin's plugin screen: curated list, recommended defaults Yes, consent toggles, verify after listeners exist.

sysc-shell already has:

- `config.Default()` bar: launcher, workspace, window-title | wordmark+clocks+media | running-apps, metrics group, clipboard, notifications
- Plugin store with catalog, SHA256 assets, staging, one-step rollback
- Theme: preset + matugen-from-wallpaper (`ThemeGen.Source = "wallpaper"`)
- Wallpaper dirs defaulting to `~/Pictures/wallpapers`
- Config preserves plugin entries even if the plugin is temporarily absent

The suite installer should **seed** that JSON (and enable catalog plugins), not grow a second bar editor. First-run “which plugins / which theme” is a thin overlay on `Default()` plus a short allow-list of catalog ids.

### 6. Honest UI

Recurring lies:

- Help says Cancel during install while Ctrl+C is ignored
- `enableService` does not gate unit install
- Complete screen claims systemd units that were never written
- `install.sh` always prints success
- Optional later steps gating required earlier work (plex2jellyfin libraries)

Rule: a toggle that does not change the task list is a bug. The complete screen reads *run state*, not *intent*.

### 7. Bubble Tea is fine for the wizard, hostile to side work

Known failure modes: model value copies, FG-only lipgloss holes under sudo, QR/OAuth/subprocess inside Update, 80×24 minimum, alt-screen hiding logs.

Keep the TUI as a collector of answers. Run install work as ordinary functions that return results. Log to a file *and* the task list. Do not put OAuth, QR, or compositor takeover in this binary.

### 8. Tests that actually caught bugs

What paid rent:

- plex2jellyfin: empty-libraries still write units; setup starts incomplete; TOML round-trip; honest complete screen; lipgloss nesting
- smolbot: 0600 perms, systemd PATH capture, optional skip
- sysc-walls: `resolveUserHome` + backup names
- sysc-Go unmerged: `--yes`, module root, PKGBUILD pin

What did not: “run the TUI in a VM” as documentation-only; zero tests (moonbit, greet, sysc-Go master).

Minimum for SYSC: dest paths, keep/override config, task gating vs toggles, setup-complete stamp, checksum verify, non-interactive `--yes` profile.

## Pitfall catalog (checklist for the design)

- [ ] `curl|bash` without a TTY and without `--yes`
- [ ] Clone `main` unpinned, no checksum of script or artifacts
- [ ] Temp clone deleted while config stores paths into it
- [ ] Compile-on-target as the only path
- [ ] Root TUI writing user XDG via `UserHomeDir()` → `/root`
- [ ] Dual `/usr` vs `/usr/local` vs `~/.local` with no documented winner
- [ ] Always-overwrite config; no backup; no keep-existing default
- [ ] Stamp setup-complete before services start
- [ ] Gate units/config on an optional later step
- [ ] Complete screen advertises work that skipped
- [ ] Advertised CLI flags that do not exist (moonbit `--force`)
- [ ] Interactive prompts on systemd ExecStart
- [ ] `pkill -f` / cmdline match that hits the agent shell (sysc-shell AGENTS.md)
- [ ] Hand-copy into `~/.local/bin/sysc-shell` that the deploy guard reverts
- [ ] CGO_ENABLED=0 release of a CGO daemon (sysc-walls idle)
- [ ] Session/DM takeover (greet)
- [ ] Lipgloss FG-only under sudo
- [ ] Uninstall that deletes user data with no keep/purge choice
- [ ] Uninstall that does not restore the previous display manager (greet)
- [ ] Enable-service toggle that still enables
- [ ] Prerequisites that do not block Continue
- [ ] No rollback of binaries (plugin store already knows how)

## What SYSC would actually put on a machine

Owner-named components, with current install stories:

| Component | Maturity | Today's install | Session role |
|---|---|---|---|
| **sysc-shell** | production on Niri | `scripts/deploy` → `~/.local/bin` + user unit + guard | bar, panels, plugins, wallpaper *client* |
| **sysc-clipboard** | shipped | `go build` → `~/.local/bin` + `contrib/` user unit | history daemon; shell panel is the UI |
| **sysc-terminal** | Stage 3 wallpaper engine | build only; shell selects it next to gSlapper | background layer, one process per output |
| **sysc-walls** | shipped screensaver | TUI installer / AUR | idle screensaver (Kitty), not wallpaper |
| **sysc-lock** | not finished | none | ext-session-lock; installer must skip or stub until the lock design's exit gate passes |

Dependencies the suite may need to *detect*, not necessarily install:

- Niri (required; other compositors need a separate approved design)
- `WAYLAND_DISPLAY` / `XDG_RUNTIME_DIR` imported into systemd --user (niri `--session` already does this; walls has been bitten when it does not)
- Secret Service for clipboard encryption (or a key file)
- gSlapper only if the user wants video wallpaper; it is an external process
- sysc-Go as a *library* of sysc-terminal, not a suite checkbox unless we want the CLI too
- Plugin catalog assets (checksummed) if the guided bar includes third-party plugins
- kitty: required by sysc-walls the screensaver, not by the shell

sysc-greet is a login greeter. It is not on the owner's list. Do not pull it into v1.

## What already exists that the suite must not reinvent

From sysc-shell:

- `internal/plugin/store`: download, SHA256, extract limits, stage, rename, previous version as rollback
- `config.Default()`: the default bar, theme preset, wallpaper dirs, matugen-from-wallpaper
- `packaging/systemd/sysc-shell.service` + deploy-guard drop-in
- Plugin entries survive a missing plugin (placeholder, not a lost layout)

From sysc-clipboard:

- User unit expecting `~/.local/bin/sysc-clipboard`
- `--check` for persistence/keyring health (installer can run this as verify)

From sysc-walls:

- Default `daemon.conf` + ASCII assets + user unit
- Keep-existing config prompt

From plex2jellyfin / smolbot:

- Collect-then-execute wizard
- Two-phase completion
- Optional components that cannot fail the core

## Recommended starting shape (not yet a design)

This is a bias for the conversation, not an approved architecture.

- New small Go binary (`sysc` or `sysc-install`), Charm TUI, same visual family
- **User-local**, Niri session, systemd --user
- Hero path: download **pinned release artifacts** with SHA256; `--from-source` for developers
- `--yes` profile that applies a documented default catalog (theme + bar plugins + component set)
- Component adapters as plain functions, not by exec'ing each project's TUI
- Seed sysc-shell JSON via the existing config types; do not fork a second schema
- sysc-lock is a checkbox that stays disabled until that product ships
- Uninstall: stop units, remove binaries, **ask** keep/purge config
- Never become greetd; never write niri's compositor config except an optional, consented snippet (spawn-at-startup vs user unit — shell packaging already prefers the user unit)

Open questions for the design conversation (do not answer them in this file):

1. Who is the first user (public Niri users vs owner machines vs Arch-first public)?
2. New repository vs a `cmd/sysc` in an existing tree?
3. Which guided questions are in v1 vs Settings-after-first-run?
4. How components are version-pinned (one suite release vs independently tagged products)?
5. Whether AUR meta-package is v1 or later?

## Sources (primary files)

Citations are in the explorer notes; the load-bearing ones:

- sysc-Go: `cmd/installer/main.go`, `install.sh`, `PKGBUILD`, unmerged `fix/installer-noninteractive-exit`
- sysc-greet: `cmd/installer/main.go`, `platform.go`, `scripts/postinstall.sh`, `install.sh`, `flake.nix`
- moonbit: `cmd/installer/main.go`, `systemd/moonbit-clean.service`, `internal/cli/root.go`
- plex2jellyfin: `cmd/installer/{types,tasks,validation,update}.go`, `docs/audit-2026-07-12-post-rebrand-wizards.md`, `docs/superpowers/specs/2026-07-12-tui-plugin-install-design.md`, `.goreleaser.yaml`
- smolbot: `cmd/installer/{main,tasks,hybrid_memory,theme}.go`, `docs/plans/2026-03-21-installer-{design,implementation,ui-fix}.md`, `install.sh`
- sysc-walls: `cmd/installer/main.go`, `cmd/installer/home_test.go`, `install.sh`, `systemd/sysc-walls-user.service`
- sysc-shell: `internal/config/config.go` `Default()`, `internal/plugin/store/install.go`, `packaging/systemd/README.md`, `AGENTS.md` deploy rules
