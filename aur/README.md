# AUR recipes

This directory is the tracked source for the suite's AUR packages. Each
subdirectory is one AUR package, exactly as it is pushed to
`ssh://aur@aur.archlinux.org/<name>.git`.

| Package | Builds from |
|---|---|
| `sysc-compositor` | the `v${pkgver}` tag tarball |
| `sysc-compositor-git` | `main` of Nomadcxx/sysc-compositor |

The suite's other AUR packages are not tracked here yet (sysc-1705).

## Checking a recipe

Build it, then run the checker on what was built:

```sh
cd aur/sysc-compositor-git && makepkg -s && cd ../..
python3 aur/check.py sysc-compositor-git
```

`check.py` with no names checks every recipe here. It compares `.SRCINFO`
with `makepkg --printsrcinfo` and checks each package's install hook, its
`/usr` layout, its systemd units and its binary. It never starts a service.

A change to `PKGBUILD` needs `.SRCINFO` regenerated with
`makepkg --printsrcinfo > .SRCINFO`.

## Releasing `sysc-compositor`

Until the tag exists the stable recipe carries `sha256sums=('SKIP')`. After
tagging `v${pkgver}`:

1. Replace `SKIP` with the tag tarball's checksum (`updpkgsums`).
2. Regenerate `.SRCINFO`, build, and run the checker.
3. Push the package to the AUR (an owner step).
