# SYSC suite installer — design

Date: 2026-10-05.
Approved in conversation. Research: `2026-10-05-sysc-suite-installer-research.md`.

This is the contract for `github.com/Nomadcxx/sysc` (checkout `/home/nomadx/sysc`). It is not an implementation plan.

## Product

SYSC is a guided installer for public Niri users. One GitHub release of this repo is a **tested pin set**: it ships the installer binary plus exact tags/assets for the SYSC components that pin enables, and the gSlapper package used when gSlapper is missing. A component row may be `disabled` with a reason — sysc-lock until it ships, sysc-terminal until it has release assets. The installer skips a disabled component with a named reason instead of failing.

It installs a user session, not a display manager. It does not vendor those projects, does not exec each component’s existing TUI, and does not become greetd.

## Decisions

| Topic | Choice |
|---|---|
| Audience | Public Niri users; independent of owner `scripts/deploy` |
| Shape | Dedicated repo; one static Go binary |
| Versions | One SYSC release pins a tested combination |
| Suite | Full set when the pin enables it: shell, clipboard, terminal, walls. Disabled rows are skipped by name. gSlapper if missing |
| Prefix | User-local (`~/.local/bin`, systemd --user) |
| Distro v1 | Arch family installs; other distros are detected and **named-refused** |
| Already installed | SYSC-owned components brought to this pin; gSlapper left alone if present |
| Wizard | Short catalog: theme, wallpaper engine, recommended plugins, weather |
| Weather | Required. IP guess or Open-Meteo city search. Builtin bar widget. GeoClue deferred (needs a D-Bus dependency; absent on minimal Niri installs) |
| Languages | Installer UI: `en`, `zh-Hans`, `de`, `fr`. Not session locale |
| Fetch | Download pinned GitHub assets + SHA256. No compile-on-target in v1 |
| Dependencies | bubbletea v1.3.4, bubbles v0.21.0, lipgloss v1.1.0, x/text v0.23.0 — greet-family parity; v2 betas avoided |
| gSlapper later | Pin file has per-distro package slots; v1 fills Arch only |
| Niri | Own sidecar `sysc.kdl` + `include`; never rewrite whole `config.kdl`; never touch `sysc-shell.kdl` (theming) |
| Units | systemd --user; stop → swap → enable; start only inside an active graphical session |
| Backups | First copy of every user file we mutate or replace, never overwritten on later runs |

## Layout

On an Arch-family Niri session:

```
~/.local/bin/<component>
~/.config/systemd/user/<component>.service
~/.config/<component>/          # XDG config, never /root
~/.local/share/<component>/     # assets when a component needs them
~/.config/niri/sysc.kdl            # SYSC-owned session sidecar (not theming)
~/.local/state/sysc/installed.json
~/.local/state/sysc/installer.log
~/.local/state/sysc/backups/       # timestamped copies of files we mutated
```

gSlapper, when missing, comes from the Arch-family package named in the pin (pacman / AUR helper). System prefix is allowed for that foreign dependency. When SYSC expands past Arch, the same pin slot names the gSlapper packages already built for those distros.

User units order after `graphical-session.target`. A missing `WAYLAND_DISPLAY` at install time (SSH) does not fail the install. Niri `--session` already exports compositor env into systemd --user.

Public users do **not** get owner `scripts/deploy` or `sysc-shell-guard`. SYSC’s stamp is how a later SYSC release knows what it installed.

## Pin file

Shipped inside the installer (embed) and published next to the release binary.

Each SYSC-owned component row: id, git tag, user-unit template name, and either `disabled` with a reason or a `binaries` list. Each binary names its install name and a per-arch map of asset URL + SHA256. v1 requires an amd64 asset; arm64 is optional per binary, and the installer refuses an unsupported host arch by name. A component may ship more than one binary (sysc-walls ships daemon, display, client; the unit runs the daemon, so the pin installs the daemon and may omit the rest).

The first pin can ship with `sysc-terminal` disabled (no release yet) and any component whose release assets are missing. The installer skips disabled rows with a named reason, and the wizard hides options that need a disabled component (the terminal-effects wallpaper engine).

gSlapper: package name/version per distro family; Arch filled in v1 (`gslapper` 1.5.1, AUR). sysc-lock: `disabled`. Recommended plugin catalog ids for this release live here too (snapshot of `sysc-plugins` at pin time).

Component release assets are cross-repo gates, tracked as bd issues in sysc-shell: sysc-shell needs a release workflow and first tag; sysc-clipboard needs a release workflow (it has tags but no assets); sysc-walls has a release workflow but no published release and amd64-only assets; sysc-terminal has neither code nor release.

## Wizard (collect, then execute)

Preflight before or as screen 0: not root, Niri session, Arch-family. Failure is a named stop.

1. **Theme** — preset `standard` / `compact` / `expressive`, dark/light. Default: `standard` + dark, `ThemeGen.Source = "wallpaper"`.
2. **Wallpaper** — one default engine: stills, gSlapper, or terminal effects. Stills directory seeded (`~/Pictures/wallpapers` unless changed). Other engines stay installed; Settings can switch later.
3. **Plugins** — recommended catalog ids, toggles, default **on**. Bar layout stays `config.Default()`. Not a bar editor.
4. **Weather** — required. IP guess they can accept, or Open-Meteo geocode (same provider the shell already uses). Writes `weather.latitude` / `longitude` / place label (≤80 bytes) and **adds the builtin `weather` item to the default bar**. Empty location cannot finish. GeoClue is a named follow-up, not v1.
5. **Confirm**, then the task list.

`--yes` uses those defaults. Weather cannot be skipped: `--city`, `--lat`/`--lon`, or a successful guess. Otherwise non-interactive install refuses.

## UI / UX

This is a sysc-family installer, not a plain form. The chrome matches **sysc-greet’s installer** first, with the greeter’s bottom help bar as the nav model.

**Banner.** The eight-line SYSC block lettering from greet’s installer (`asciiHeaderLines` in `sysc-greet/cmd/installer/main.go`), including the `SEE YOU IN SPACE COWBOY` underline. Rendered with **BeamsTextEffect** (monochrome beam wipe, ~50 ms tick), full-width alt screen, rounded lipgloss box for the page body. Tagline stays English (brand), not translated.

**Palette.** Monochrome: dark base, white primary, muted grey secondary. Lipgloss styles set **foreground and background** together (plex2jellyfin TrueColor/sudo hole). Minimum terminal 80×24; smaller gets a “enlarge the terminal” page, not a broken layout.

**Page copy.** Every screen has a title, a short “what this is,” the control, and a one-line consequence (as greet describes each compositor). Plugins say Settings can change them later. Weather says the bar widget needs a place.

**Bottom nav.** Always visible, greet-style: keys that actually work on this screen, plus Back / Next / Quit, plus language. Example: `↑↓ Navigate • Enter Next • Esc Back • F9 Language • q Quit`. During the task list the bar says `Please wait` — Esc does not cancel mid-swap. After a hard fail: Enter to exit, log path shown. Toggles that do not change the task list are bugs.

**Motion.** Beams on the banner; spinner on the current task (`bubbles/spinner`). Do not port greet’s ASCII-1..4 greeter border skins or typewriter session titles into v1.

## Languages

Installer UI only. Catalogs: `en`, `zh-Hans`, `de`, `fr`. Start from `LANG` / `LC_MESSAGES` (`zh*` → Simplified, `de*`, `fr*`, else English). F9 cycles on every screen. String ids, not sprintf soup. Every id exists in all four catalogs (tested). CJK width through lipgloss. UTF-8 required.

Does not set the Niri/session locale.

## Download and “already installed”

Hero path: download the pin, never `go build` on the machine.

For each SYSC-owned component:

1. `disabled` in the pin → skip with the named reason.
2. Read stamp + on-disk `--version` if the binary speaks it (none do today; the stamp is the source of truth).
3. Already at this pin → skip.
4. Present but not this pin, or missing → stop user unit if running, download **all** remaining assets first, verify SHA256, stage, atomic `dst.new` → rename into `~/.local/bin`, keep `dst.bak` until stamp, write/enable user unit with `ExecStart` at that path.
5. Start units only after every binary is in place.

gSlapper: if on `PATH`, skip. Else install the pinned AUR package through an AUR helper (`yay`/`paru`); no helper → SKIP naming the package, never build from source. Never replace an existing gSlapper.

Stamp after the selected units are installed and enabled — never before enable. The stamp records the SYSC release, each component version, whether *this* run installed gSlapper, and whether the units were started. From SSH/TTY the units are enabled but not started (`started: false`); the complete screen says to log into Niri, and the next run sees the stamp and offers Update.

## Niri config

Do not rewrite `~/.config/niri/config.kdl`. Missing config → named refuse (Niri is present but unconfigured).

The shell already writes `sysc-shell.kdl` for theme colours (`include "sysc-shell.kdl"`). SYSC must not use that path.

SYSC owns `~/.config/niri/sysc.kdl` (marker comment: generated by SYSC, do not edit). Append `include "sysc.kdl"` to `config.kdl` if absent (idempotent, atomic write). Uninstall removes that include and the sidecar only.

The sidecar may hold recommended ipc binds and an optional panel `layer-rule`. It must **not** contain `spawn-at-startup` for the shell, walls, or gSlapper.

In `config.kdl` itself, the only allowed edit besides the include is **commenting out** `spawn-at-startup` lines that launch `sysc-shell` (otherwise systemd and Niri start two shells). Leave cliphist, polkit, gsettings, and everything else.

Binds: add a recommended ipc bind only when that key is not already bound in `config.kdl` or its includes. Skip with a log line. Never steal an existing `Mod+Space` / `Mod+Comma` / etc.

## systemd user units

Unit templates are embedded in the installer (`go:embed`), `ExecStart=%h/.local/bin/...`, `WantedBy=graphical-session.target`. Do not copy clipboard’s `WantedBy=default.target`. The walls unit runs `sysc-walls-daemon` from `~/.local/bin`, not the repo’s `/usr/local/bin` unit.

Never `pkill -f`.

Replace a running session:

1. If `sysc-shell.service` is active, `systemctl --user stop` it first (it owns wallpaper children; `KillMode=mixed`, `TimeoutStopSec=5`).
2. Stop walls, then clipboard, if active.
3. Swap binaries (`.bak` until stamp).
4. Write unit files (backup first if replacing a unit we did not write).
5. `daemon-reload`, then `enable`.
6. `start` only if `graphical-session.target` is active. From SSH/TTY: enable only; complete screen says log into Niri.
7. Start order: clipboard → walls → shell. If shell does not stay up, keep binaries, no stamp, point at the journal.

## Backups

Every user file we mutate or replace is copied **before** the first SYSC write. The first backup is sacred (same rule as shell theming `ApplyWriteForce`): later runs must not overwrite it.

| File | When | Where |
|---|---|---|
| `~/.config/niri/config.kdl` | Before the first include-line or spawn-comment edit | Sibling `config.kdl.sysc.bak` **and** `~/.local/state/sysc/backups/niri-config.kdl.<rfc3339>` |
| Existing `sysc.kdl` we did not generate | Before replace | `sysc.kdl.sysc.bak` |
| Existing systemd user unit we did not write | Before replace | `<unit>.sysc.bak` next to it |
| Existing `sysc-shell/config.json` | **Never overwritten on update.** Seed only if absent. If a future force-seed exists, first backup as above | — |
| `sysc-walls` `daemon.conf` | **Not written in v1** — the daemon has built-in defaults when the file is absent. If a later pin writes it: keep-by-default, override → numbered `.backup`, `.backup.1` | beside the conf |
| Binaries | `dst.bak` until stamp, as already specified | `~/.local/bin` |

Timestamped copies under `~/.local/state/sysc/backups/` on every run that mutates `config.kdl`, so a later edit still has a trail. Rotate to a small cap (five), never delete `*.sysc.bak`.

Uninstall keep: remove include + sidecar + units; leave `config.kdl` (spawn comments stay). Optional “restore niri from `config.kdl.sysc.bak`” is a separate confirm, off by default — the file may be older than the user’s later edits.

## Failure, rollback, uninstall

- Checksum mismatch or download failure → no writes under `~/.local`.
- Mid-swap failure → restore `.bak` for anything already replaced.
- Unit start failure → binaries stay; no stamp; complete screen names the unit and the command.
- gSlapper package failure → SKIP; suite can still finish.
- Log: `~/.local/state/sysc/installer.log`.
- Complete screen is driven by run state, not intent.

Uninstall (welcome: Install / Uninstall; Update when a stamp exists): stop units we wrote, remove stamped binaries and those user units, then **keep or purge** XDG config. Remove gSlapper only if the stamp says we installed it, and only after yes. Foreign gSlapper stays.

## One-liner

When a release exists:

```bash
curl -fsSL https://github.com/Nomadcxx/sysc/releases/latest/download/sysc-linux-amd64 -o /tmp/sysc
chmod +x /tmp/sysc && /tmp/sysc
```

`install.sh` on `main` is a fetcher only: arch detect, SHA256 of that release, exec. No clone, no Go, no sudo. Non-TTY implies `--yes` and the weather-flag rule. Until the first tag, README stays “do not curl this yet.”

## Tests

Table tests, one package, named `-run`. Never `go test ./...` or `-race`.

Pin completeness (including a disabled row and a multi-binary row); checksum refuse; download-all-then-swap restores `.bak`; stamp only after enable and records `started`; disabled component is skipped by name; `--yes` without weather fails; IP guess resolves a stub endpoint; uninstall keep/purge; gSlapper removal gated on stamp; Arch proceeds / Fedora|Debian named refusal; all string ids in four catalogs; `LANG=zh_CN.UTF-8` → zh-Hans; unknown LANG → en; nav strings follow the language switch; niri include is idempotent and does not touch `sysc-shell.kdl`; spawn-at-startup of sysc-shell is commented not deleted; binds skip occupied keys; first `*.sysc.bak` is not overwritten on a second run; seed does not replace an existing shell `config.json`.

## Out of scope (v1)

sysc-lock install; non-Arch package install; compile-from-source; AUR meta-package; bar editor; session locale; greetd; owner deploy-guard; replacing an existing gSlapper; GeoClue guess (IP only in v1); installing walls display/client binaries.
