package preflight

import (
	"testing"

	"github.com/Nomadcxx/sysc/internal/i18n"
)

var archRelease = []byte("ID=arch\n")

func TestPreflightRefusesRoot(t *testing.T) {
	err := Check(Env{EUID: 0, OSRelease: archRelease, Locale: i18n.EN})
	if err == nil {
		t.Fatal("Check allowed root")
	}
}

func TestPreflightRefusesForeignDistro(t *testing.T) {
	err := Check(Env{EUID: 1000, OSRelease: []byte("ID=fedora\n"), Locale: i18n.EN})
	if err == nil {
		t.Fatal("Check allowed Fedora")
	}
}

func TestPreflightRefusesWaylandWithoutNiri(t *testing.T) {
	err := Check(Env{EUID: 1000, OSRelease: archRelease, WaylandDisplay: "wayland-1", Locale: i18n.EN})
	if err == nil {
		t.Fatal("Check allowed a Wayland session without Niri")
	}
}

func TestPreflightAllowsSSHInstall(t *testing.T) {
	if err := Check(Env{EUID: 1000, OSRelease: archRelease, Locale: i18n.EN}); err != nil {
		t.Fatalf("Check refused an SSH install: %v", err)
	}
}

func TestPreflightAllowsNiriSession(t *testing.T) {
	err := Check(Env{EUID: 1000, OSRelease: archRelease, WaylandDisplay: "wayland-1", NiriSocket: "/run/user/1000/niri.sock", Locale: i18n.EN})
	if err != nil {
		t.Fatalf("Check refused a Niri session: %v", err)
	}
}
