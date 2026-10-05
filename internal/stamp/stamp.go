// Package stamp records what a SYSC install put on disk so uninstall can undo
// it. Callers write a stamp as soon as the first durable change lands and
// update it as later steps finish. Started stays false when the graphical
// session was not running to start the units.
package stamp

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// FileName is the stamp file inside the state directory.
const FileName = "installed.json"

// Stamp names the release and component versions an install placed, whether
// this run installed gSlapper, and whether the units were started (false for
// an SSH or TTY install that could only enable them).
type Stamp struct {
	Release           string            `json:"release"`
	Components        map[string]string `json:"components"`
	GSlapperInstalled bool              `json:"gslapper_installed"`
	Started           bool              `json:"started"`
}

// Read loads the stamp from stateDir.
func Read(stateDir string) (Stamp, error) {
	data, err := os.ReadFile(filepath.Join(stateDir, FileName))
	if err != nil {
		return Stamp{}, err
	}
	var s Stamp
	if err := json.Unmarshal(data, &s); err != nil {
		return Stamp{}, err
	}
	return s, nil
}

// Write stores the stamp when enabled is true. A false value writes nothing,
// so a caller that has not yet changed the system does not publish a record.
func Write(stateDir string, s Stamp, enabled bool) error {
	if !enabled {
		return nil
	}
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(stateDir, ".installed-*")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	if err := os.Rename(tmp.Name(), filepath.Join(stateDir, FileName)); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return nil
}
