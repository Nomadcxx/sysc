package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Nomadcxx/sysc/internal/distro"
	"github.com/Nomadcxx/sysc/internal/fetch"
	"github.com/Nomadcxx/sysc/internal/i18n"
	"github.com/Nomadcxx/sysc/internal/install"
	"github.com/Nomadcxx/sysc/internal/pin"
)

// These targets match gSlapper's release builds and documented compatibility.
// A derivative's VERSION_ID does not identify its parent release.
func gslapperPlatform(osRelease []byte) (string, error) {
	id, like := distro.ParseOSRelease(osRelease)
	version := ""
	for _, line := range strings.Split(string(osRelease), "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if ok && key == "VERSION_ID" {
			version = strings.Trim(value, `"'`)
		}
	}
	major, _ := strconv.Atoi(strings.Split(version, ".")[0])
	switch id {
	case "fedora":
		if major >= 42 {
			return "fedora42", nil
		}
	case "debian":
		if major == 13 {
			return "debian13", nil
		}
	case "ubuntu":
		if version == "24.04" {
			return "ubuntu24.04", nil
		}
		if major == 25 {
			return "debian13", nil
		}
	default:
		if distro.Family(id, like) == nil {
			return "arch", nil
		}
	}
	return "", fmt.Errorf("gSlapper has no verified package target for %s %s", id, version)
}

func gslapperManager(osRelease []byte) (string, error) {
	id, like := distro.ParseOSRelease(osRelease)
	tool := ""
	switch id {
	case "debian", "ubuntu":
		tool = "apt-get"
	case "fedora":
		tool = "dnf"
	default:
		if distro.Family(id, like) == nil {
			if helper := aurHelper(); helper != "" {
				return helper, nil
			}
			return "", fmt.Errorf("gSlapper needs yay or paru on Arch-family systems")
		}
		return "", fmt.Errorf("no gSlapper package installer for distribution %q", id)
	}
	for _, required := range []string{tool, "sudo"} {
		if _, err := exec.LookPath(required); err != nil {
			return "", fmt.Errorf("gSlapper requires %s: %w", required, err)
		}
	}
	return tool, nil
}

func gslapperAsset(osRelease []byte, p pin.Pin, arch string) (pin.Asset, error) {
	target, err := gslapperPlatform(osRelease)
	if err != nil {
		return pin.Asset{}, err
	}
	asset, ok := p.GSlapper.Assets[target][arch]
	if !ok {
		return pin.Asset{}, fmt.Errorf("no pinned gSlapper package for %s/%s", target, arch)
	}
	return asset, nil
}

// Both the CLI and TUI inject the same command runner. The TUI runner hands
// the terminal to Bubble Tea's ExecProcess for the native sudo prompt.
func wirePackages(opts *install.Options, osRelease []byte, arch string, run func(*exec.Cmd) error) {
	manager, managerErr := gslapperManager(osRelease)
	opts.InstallPkg = func(pkg string) error {
		if managerErr != nil {
			return managerErr
		}
		if manager == "yay" || manager == "paru" {
			return run(packageCommand(manager, "-S", pkg, opts.Yes))
		}
		asset, err := gslapperAsset(osRelease, opts.Pin, arch)
		if err != nil {
			return err
		}
		parsed, err := url.Parse(asset.URL)
		if err != nil {
			return err
		}
		name := filepath.Base(parsed.Path)
		extension := ".deb"
		if manager == "dnf" {
			extension = ".rpm"
		}
		if !strings.HasSuffix(name, extension) {
			return fmt.Errorf("gSlapper package %q must be %s", name, extension)
		}
		dir, err := os.MkdirTemp("", "sysc-gslapper-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(dir)
		if err := fetch.DownloadAll(context.Background(), opts.Client, dir, []fetch.Asset{{Name: name, URL: asset.URL, SHA256: asset.SHA256}}); err != nil {
			return err
		}
		path := filepath.Join(dir, name)
		// Public release packages can be read by apt's _apt sandbox user. Other
		// users cannot modify the directory or the verified package.
		if err := os.Chmod(path, 0o644); err != nil {
			return err
		}
		if err := os.Chmod(dir, 0o755); err != nil {
			return err
		}
		return run(nativePackageCommand(manager, "install", path, opts.Yes))
	}
	opts.RemovePkg = func(pkg string) error {
		if managerErr != nil {
			return managerErr
		}
		if manager == "yay" || manager == "paru" {
			return run(packageCommand(manager, "-Rns", pkg, opts.Yes))
		}
		// Removal uses the recorded ownership gate in install.Uninstall, and does
		// not need a compatible release asset or a fresh download.
		return run(nativePackageCommand(manager, "remove", pkg, opts.Yes))
	}
	opts.CJKFont = cjkFontPackage(manager)
	opts.InstallSystemPkg = func(pkg string) error {
		if managerErr != nil {
			return managerErr
		}
		if manager == "yay" || manager == "paru" {
			return run(packageCommand(manager, "-S", pkg, opts.Yes))
		}
		return run(nativePackageCommand(manager, "install", pkg, opts.Yes))
	}
	opts.HasCJKFont = cjkFontPresent
}

// cjkFontPackage maps the detected package manager to the distro's Noto CJK
// package; one package covers zh/ja/ko glyphs. "" means no known package.
func cjkFontPackage(manager string) string {
	switch manager {
	case "apt-get":
		return "fonts-noto-cjk"
	case "dnf":
		return "google-noto-sans-cjk-ttc-fonts"
	case "yay", "paru":
		return "noto-fonts-cjk"
	}
	return ""
}

// cjkFontPresent asks fontconfig whether any installed font covers the
// locale script. ponytail: fc-list missing counts as absent, so the
// distro package install pulls fontconfig itself.
func cjkFontPresent(locale i18n.Locale) bool {
	lang := map[i18n.Locale]string{i18n.JA: "ja", i18n.KO: "ko", i18n.ZH: "zh"}[locale]
	if lang == "" {
		return true
	}
	out, err := exec.Command("fc-list", ":lang="+lang).Output()
	return err == nil && len(strings.TrimSpace(string(out))) > 0
}

func nativePackageCommand(manager, action, pkg string, yes bool) *exec.Cmd {
	args := []string{manager, action}
	if yes {
		args = append(args, "-y")
	}
	return exec.Command("sudo", append(args, "--", pkg)...)
}
