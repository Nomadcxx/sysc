# SYSC

Guided installer for the SYSC desktop on [Niri](https://github.com/YaLTeR/niri).

Not ready to run. This repository will ship one binary that installs a tested
set of components into the user session (`~/.local/bin`, systemd --user) and
walks first-run defaults: theme, wallpaper engine, recommended bar plugins, and
weather location.

## What it will install

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

## Install

Once the first release exists, the intended one-liner is:

```sh
curl -fsSL https://github.com/Nomadcxx/sysc/releases/latest/download/sysc-linux-amd64 -o /tmp/sysc
chmod +x /tmp/sysc && /tmp/sysc
```

`install.sh` in this repository is the same fetcher: it detects the
architecture, verifies the release checksum, and runs the installer. A
non-interactive shell runs it with `--yes`.

Flags: `--yes` installs with defaults, `--city` or `--lat`/`--lon` set the
weather location without prompts, and `--lang` picks the installer language
(`en`, `zh-Hans`, `de`, `fr`). Without a location flag, `--yes` guesses from
the network; if that fails it refuses and names the flags.

## Status

Not ready to run. Do not curl this repository until the first release exists.

