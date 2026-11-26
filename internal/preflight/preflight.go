// Package preflight refuses installs that cannot work before anything is written.
package preflight

import (
	"errors"

	"github.com/Nomadcxx/sysc/internal/distro"
	"github.com/Nomadcxx/sysc/internal/i18n"
)

// Env is the machine state preflight reads. EUID is a parameter so tests can fake root.
type Env struct {
	EUID           int
	OSRelease      []byte
	WaylandDisplay string
	NiriSocket     string
	Locale         i18n.Locale
}

// Check returns a named refusal, or nil when the install may proceed.
// A missing WAYLAND_DISPLAY (SSH, TTY) is allowed: the run enables units only.
func Check(env Env) error {
	if env.EUID == 0 {
		return errors.New(i18n.T(env.Locale, "refuse.root"))
	}
	if err := CheckDistro(env.OSRelease); err != nil {
		return errors.New(i18n.T(env.Locale, "refuse.distro"))
	}
	if env.WaylandDisplay != "" && env.NiriSocket == "" {
		return errors.New(i18n.T(env.Locale, "refuse.niri"))
	}
	return nil
}

// CheckDistro gates on the Arch family alone. Install and both halves of
// update share it so a stamped install cannot move to another distribution
// and keep updating past the gate a fresh install enforces.
func CheckDistro(osRelease []byte) error {
	id, idLike := distro.ParseOSRelease(osRelease)
	return distro.Family(id, idLike)
}
