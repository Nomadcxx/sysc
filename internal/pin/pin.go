// Package pin decodes the SYSC suite pin file: the tested combination of
// component releases that one installer release ships.
package pin

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

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

// GSlapper is the external video-wallpaper package slot for a distro family.
type GSlapper struct {
	Family  string `json:"family"`
	Package string `json:"package"`
	Version string `json:"version"`
}

// Pin is one tested suite release.
type Pin struct {
	Release     string      `json:"release"`
	Components  []Component `json:"components"`
	GSlapper    GSlapper    `json:"gslapper"`
	Recommended []string    `json:"recommended"`
}

// Decode parses and validates a pin file. It rejects a missing SHA, a missing
// amd64 asset, an enabled sysc-lock, a disabled row without a reason, and an
// empty recommended plugin list.
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
	for _, c := range p.Components {
		if c.Disabled {
			if c.Reason == "" {
				return Pin{}, fmt.Errorf("pin: component %q is disabled without a reason", c.ID)
			}
			continue
		}
		if c.ID == "sysc-lock" {
			return Pin{}, fmt.Errorf("pin: sysc-lock must stay disabled until it ships")
		}
		if len(c.Binaries) == 0 {
			return Pin{}, fmt.Errorf("pin: component %q has no binaries", c.ID)
		}
		for _, b := range c.Binaries {
			a, ok := b.Assets["amd64"]
			if !ok {
				return Pin{}, fmt.Errorf("pin: component %q binary %q has no amd64 asset", c.ID, b.Name)
			}
			if a.URL == "" || a.SHA256 == "" {
				return Pin{}, fmt.Errorf("pin: component %q binary %q amd64 asset needs url and sha256", c.ID, b.Name)
			}
		}
	}
	return p, nil
}

//go:embed pin.json
var embedded []byte

// Load decodes the pin embedded in this binary. The first pin cut replaces
// pin.json with the tested component releases.
func Load() (Pin, error) {
	return Decode(embedded)
}
