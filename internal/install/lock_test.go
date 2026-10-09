package install

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Nomadcxx/sysc/internal/fetch"
	"github.com/Nomadcxx/sysc/internal/pin"
)

func TestLockInstallsEnablesAndUninstalls(t *testing.T) {
	home := setupHome(t)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local/state"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local/share"))
	p := loadPin(t)
	p.Components = []pin.Component{{ID: "sysc-lock", Tag: "v0.1.0", Unit: "sysc-lock-session.service", Binaries: []pin.Binary{{Name: "sysc-lock", Assets: map[string]pin.Asset{"amd64": {URL: "https://example.invalid/sysc-lock", SHA256: strings.Repeat("1", 64)}}}}}}
	var calls []string
	opts := Options{Home: home, Pin: p, Answers: weatherAnswers(), InNiriSession: true,
		Systemctl: func(args ...string) error { calls = append(calls, strings.Join(args, " ")); return nil },
		LookPath:  func(string) (string, error) { return "/usr/bin/gslapper", nil },
		Download: func(_ context.Context, staging string, assets []fetch.Asset) error {
			if len(assets) != 1 || assets[0].Name != "sysc-lock" {
				t.Fatalf("assets=%v", assets)
			}
			if err := os.MkdirAll(staging, 0700); err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(staging, "sysc-lock"), []byte("candidate"), 0755)
		},
	}
	if _, err := Run(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"enable sysc-lock-session.service", "start sysc-lock-session.service"} {
		if !strings.Contains(strings.Join(calls, "\n"), want) {
			t.Errorf("missing %s: %v", want, calls)
		}
	}
	unitPath := filepath.Join(home, ".config/systemd/user/sysc-lock-session.service")
	unit, err := os.ReadFile(unitPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(unit), "ExecStart=%h/.local/bin/sysc-lock --session") {
		t.Fatal("wrong lock service")
	}
	data, err := os.ReadFile(filepath.Join(home, ".config/sysc-shell/config.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Session struct {
			Locker string `json:"locker"`
		} `json:"session"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Session.Locker != "sysc-lock" {
		t.Fatalf("locker=%q", cfg.Session.Locker)
	}
	if _, err := Uninstall(opts); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{unitPath, filepath.Join(home, ".local/bin/sysc-lock")} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("uninstall left %s: %v", path, err)
		}
	}
	if !strings.Contains(strings.Join(calls, "\n"), "disable sysc-lock-session.service") {
		t.Fatal("uninstall did not disable lock")
	}
}

func TestDisabledLockPreservesIndependentService(t *testing.T) {
	home := setupHome(t)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local/state"))
	p := loadPin(t)
	var calls []string
	opts := Options{Home: home, Pin: p, Answers: weatherAnswers(), Systemctl: func(args ...string) error { calls = append(calls, strings.Join(args, " ")); return nil }, LookPath: func(string) (string, error) { return "/usr/bin/gslapper", nil }, Download: func(context.Context, string, []fetch.Asset) error { return nil }, Swap: func(string, string, []string) error { return nil }}
	if _, err := Run(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
	if _, err := Uninstall(opts); err != nil {
		t.Fatal(err)
	}
	for _, call := range calls {
		if strings.Contains(call, "sysc-lock-session.service") && !strings.HasPrefix(call, "is-active ") {
			t.Errorf("changed independent lock service: %s", call)
		}
	}
}
