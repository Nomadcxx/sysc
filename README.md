<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="assets/brand/logos/sysc-inverse-transparent.svg">
    <img src="assets/brand/logos/sysc-primary-transparent.svg" alt="SYSC" height="180">
  </picture>
</p>

Guided installer for the SYSC desktop on [Niri](https://github.com/YaLTeR/niri).
Choose your desktop defaults, review existing bars and notification daemons,
then install the suite into your user account.

[Documentation](https://nomadcxx.github.io/sysc/docs/) · [Install](#install) · [Wizard guide](#wizard-guide) · [Sudo and install locations](#sudo-and-install-locations) · [Uninstall](#uninstall) · [Troubleshooting](#troubleshooting)

<p align="center">
  <img src="assets/tour.webp" alt="The SYSC desktop: Terminal Art, Settings and the control centre opening over a live terminal-art wallpaper" width="800"><br>
  <sub>The desktop the installer sets up, with <a href="https://github.com/Nomadcxx/sysc-shell">sysc-shell</a> over a live <a href="https://github.com/Nomadcxx/sysc-terminal">sysc-terminal</a> wallpaper. <a href="assets/tour.mp4">Full-quality video</a></sub>
</p>

## What you get

<table>
  <tr>
    <td align="center" valign="top"><a href="https://github.com/Nomadcxx/sysc-shell"><img src="assets/suite-shell.webp" alt="The sysc-shell control centre" width="300"></a><br><b><a href="https://github.com/Nomadcxx/sysc-shell">sysc-shell</a></b><br><sub>Bars, panels, launcher and settings</sub></td>
    <td align="center" valign="top"><a href="https://github.com/Nomadcxx/sysc-lock"><img src="assets/suite-lock.webp" alt="The sysc-lock lock screen with the fire effect" width="300"></a><br><b><a href="https://github.com/Nomadcxx/sysc-lock">sysc-lock</a></b><br><sub>Compositor-enforced lock screen</sub></td>
    <td align="center" valign="top"><a href="https://github.com/Nomadcxx/sysc-terminal"><img src="assets/suite-terminal.webp" alt="sysc-terminal fire effect as the desktop wallpaper" width="300"></a><br><b><a href="https://github.com/Nomadcxx/sysc-terminal">sysc-terminal</a></b><br><sub>Live terminal-art wallpaper</sub></td>
  </tr>
</table>

Plus notifications ([sysc-notify](https://github.com/Nomadcxx/sysc-notify)), clipboard history
([sysc-clipboard](https://github.com/Nomadcxx/sysc-clipboard)), a system tray
([sysc-tray](https://github.com/Nomadcxx/sysc-tray)) and
[16 official plugins](https://github.com/Nomadcxx/sysc-plugins). Screenshots use fixture data.

## Before you start

- Use an **Arch-family Linux system on x86_64/amd64**. The current suite
  rejects other distros and architectures, even though we publish an arm64
  installer binary.
- Have Niri configured at `~/.config/niri/config.kdl` and systemd user services
  available. Run the installer from a Niri session for immediate startup.
  Start Niri with `niri-session` so its systemd session target is active.
- Use a terminal of at least **80 × 24** and have internet access for downloads
  and weather lookup.
- Have `yay` or `paru` available if you want the installer to add gSlapper.
  Without either helper, it skips gSlapper and continues with the suite.

**Run as your normal user. Do not prefix the installer with `sudo`.**
The installer requests package-manager privileges when needed.

## Install

### Release binary: no Go required

Download the [latest release](https://github.com/Nomadcxx/sysc/releases/latest),
verify its checksum, and keep the installer for future reruns or uninstall:

```sh
download_dir=$(mktemp -d)
curl -fsSL https://github.com/Nomadcxx/sysc/releases/latest/download/sysc-linux-amd64 -o "$download_dir/sysc-linux-amd64" &&
curl -fsSL https://github.com/Nomadcxx/sysc/releases/latest/download/SHA256SUMS -o "$download_dir/SHA256SUMS" &&
(cd "$download_dir" && sha256sum -c --ignore-missing SHA256SUMS) &&
mkdir -p "$HOME/.local/bin" &&
install -m 755 "$download_dir/sysc-linux-amd64" "$HOME/.local/bin/sysc" &&
"$HOME/.local/bin/sysc"
```

Only continue after the checksum reports `sysc-linux-amd64: OK`. To choose a
specific release, replace `latest/download` in both URLs with
`download/v0.1.1`. Add `~/.local/bin` to your `PATH` to use `sysc` directly;
the commands below use its full path.

### Source one-liner: requires Go 1.26+ and git

```sh
curl -fsSL https://raw.githubusercontent.com/Nomadcxx/sysc/main/.install | sh
```

This builds current `main` in a temporary directory, opens the wizard when a
terminal is available, and removes the temporary build afterward. It does not
keep a `sysc` installer command. Without a terminal, it uses `--yes`.

For a source checkout you can keep and rerun:

```sh
git clone https://github.com/Nomadcxx/sysc.git
cd sysc
go build -p 2 -o sysc ./cmd/sysc
./sysc
```

## Wizard guide

Each page includes advice for the current choice. Use **Enter** to continue
and **Esc** to return to the previous page.

| Page | What to do |
|---|---|
| Theme | Use **↑/↓** to choose Standard, Compact, or Expressive. Use **←/→** for the desktop's light/dark mode. The installer stays pure black in either mode. |
| Wallpaper | Type a directory such as `~/Pictures/wallpapers`, then press **Enter**. Use an absolute path or a path beginning with `~/`. |
| Plugins | Use **↑/↓** to focus Weather or Media; **Space** toggles that checkbox independently. |
| Weather | Check the automatic location or type a city and press **Enter** to search. Once the location is correct, press **Enter** with the input empty to accept it. The built-in weather widget needs a location even if you untick the Weather plugin. |
| Conflicts | When another provider is detected, use **↑/↓** to select it and **←/→** to choose how to handle it. See [conflict choices](#conflict-choices). |
| Review | Check the component list, paths, and package-manager advice. Press **Enter** to begin installation. |

**PgUp/PgDn** scrolls long pages, task results, and warnings. **F9** cycles
English, German, French, and Simplified Chinese. **Ctrl+C** quits; **q** also
quits outside text-entry pages. Set `SYSC_REDUCED_MOTION=1` for a static banner.

If you already have a shell configuration, the installer preserves it. Wizard
defaults apply when creating a new configuration.

### Conflict choices

The installer detects providers such as mako, dunst, swaync, fnott, Waybar,
Noctalia, Quickshell shells, and DMS.

| Choice | Effect |
|---|---|
| Hand over | Stop the existing provider, disable its user unit where present, and comment its Niri startup lines so SYSC can take over. |
| Keep both | Leave the existing provider enabled. Review warnings about duplicate bars or competing notification daemons. |
| Skip SYSC component | Leave the corresponding SYSC service disabled and stopped. Its binary still downloads with the suite. |

Uninstall restores recorded startup lines and previously enabled units, plus
any notification activation file the installer displaced.

## Sudo and install locations

The suite runs as your user. gSlapper is a system package, so its package
manager may request your sudo password. The wizard hands over the terminal
for that prompt and resumes afterward. On Arch, yay/paru builds as your user
and elevates its package-manager step.

If gSlapper is already available on `PATH`, the installer leaves it alone.
If its package install fails or no compatible installer is available, the
result lists gSlapper as skipped; the rest of the suite can still install.
`--yes` skips the wizard but does not bypass sudo authentication.

| Location | Contents |
|---|---|
| `~/.local/bin` | Suite executables |
| `~/.config/systemd/user` | User service units |
| `~/.config/sysc-shell/config.json` | Desktop configuration; preserved if it exists |
| `~/.config/niri/sysc.kdl` | SYSC startup configuration included from Niri's config |
| `~/.local/state/sysc` | Install stamp and staging files; guided installer log |

Config, state, and data paths follow absolute `XDG_CONFIG_HOME`,
`XDG_STATE_HOME`, and `XDG_DATA_HOME` overrides. Binaries stay in `~/.local/bin`.

## What the current release installs

The embedded [suite pin](internal/pin/pin.json) records component versions and
checksums. Installer `v0.1.1` includes:

| Component | Version | Role |
|---|---|---|
| [sysc-shell](https://github.com/Nomadcxx/sysc-shell) | v0.1.0 | Desktop bar, panels, and plugins |
| [sysc-clipboard](https://github.com/Nomadcxx/sysc-clipboard) | v0.1.2 | Clipboard history |
| [sysc-walls](https://github.com/Nomadcxx/sysc-walls) | v1.0.2 | Idle screensaver |
| [sysc-notify](https://github.com/Nomadcxx/sysc-notify) | v0.1.0 | Notifications |
| [sysc-tray](https://github.com/Nomadcxx/sysc-tray) | v0.1.1 | StatusNotifierItem tray |
| [sysc-lock](https://github.com/Nomadcxx/sysc-lock) | v0.1.0 | Session lock owner |
| [gSlapper](https://github.com/Nomadcxx/gSlapper) | External package | Video wallpaper; install only when missing |

The installer does not yet install
[sysc-terminal](https://github.com/Nomadcxx/sysc-terminal); its suite entry is
disabled until a user unit is wired. sysc-greet is a separate greeter project.

The installer enables `sysc-lock-session.service`. Starting the service does
not lock your screen. The installer makes `sysc-lock` the shell's locker, in a
new shell config or an existing one that names no locker, and sets it to lock
after 5 minutes idle unless sysc-walls is installed too (the screensaver then
keeps idle; Settings → When idle switches between them). A locker you already
chose is left alone.

### Distro support

The complete suite currently supports Arch-family systems only. The gSlapper
package code also has checksum-pinned routes for Debian 13, Ubuntu 24.04,
Ubuntu 25.x, and Fedora 42+, using apt-get or dnf. Those routes prepare for
future suite support; they do not enable installation on those distros today.

| Platform | Status |
|---|---|
| Arch-family, x86_64 | Supported |
| AUR packages | Planned. Nothing is published to the AUR yet. |
| Debian 13, Ubuntu 24.04+ | Stub. Only the gSlapper package route exists. |
| Fedora 42+ | Stub. Only the gSlapper package route exists. |
| Other distributions and architectures | Not supported. The installer refuses to run. |

## Command-line installation

Skip the wizard and choose a weather city:

```sh
"$HOME/.local/bin/sysc" --yes --city "Melbourne"
```

Or supply both coordinates:

```sh
"$HOME/.local/bin/sysc" --yes --lat=-37.8136 --lon=144.9631
```

| Flag | Use |
|---|---|
| `--yes` | Use Standard, dark mode, `~/Pictures/wallpapers`, and recommended plugins. Without location flags, attempt a public-IP weather lookup. |
| `--city "Berlin"` | Look up a weather city; requires network access. |
| `--lat=… --lon=…` | Supply both weather coordinates without a location lookup. |
| `--lang en` | Choose `en`, `de`, `fr`, or `zh-Hans`. |
| `--keep-conflicts` | Keep existing providers. |
| `--handover=all` | Hand over detected notification, bar, and shell providers. |

By default, `--yes` hands over notification daemons and keeps bars and shells,
with warnings for kept conflicts. `--keep-conflicts` and `--handover=all` are
mutually exclusive. In a source checkout, substitute `./sysc` for the path.

## Uninstall

Remove suite binaries and user units, restore recorded handovers, and keep
your shell configuration:

```sh
"$HOME/.local/bin/sysc" uninstall
```

Add `--purge` to also remove the `sysc-shell` configuration directory.
Add `--remove-gslapper` to remove gSlapper **only if SYSC installed it**; its
package manager may request sudo. Add `--yes` to skip the uninstall confirmation.

If you used the source checkout, run `./sysc uninstall` there. If you used only
the source one-liner, download the installer as described above and run its
`uninstall` command instead of starting an installation.

## Troubleshooting

| Symptom | Next step |
|---|---|
| Root, distro, or architecture refusal | Run as your normal user on an Arch-family x86_64 system. Other suite targets are not enabled yet. |
| Missing Niri configuration | Configure Niri first and ensure `~/.config/niri/config.kdl` exists, accounting for your XDG config override. |
| Weather lookup fails | Retry a city search; for command-line installs, supply both `--lat` and `--lon`. |
| gSlapper is skipped | Read its task reason. Install yay/paru if missing, or resolve the package error and rerun. |
| Services enabled but not started | From SSH/TTY, log into Niri. If the installer reports an inactive session target, start Niri with `niri-session` and rerun. |
| A guided install fails | Read the final task results and `~/.local/state/sysc/installer.log`, or the corresponding XDG state path. |

To inspect the desktop shell service:

```sh
systemctl --user status sysc-shell.service
journalctl --user -u sysc-shell.service -b
```
