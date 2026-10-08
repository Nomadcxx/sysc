<div align="center">
  <img src="assets/banner.svg?v=2" alt="SYSC">
</div>

Guided installer for the SYSC desktop on [Niri](https://github.com/YaLTeR/niri), written in Go with the Bubble Tea framework.

One binary installs a tested set of components into the user session
(`~/.local/bin`, systemd --user) and walks first-run defaults: theme, wallpaper
directory, recommended bar plugins, and weather location.

## Installation

### Quick Install

Once the first release exists, the one-liner is:

```sh
f=$(mktemp); trap 'rm -f "$f"' EXIT; curl -fsSL https://github.com/Nomadcxx/sysc/releases/latest/download/sysc-linux-amd64 -o "$f" && chmod 700 "$f" && "$f"
```

`install.sh` in this repository is the same fetcher: it detects the
architecture, verifies the release checksum, and runs the installer. A
non-interactive shell runs it with `--yes`.

### Build from Source

Build and run the installer from source with one line:

```sh
curl -fsSL https://raw.githubusercontent.com/Nomadcxx/sysc/main/.install | sh
```

Or clone and run it yourself:

```sh
git clone https://github.com/Nomadcxx/sysc
cd sysc
go run ./cmd/sysc
```

## What it installs

| Component | Role |
|---|---|
| [sysc-shell](https://github.com/Nomadcxx/sysc-shell) | Desktop shell (bar, panels, plugins) |
| [sysc-clipboard](https://github.com/Nomadcxx/sysc-clipboard) | Clipboard history daemon |
| [sysc-terminal](https://github.com/Nomadcxx/sysc-terminal) | Terminal-effect wallpaper engine |
| [sysc-walls](https://github.com/Nomadcxx/sysc-walls) | Idle screensaver (not wallpaper) |
| [sysc-notify](https://github.com/Nomadcxx/sysc-notify) | Notifications daemon (enabled when its first release ships) |
| [sysc-tray](https://github.com/Nomadcxx/sysc-tray) | StatusNotifierItem tray (enabled when its first release ships) |
| [gSlapper](https://github.com/Nomadcxx/gslapper) | Video wallpaper, only if missing |
| [sysc-lock](https://github.com/Nomadcxx/sysc-lock) | Session lock, when it ships |

Niri first. Arch-family distros in v1; others are detected and refused with a
named message. sysc-lock stays out until that product is finished.

## Conflicts and handover

An install that finds an existing provider (mako, dunst, swaync, fnott,
waybar, Noctalia, Quickshell shells, DMS) offers a handover on a Conflicts
wizard page: **Hand over**, **Keep both**, or **Skip SYSC component**. Handing
over stops and disables the provider and comments its niri
`spawn-at-startup` line with a `// sysc-handover: ` marker; each step is
recorded in the install stamp before it runs. When sysc-notify is enabled,
SYSC also writes
`$XDG_DATA_HOME/dbus-1/services/org.freedesktop.Notifications.service` so a
packaged daemon cannot win the bus name.

`--yes` hands over notification daemons and keeps bars and shells (with a
warning per kept conflict); `--keep-conflicts` keeps everything;
`--handover=all` hands everything over. The last two are mutually exclusive.

`sysc uninstall` reverses exactly what the stamp recorded: restores commented
lines byte-for-byte, re-enables units that were enabled before, and puts back
a displaced activation file.

## Flags

`--yes` installs with defaults, `--city` or `--lat`/`--lon` set the weather
location without prompts, and `--lang` picks the installer language (`en`,
`zh-Hans`, `de`, `fr`). Without a location flag, `--yes` guesses from the
network; if that fails it refuses and names the flags. `--keep-conflicts` and
`--handover=all` control conflict handover as described above.

## Status

Not ready to run. Do not curl this repository until the first release exists.
