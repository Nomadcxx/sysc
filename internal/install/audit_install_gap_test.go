package install

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Nomadcxx/sysc/internal/fetch"
)

// Audit §5.3: a corrupt stamp must make Uninstall refuse, naming the problem.
func TestAuditUninstallCorruptStamp(t *testing.T) {
	home := uninstallHome(t)
	if err := os.WriteFile(filepath.Join(home, ".local", "state", "sysc", "installed.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Uninstall(Options{Home: home, Pin: loadPin(t)})
	if err == nil || !strings.Contains(err.Error(), "no SYSC installation found") {
		t.Fatalf("want refusal naming missing install, got %v", err)
	}
}

// Audit §5.3: a stamp listing an unknown component must not crash uninstall;
// known components are still removed.
func TestAuditUninstallIgnoresUnknownStampedComponent(t *testing.T) {
	home := uninstallHome(t)
	dir := filepath.Join(home, ".local", "state", "sysc")
	raw := []byte(`{"release":"v0.1.0","components":{"sysc-shell":"v0.1.0","sysc-ghost":"v9.9.9"},"gslapper_installed":false,"started":true}`)
	if err := os.Remove(filepath.Join(dir, "installed.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "installed.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := Uninstall(Options{Home: home, Pin: loadPin(t), Systemctl: func(...string) error { return nil }})
	if err != nil {
		t.Fatalf("uninstall with ghost component: %v", err)
	}
	for _, task := range res.Tasks {
		if task.Name == "sysc-ghost" {
			t.Fatalf("unknown component surfaced as a task: %+v", task)
		}
	}
	if exists(filepath.Join(home, ".local", "bin", "sysc-shell")) {
		t.Fatal("known component binary survived")
	}
}

// Audit §5.6 purge/keep split: purge removes config, default keeps it.
func TestAuditUninstallPurgeRemovesConfig(t *testing.T) {
	home := uninstallHome(t)
	if _, err := Uninstall(Options{Home: home, Pin: loadPin(t), Purge: true, Systemctl: func(...string) error { return nil }}); err != nil {
		t.Fatal(err)
	}
	if exists(filepath.Join(home, ".config", "sysc-shell", "config.json")) {
		t.Fatal("purge left the config behind")
	}
}

// Audit claim 9: a disabled component is skipped by name with zero fetches.
func TestAuditDisabledComponentZeroFetch(t *testing.T) {
	home := setupHome(t)
	var fetched []string
	_, err := Run(context.Background(), Options{
		Home: home, Pin: loadPin(t), Answers: weatherAnswers(), Yes: true,
		Download: func(_ context.Context, _ string, assets []fetch.Asset) error {
			for _, a := range assets {
				fetched = append(fetched, a.Name)
			}
			return nil
		},
		Swap:      func(string, string, []string) error { return nil },
		LookPath:  func(string) (string, error) { return "", os.ErrNotExist },
		Systemctl: func(...string) error { return nil },
		Now:       time.Unix(1000, 0),
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range fetched {
		if strings.Contains(name, "terminal") || strings.Contains(name, "lock") {
			t.Fatalf("disabled component fetched: %v", fetched)
		}
	}
}
