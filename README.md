<div align="center">
  <img src="assets/banner.svg?v=2" alt="SYSC">
</div>

Guided installer for the SYSC desktop on [Niri](https://github.com/YaLTeR/niri), written in Go with the Bubble Tea framework.

One binary installs a tested set of components into the user session
(`~/.local/bin`, systemd --user) and walks first-run defaults: theme, wallpaper
engine, recommended bar plugins, and weather location.

## Installation

### Quick Install

Once the first release exists, the one-liner is:

```sh
curl -fsSL https://github.com/Nomadcxx/sysc/releases/latest/download/sysc-linux-amd64 -o /tmp/sysc
chmod +x /tmp/sysc && /tmp/sysc
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
| [gSlapper](https://github.com/Nomadcxx/gslapper) | Video wallpaper, only if missing |
| [sysc-lock](https://github.com/Nomadcxx/sysc-lock) | Session lock, when it ships |

Niri first. Arch-family distros in v1; others are detected and refused with a
named message. sysc-lock stays out until that product is finished.

## Flags

`--yes` installs with defaults, `--city` or `--lat`/`--lon` set the weather
location without prompts, and `--lang` picks the installer language (`en`,
`zh-Hans`, `de`, `fr`). Without a location flag, `--yes` guesses from the
network; if that fails it refuses and names the flags.

## Status

Not ready to run. Do not curl this repository until the first release exists.
