// Package pin decodes the SYSC suite pin file: the tested combination of
// component releases that one installer release ships.
package pin

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Nomadcxx/sysc/internal/units"
)

var shaRE = regexp.MustCompile(`^[0-9a-fA-F]{64}$`)

// Asset is one downloadable file for a component binary.
type Asset struct {
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
}

// Binary is one executable a component installs, with per-architecture assets.
type Binary struct {
	Name   string           `json:"name"`
	Assets map[string]Asset `json:"assets"`
}

// Component is one SYSC-owned program in the pin. A disabled component is
// skipped by name and carries the reason it is not shipped yet.
type Component struct {
	ID       string   `json:"id"`
	Tag      string   `json:"tag"`
	Unit     string   `json:"unit"`
	Disabled bool     `json:"disabled,omitempty"`
	Reason   string   `json:"reason,omitempty"`
	Binaries []Binary `json:"binaries,omitempty"`
}

// GSlapper pins the AUR package and native release packages by build target
// and architecture. Family identifies the original AUR slot.
type GSlapper struct {
	Family  string                      `json:"family"`
	Package string                      `json:"package"`
	Version string                      `json:"version"`
	Assets  map[string]map[string]Asset `json:"assets,omitempty"`
}

// Pin is one tested suite release.
type Pin struct {
	Release     string      `json:"release"`
	Components  []Component `json:"components"`
	GSlapper    GSlapper    `json:"gslapper"`
	Recommended []string    `json:"recommended"`
}

// Decode parses and validates a pin file. It rejects a missing SHA, a missing
// amd64 asset, a malformed or non-https asset, a binary name that is not a
// plain file name, an unknown or duplicate unit, an empty tag, a disabled row without a reason, an empty recommended plugin
// list, and a gslapper row without a package name.
func Decode(data []byte) (Pin, error) {
	var p Pin
	if err := json.Unmarshal(data, &p); err != nil {
		return Pin{}, fmt.Errorf("pin: %w", err)
	}
	if p.Release == "" {
		return Pin{}, fmt.Errorf("pin: release is empty")
	}
	if len(p.Recommended) == 0 {
		return Pin{}, fmt.Errorf("pin: recommended plugin list is empty")
	}
	// A gslapper row that declares a family or version must name a package;
	// pins with no external package slot at all remain valid.
	if (p.GSlapper.Family != "" || p.GSlapper.Version != "" || len(p.GSlapper.Assets) > 0) && strings.TrimSpace(p.GSlapper.Package) == "" {
		return Pin{}, fmt.Errorf("pin: gslapper package name is empty")
	}
	for target, assets := range p.GSlapper.Assets {
		for arch, asset := range assets {
			if !strings.HasPrefix(asset.URL, "https://") || !shaRE.MatchString(asset.SHA256) {
				return Pin{}, fmt.Errorf("pin: gslapper %s/%s needs an https URL and a 64-hex SHA256", target, arch)
			}
		}
	}
	seen := map[string]bool{}
	for _, c := range p.Components {
		if seen[c.ID] {
			return Pin{}, fmt.Errorf("pin: duplicate component id %q", c.ID)
		}
		seen[c.ID] = true
		if c.Disabled {
			if c.Reason == "" {
				return Pin{}, fmt.Errorf("pin: component %q is disabled without a reason", c.ID)
			}
			continue
		}
		if len(c.Binaries) == 0 {
			return Pin{}, fmt.Errorf("pin: component %q has no binaries", c.ID)
		}
		for _, b := range c.Binaries {
			if b.Name == "" || b.Name == "." || b.Name == ".." || b.Name != filepath.Base(b.Name) {
				return Pin{}, fmt.Errorf("pin: component %q binary name %q is not a plain file name", c.ID, b.Name)
			}
			a, ok := b.Assets["amd64"]
			if !ok {
				return Pin{}, fmt.Errorf("pin: component %q binary %q has no amd64 asset", c.ID, b.Name)
			}
			if a.URL == "" || a.SHA256 == "" {
				return Pin{}, fmt.Errorf("pin: component %q binary %q amd64 asset needs url and sha256", c.ID, b.Name)
			}
			if !strings.HasPrefix(a.URL, "https://") {
				return Pin{}, fmt.Errorf("pin: component %q binary %q url %q is not https", c.ID, b.Name, a.URL)
			}
			if !shaRE.MatchString(a.SHA256) {
				return Pin{}, fmt.Errorf("pin: component %q binary %q sha256 %q is not 64 hex chars", c.ID, b.Name, a.SHA256)
			}
		}
		if c.Unit != "" && !knownUnit(c.Unit) {
			return Pin{}, fmt.Errorf("pin: component %q unit %q is not a known SYSC unit", c.ID, c.Unit)
		}
		if c.Tag == "" {
			return Pin{}, fmt.Errorf("pin: component %q tag is empty", c.ID)
		}
	}
	return p, nil
}

func knownUnit(name string) bool {
	for _, u := range units.All {
		if u.Name == name {
			return true
		}
	}
	return false
}

//go:embed pin.json
var embedded []byte

// Load decodes the pin embedded in this binary. The first pin cut replaces
// pin.json with the tested component releases.
func Load() (Pin, error) {
	return Decode(embedded)
}
