package stamp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStampAfterEnable(t *testing.T) {
	stateDir := t.TempDir()
	s := Stamp{
		Release:           "v0.1.0",
		Components:        map[string]string{"sysc-shell": "v0.1.0", "sysc-clipboard": "v0.1.1"},
		GSlapperInstalled: true,
		Started:           false,
	}

	if err := Write(stateDir, s, false); err != nil {
		t.Fatalf("write with units not enabled: %v", err)
	}
	if _, err := os.Stat(filepath.Join(stateDir, FileName)); !os.IsNotExist(err) {
		t.Fatalf("stamp written before units were enabled: %v", err)
	}

	if err := Write(stateDir, s, true); err != nil {
		t.Fatalf("write after enable: %v", err)
	}
	got, err := Read(stateDir)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got.Release != s.Release {
		t.Errorf("release = %q, want %q", got.Release, s.Release)
	}
	if got.Components["sysc-shell"] != "v0.1.0" || got.Components["sysc-clipboard"] != "v0.1.1" {
		t.Errorf("components = %v", got.Components)
	}
	if !got.GSlapperInstalled {
		t.Error("gslapper flag lost")
	}
	if got.Started {
		t.Error("started should be false for an SSH/TTY install")
	}
}

func TestOldStampStillDecodes(t *testing.T) {
	stateDir := t.TempDir()
	old := []byte(`{"release":"v0.1.0","components":{"sysc-shell":"v0.1.0"},"gslapper_installed":false,"started":true}`)
	if err := os.WriteFile(filepath.Join(stateDir, FileName), old, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Read(stateDir)
	if err != nil {
		t.Fatalf("read old stamp: %v", err)
	}
	if got.HandedOver != nil || got.Activation != nil {
		t.Fatalf("old stamp gained handover state: %+v", got)
	}
	if got.Release != "v0.1.0" || !got.Started {
		t.Fatalf("old stamp lost fields: %+v", got)
	}
}

func TestHandoverRoundTrip(t *testing.T) {
	stateDir := t.TempDir()
	s := Stamp{
		Release:    "v0.1.0",
		Components: map[string]string{"sysc-notify": "v0.1.0"},
		HandedOver: []Handover{{
			Name:           "mako",
			Unit:           "mako.service",
			UnitWasEnabled: true,
			Marker:         "// sysc-handover: ",
			Lines: []HandoverLine{{
				Path:      "/home/u/.config/niri/config.kdl",
				Commented: `    // sysc-handover: spawn-at-startup "mako"`,
				Original:  `    spawn-at-startup "mako"`,
			}},
		}},
		Activation: &Activation{
			Path:   "/home/u/.local/share/dbus-1/services/org.freedesktop.Notifications.service",
			Backup: "/home/u/.local/share/dbus-1/services/org.freedesktop.Notifications.service.sysc.bak",
		},
	}
	if err := Write(stateDir, s, true); err != nil {
		t.Fatal(err)
	}
	got, err := Read(stateDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.HandedOver) != 1 || got.HandedOver[0].Name != "mako" || !got.HandedOver[0].UnitWasEnabled {
		t.Fatalf("handovers = %+v", got.HandedOver)
	}
	if len(got.HandedOver[0].Lines) != 1 || got.HandedOver[0].Lines[0].Original != `    spawn-at-startup "mako"` {
		t.Fatalf("handover lines = %+v", got.HandedOver[0].Lines)
	}
	if got.Activation == nil || got.Activation.Backup != s.Activation.Backup {
		t.Fatalf("activation = %+v", got.Activation)
	}
}
