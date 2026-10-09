package main

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Nomadcxx/sysc/internal/install"
	"github.com/Nomadcxx/sysc/internal/pin"
)

func TestGSlapperPackagePlatform(t *testing.T) {
	for _, tc := range []struct{ release, target string }{
		{"ID=arch", "arch"},
		{"ID=garuda\nID_LIKE=arch", "arch"},
		{"ID=fedora\nVERSION_ID=42", "fedora42"},
		{"ID=fedora\nVERSION_ID=43", "fedora42"},
		{"ID=debian\nVERSION_ID=13", "debian13"},
		{"ID=ubuntu\nID_LIKE=debian\nVERSION_ID=24.04", "ubuntu24.04"},
		{"ID=ubuntu\nVERSION_ID=25.10", "debian13"},
		{"ID=debian\nVERSION_ID=12", ""},
		{"ID=fedora\nVERSION_ID=41", ""},
		{"ID=ubuntu\nVERSION_ID=22.04", ""},
		{"ID=ubuntu\nVERSION_ID=26.04", ""},
		{"ID=linuxmint\nID_LIKE=\"ubuntu debian\"\nVERSION_ID=22", ""},
		{"ID=fedora", ""},
		{"ID=nixos", ""},
	} {
		target, err := gslapperPlatform([]byte(tc.release))
		if target != tc.target || (err != nil) != (tc.target == "") {
			t.Fatalf("%s: target=%q err=%v", tc.release, target, err)
		}
	}
}

func TestNativePackageRequiresVerifiedAsset(t *testing.T) {
	// Fake only command execution. Real HTTP and checksum verification run.
	bin := t.TempDir()
	for _, tool := range []string{"sudo", "apt-get", "dnf", "yay"} {
		if err := os.WriteFile(filepath.Join(bin, tool), []byte("#!/bin/sh\nexit 99\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)
	payload := []byte("a pinned package")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(payload) }))
	defer server.Close()
	for _, tc := range []struct{ id, target, tool, file string }{
		{"debian\nVERSION_ID=13", "debian13", "apt-get", "gslapper.deb"},
		{"ubuntu\nVERSION_ID=24.04", "ubuntu24.04", "apt-get", "gslapper.deb"},
		{"fedora\nVERSION_ID=42", "fedora42", "dnf", "gslapper.rpm"},
	} {
		for _, valid := range []bool{true, false} {
			hash := fmt.Sprintf("%x", sha256.Sum256(payload))
			if !valid {
				hash = strings.Repeat("0", 64)
			}
			opts := install.Options{Pin: pin.Pin{GSlapper: pin.GSlapper{Package: "gslapper", Assets: map[string]map[string]pin.Asset{tc.target: {"amd64": {URL: server.URL + "/" + tc.file, SHA256: hash}}}}}}
			calls := 0
			staged := ""
			wirePackages(&opts, []byte("ID="+tc.id), "amd64", func(cmd *exec.Cmd) error {
				calls++
				if cmd.Args[0] != "sudo" || cmd.Args[1] != tc.tool || cmd.Args[2] != "install" {
					t.Fatalf("command=%v", cmd.Args)
				}
				staged = cmd.Args[len(cmd.Args)-1]
				data, err := os.ReadFile(staged)
				if err != nil || string(data) != string(payload) {
					t.Fatalf("unverified/missing package: %q %v", data, err)
				}
				return nil
			})
			err := opts.InstallPkg("gslapper")
			if valid && (err != nil || calls != 1) {
				t.Fatalf("valid package: calls=%d err=%v", calls, err)
			}
			if !valid && (err == nil || calls != 0) {
				t.Fatalf("bad checksum executed a privileged command: calls=%d err=%v", calls, err)
			}
			if staged != "" {
				if _, err := os.Stat(staged); !os.IsNotExist(err) {
					t.Fatal("staged package was not cleaned up")
				}
			}
		}
	}
}

func TestNativeRemovalAndUnavailablePackages(t *testing.T) {
	bin := t.TempDir()
	for _, tool := range []string{"sudo", "apt-get", "dnf", "yay"} {
		if err := os.WriteFile(filepath.Join(bin, tool), []byte("#!/bin/sh\nexit 99\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)
	p, err := pin.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ release, tool string }{{"ID=debian\nVERSION_ID=13", "apt-get"}, {"ID=fedora\nVERSION_ID=42", "dnf"}} {
		opts := install.Options{Pin: p, Yes: true}
		calls := 0
		wirePackages(&opts, []byte(tc.release), "amd64", func(cmd *exec.Cmd) error {
			calls++
			if strings.Join(cmd.Args, " ") != "sudo "+tc.tool+" remove -y -- gslapper" {
				t.Fatalf("removal command=%v", cmd.Args)
			}
			return nil
		})
		if err := opts.RemovePkg("gslapper"); err != nil || calls != 1 {
			t.Fatalf("removal: %v calls=%d", err, calls)
		}
	}
	for _, tc := range []struct{ release, arch string }{{"ID=fedora\nVERSION_ID=41", "amd64"}, {"ID=debian\nVERSION_ID=13", "arm64"}, {"ID=linuxmint\nVERSION_ID=22\nID_LIKE=ubuntu", "amd64"}} {
		opts := install.Options{Pin: p}
		wirePackages(&opts, []byte(tc.release), tc.arch, func(*exec.Cmd) error { t.Fatal("unavailable package ran a command"); return nil })
		if err := opts.InstallPkg("gslapper"); err == nil {
			t.Fatalf("accepted incompatible package: %v", tc)
		}
	}
}

func TestArchPackagesStayUnprivilegedAndNativeRemovalNeedsNoAsset(t *testing.T) {
	bin := t.TempDir()
	for _, tool := range []string{"sudo", "apt-get", "yay"} {
		if err := os.WriteFile(filepath.Join(bin, tool), []byte("#!/bin/sh\nexit 99\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)
	opts := install.Options{}
	calls := 0
	wirePackages(&opts, []byte("ID=arch"), "amd64", func(cmd *exec.Cmd) error {
		calls++
		if strings.Join(cmd.Args, " ") != "yay -S --needed -- gslapper" {
			t.Fatalf("AUR must build as the user: %v", cmd.Args)
		}
		return nil
	})
	if err := opts.InstallPkg("gslapper"); err != nil || calls != 1 {
		t.Fatalf("Arch: %v calls=%d", err, calls)
	}
	opts = install.Options{}
	calls = 0
	wirePackages(&opts, []byte("ID=debian\nVERSION_ID=12"), "amd64", func(cmd *exec.Cmd) error {
		calls++
		if strings.Join(cmd.Args, " ") != "sudo apt-get remove -- gslapper" {
			t.Fatalf("removal command: %v", cmd.Args)
		}
		return nil
	})
	if err := opts.RemovePkg("gslapper"); err != nil || calls != 1 {
		t.Fatalf("removal without asset: %v calls=%d", err, calls)
	}
}
