package install

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Nomadcxx/sysc/internal/fetch"
	"github.com/Nomadcxx/sysc/internal/seed"
	"github.com/Nomadcxx/sysc/internal/stamp"
)

// issueReproRun is the audited setup: temp HOME, injected download, real
// SwapAll, stubbed systemctl.
func issueReproRun(t *testing.T, home string, answers seed.Answers, systemctl func(args ...string) error) error {
	t.Helper()
	_, err := Run(context.Background(), Options{
		Home: home, Pin: loadPin(t), Answers: answers, Yes: true,
		Download: func(_ context.Context, staging string, assets []fetch.Asset) error {
			if err := os.MkdirAll(staging, 0o755); err != nil {
				return err
			}
			for _, a := range assets {
				if err := os.WriteFile(filepath.Join(staging, a.Name), []byte("fake-elf"), 0o755); err != nil {
					return err
				}
			}
			return nil
		},
		LookPath:  func(string) (string, error) { return "", os.ErrNotExist },
		Systemctl: systemctl,
		Now:       time.Unix(1000, 0),
	})
	return err
}

func installLeftovers(home string) []string {
	var left []string
	for _, rel := range []string{
		".local/bin/sysc-shell",
		".local/bin/sysc-clipboard",
		".local/bin/sysc-walls-daemon",
		".config/systemd/user/sysc-shell.service",
		".config/systemd/user/sysc-clipboard.service",
		".config/systemd/user/sysc-walls.service",
		".config/sysc-shell/config.json",
		".config/niri/sysc.kdl",
		".local/state/sysc/installed.json",
	} {
		if exists(filepath.Join(home, rel)) {
			left = append(left, rel)
		}
	}
	return left
}

// Bad --lat must be rejected before StopAll, swap, enable, or seed. A failure
// here used to leave swapped binaries and enabled units with no stamp.
func TestBadLatitudeDoesNotTouchSystem(t *testing.T) {
	home := setupHome(t)
	rec := &recorder{}
	err := issueReproRun(t, home, seed.Answers{
		Latitude: 95, Longitude: 10, Location: "95.0000, 10.0000",
	}, rec.systemctl)
	if err == nil || !strings.Contains(err.Error(), "latitude") || !strings.Contains(err.Error(), "95") {
		t.Fatalf("Run err=%v, want weather latitude range error", err)
	}
	if len(rec.Systemd) != 0 {
		t.Fatalf("systemctl calls=%v, want none before a rejected latitude", rec.Systemd)
	}
	// Nothing was installed, so uninstall must not find a half-install, and
	// the files the old failure left behind must not be there.
	if _, uerr := Uninstall(Options{Home: home, Pin: loadPin(t), Systemctl: noopSystemctl}); uerr == nil {
		t.Fatal("uninstall found an install after a rejected latitude")
	} else if !strings.Contains(uerr.Error(), "no SYSC installation found") {
		t.Fatalf("Uninstall err=%v", uerr)
	}
	if left := installLeftovers(home); len(left) != 0 {
		t.Fatalf("rejected latitude left %v", left)
	}
}

// A missing niri config.kdl must be rejected before anything is swapped or
// enabled. niri.Apply used to be the first check, after the durable steps.
func TestMissingNiriConfigDoesNotTouchSystem(t *testing.T) {
	home := t.TempDir()
	rec := &recorder{}
	err := issueReproRun(t, home, weatherAnswers(), rec.systemctl)
	if err == nil || !strings.Contains(err.Error(), "niri config not found") {
		t.Fatalf("Run err=%v, want niri config not found", err)
	}
	if len(rec.Systemd) != 0 {
		t.Fatalf("systemctl calls=%v, want none before a missing niri config", rec.Systemd)
	}
	if _, uerr := Uninstall(Options{Home: home, Pin: loadPin(t), Systemctl: noopSystemctl}); uerr == nil {
		t.Fatal("uninstall found an install after a missing niri config")
	} else if !strings.Contains(uerr.Error(), "no SYSC installation found") {
		t.Fatalf("Uninstall err=%v", uerr)
	}
	if left := installLeftovers(home); len(left) != 0 {
		t.Fatalf("missing niri config left %v", left)
	}
}

// Once a swap has landed, a later failure (enable here) must leave a stamp so
// uninstall can see and remove the partial install.
func TestEnableFailureAfterSwapUninstallRemovesInstall(t *testing.T) {
	home := setupHome(t)
	rec := &recorder{}
	err := issueReproRun(t, home, weatherAnswers(), func(args ...string) error {
		rec.systemctl(args...)
		if len(args) > 0 && args[0] == "enable" {
			return os.ErrPermission
		}
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "enable") {
		t.Fatalf("Run err=%v, want enable failure", err)
	}
	if _, err := stamp.Read(filepath.Join(home, ".local", "state", "sysc")); err != nil {
		t.Fatalf("stamp missing after swap+enable failure: %v", err)
	}
	for _, name := range []string{"sysc-shell", "sysc-clipboard", "sysc-walls-daemon"} {
		if !exists(filepath.Join(home, ".local", "bin", name)) {
			t.Fatalf("binary %s was not swapped before the enable failure", name)
		}
	}
	if _, err := Uninstall(Options{Home: home, Pin: loadPin(t), Systemctl: noopSystemctl}); err != nil {
		t.Fatalf("Uninstall err=%v; partial install not visible", err)
	}
	if left := installLeftovers(home); len(left) != 0 {
		t.Fatalf("uninstall did not remove the install: %v", left)
	}
}

// The stamp has to exist before gSlapper. Ctrl+C during yay happens after the
// swap and before units; without a stamp, uninstall cannot see the binaries.
func TestStampExistsBeforeGSlapperAndUpdatesAfter(t *testing.T) {
	home := setupHome(t)
	var during stamp.Stamp
	called := false
	res, err := Run(context.Background(), Options{
		Home: home, Pin: loadPin(t), Answers: weatherAnswers(), Yes: true,
		Download: func(context.Context, string, []fetch.Asset) error { return nil },
		Swap:     func(string, string, []string) error { return nil },
		LookPath: func(string) (string, error) { return "", os.ErrNotExist },
		InstallPkg: func(string) error {
			called = true
			s, rerr := stamp.Read(filepath.Join(home, ".local", "state", "sysc"))
			if rerr != nil {
				t.Fatalf("no stamp before gSlapper: %v", rerr)
			}
			during = s
			return nil
		},
		Systemctl: noopSystemctl,
		Now:       time.Unix(1000, 0),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("gSlapper step did not run")
	}
	if during.GSlapperInstalled {
		t.Fatal("stamp marked gSlapper installed before the package step finished")
	}
	if len(during.Components) != 3 {
		t.Fatalf("stamp before gSlapper = %+v, want the swapped components", during)
	}
	if !res.Stamp.GSlapperInstalled {
		t.Fatal("stamp was not updated after gSlapper installed")
	}
}

// A swap that fails is rolled back inside SwapAll. That must not publish a
// stamp, or uninstall would delete the binaries the rollback just restored.
func TestSwapFailureDoesNotPublishStamp(t *testing.T) {
	home := setupHome(t)
	_, err := Run(context.Background(), Options{
		Home: home, Pin: loadPin(t), Answers: weatherAnswers(), Yes: true,
		Download:  func(context.Context, string, []fetch.Asset) error { return nil },
		Swap:      func(string, string, []string) error { return fmt.Errorf("disk full") },
		LookPath:  func(string) (string, error) { return "", os.ErrNotExist },
		Systemctl: noopSystemctl,
		Now:       time.Unix(1000, 0),
	})
	if err == nil || !strings.Contains(err.Error(), "disk full") {
		t.Fatalf("Run err=%v, want the swap error", err)
	}
	if _, rerr := stamp.Read(filepath.Join(home, ".local", "state", "sysc")); rerr == nil {
		t.Fatal("stamp written after a failed swap")
	}
	if left := installLeftovers(home); len(left) != 0 {
		t.Fatalf("failed swap left %v", left)
	}
}
