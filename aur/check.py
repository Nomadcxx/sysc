#!/usr/bin/env python3
"""Run after makepkg: python3 aur/check.py [recipe ...]. Never starts desktop services.

With no names it checks every recipe here; the shell's companion and first-run
checks run whenever sysc-shell is among them."""
import json
import shlex
import subprocess
import sys
import tempfile
from pathlib import Path

root = Path(__file__).resolve().parent
units = {}
names = sys.argv[1:]
packages = sorted(p.parent for p in root.glob('*/PKGBUILD') if not names or p.parent.name in names)
missing = set(names) - {p.name for p in packages}
assert packages and not missing, f'no such recipe: {sorted(missing)}'
for package in packages:
    name = package.name
    metadata = subprocess.check_output(['makepkg', '--printsrcinfo'], cwd=package, text=True)
    assert metadata == (package / '.SRCINFO').read_text(), f'{name}: stale .SRCINFO'
    archive, = [p for p in package.glob(f'{name}-*.pkg.tar.zst') if '-debug-' not in p.name]
    paths = subprocess.check_output(['bsdtar', '-tf', str(archive)], text=True).splitlines()
    assert all(p.startswith(('usr/', '.')) for p in paths), name
    assert not any(p.startswith('usr/local/') for p in paths), name
    assert '.INSTALL' in paths
    hook = subprocess.check_output(['bsdtar', '-xOf', str(archive), '.INSTALL'], text=True)
    assert hook == (package / f'{name}.install').read_text(), f'{name}: stale hook'
    assert '▄▀▀▀▀ █   █ ▄▀▀▀▀ ▄▀▀▀▀' in hook
    assert 'https://nomadcxx.github.io/sysc/docs/' in hook
    assert len(hook.split("<<'EOF'\n")[1].split('\nEOF')[0].strip().splitlines()) <= 7
    subprocess.run(['bash', '-n', str(package / 'PKGBUILD')], check=True)
    subprocess.run(['bash', '-n', str(package / f'{name}.install')], check=True)
    for path in paths:
        if path.endswith('.service'):
            content = subprocess.check_output(['bsdtar', '-xOf', str(archive), path], text=True)
            assert '%h/.local/bin' not in content
            assert 'ExecStart=/usr/bin/' in content
            if path.endswith('/sysc-compositor.service'):
                # The session itself: sysc-compositor-session starts it, and
                # it binds the graphical session rather than joining it.
                assert 'BindsTo=graphical-session.target' in content
            else:
                assert 'WantedBy=graphical-session.target' in content
            units[Path(path).name] = content
    staging = package / 'pkg' / name
    if name == 'sysc-plugins-git':
        plugins = list(staging.glob('usr/share/sysc-shell/plugins/*/manifest.json'))
        assert len(plugins) == 16
        for manifest in plugins:
            data = json.loads(manifest.read_text())
            binary = manifest.parent / data['exec']
            assert binary.is_file() and binary.stat().st_mode & 0o111, binary
        assert (staging / 'usr/share/sysc-shell/plugins/wallpaper-depth/depth_helper.py').is_file()
    else:
        binary = staging / 'usr/bin' / name.removesuffix('-git')
        assert binary.is_file() and binary.stat().st_mode & 0o111, binary
    print(f'{name}: archive, metadata, hook and layout OK')

checked = {package.name for package in packages}
if 'sysc-notify' in checked:
    assert 'Type=dbus\nBusName=org.freedesktop.Notifications' in units['sysc-notify.service']
if 'sysc-shell' not in checked:
    sys.exit(0)
shell = units['sysc-shell.service']
for name in ['sysc-notify.service', 'sysc-tray.service', 'sysc-clipboard.service', 'sysc-lock-session.service']:
    assert name in next(line for line in shell.splitlines() if line.startswith('Wants='))
    assert name in next(line for line in shell.splitlines() if line.startswith('After=sysc-'))

# Execute the packaged first-run commands in an isolated config directory.
sample = root / 'sysc-shell/config.example.json'
config = json.loads(sample.read_text())
assert config['session']['locker'] == 'sysc-lock' and config['tray']['enabled']
with tempfile.TemporaryDirectory(prefix='sysc-aur-') as temp:
    for existing in [False, True]:
        destination = Path(temp) / 'sysc-shell/config.json'
        if existing:
            destination.write_text('{"theme":{"preset":"compact"}}\n')
        before = destination.read_bytes() if existing else sample.read_bytes()
        for line in shell.splitlines():
            if line.startswith('ExecStartPre='):
                command = line.removeprefix('ExecStartPre=')
                args = [part.replace('%E', temp).replace('/usr/share/doc/sysc-shell/config.example.json', str(sample)) for part in shlex.split(command)]
                subprocess.run(args, check=True)
        assert destination.read_bytes() == before, 'first run overwrote an existing configuration'
print('First-run config creation, preservation and companion ordering OK')
