// Package seed writes the sysc-shell configuration overlay the installer
// chose: theme, wallpaper directory, recommended plugins, and a weather
// widget on the default bar, and the session locker when sysc-lock is
// installed. The shell treats the file as a partial overlay
// over its built-in defaults, but a bar section is replaced wholesale, so the
// default right section is baked in alongside the weather item.
package seed

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Answers are the wizard choices that reach the shell configuration.
type Answers struct {
	Locker string
	// IdleLock locks the session after this much inactivity; zero leaves
	// idle locking off. It only applies alongside Locker.
	IdleLock     time.Duration
	Preset       string // standard, compact or expressive
	Mode         string // dark or light
	WallpaperDir string
	Plugins      []string
	Latitude     float64
	Longitude    float64
	Location     string
}

// defaultRight is the right bar section from sysc-shell config.Default(),
// which has no weather item because weather requires configured coordinates.
var defaultRight = []any{
	map[string]any{"id": "running-apps"},
	map[string]any{"id": "group", "items": []any{
		map[string]any{"id": "cpu", "display": "radial", "interval": "2s"},
		map[string]any{"id": "memory", "display": "radial", "interval": "2s"},
		map[string]any{"id": "temperature", "display": "radial", "interval": "2s"},
		map[string]any{"id": "gpu", "display": "radial", "interval": "2s"},
	}},
	map[string]any{"id": "clipboard"},
	map[string]any{"id": "notifications"},
}

// maxLocationBytes mirrors sysc-shell's weather place-label limit: a longer
// label is rejected by the shell and an invalid config stops the bar.
const maxLocationBytes = 80

// ConfigJSON builds the overlay. Coordinates and a place label are required,
// coordinates must be real, and the label must fit the shell's byte cap:
// anything the shell's own validation rejects would stop the bar from
// starting at all.
func ConfigJSON(a Answers) ([]byte, error) {
	if a.Latitude == 0 && a.Longitude == 0 {
		return nil, errors.New("weather coordinates are required")
	}
	if a.Location == "" {
		return nil, errors.New("weather location label is required")
	}
	if !(a.Latitude >= -90 && a.Latitude <= 90) {
		return nil, fmt.Errorf("weather latitude %g is outside -90 through 90", a.Latitude)
	}
	if !(a.Longitude >= -180 && a.Longitude <= 180) {
		return nil, fmt.Errorf("weather longitude %g is outside -180 through 180", a.Longitude)
	}
	if len(a.Location) > maxLocationBytes {
		return nil, fmt.Errorf("weather location %q is %d bytes, over the %d-byte limit",
			a.Location, len(a.Location), maxLocationBytes)
	}
	if a.Preset == "" {
		a.Preset = "standard"
	}
	if a.Mode == "" {
		a.Mode = "dark"
	}

	right := append(append([]any{}, defaultRight...), map[string]any{"id": "weather"})
	out := map[string]any{
		"theme":     map[string]any{"preset": a.Preset},
		"theme-gen": map[string]any{"source": "wallpaper", "mode": a.Mode},
		"weather": map[string]any{
			"latitude":  a.Latitude,
			"longitude": a.Longitude,
			"location":  a.Location,
		},
		"bar": map[string]any{"items": map[string]any{"right": right}},
	}
	if a.Locker != "" {
		out["session"] = map[string]any{"locker": a.Locker}
		if a.IdleLock > 0 {
			out["idle"] = map[string]any{"lock": a.IdleLock.String()}
		}
	}
	if a.WallpaperDir != "" {
		out["wallpaper"] = map[string]any{"image_directory": a.WallpaperDir}
	}
	if len(a.Plugins) > 0 {
		out["plugins"] = map[string]any{"enabled": a.Plugins}
	}
	return json.MarshalIndent(out, "", "  ")
}

// Write seeds the shell configuration at path. An existing file is never
// overwritten: the user's shell configuration outlives installer updates.
// The one addition it takes is the locker, through activateLocker.
func Write(path string, a Answers) error {
	if fi, err := os.Stat(path); err == nil {
		if fi.IsDir() {
			return fmt.Errorf("seed %s: path exists and is a directory", path)
		}
		return activateLocker(path, fi.Mode().Perm(), a)
	} else if !os.IsNotExist(err) {
		return err
	}
	data, err := ConfigJSON(a)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return writeFile(path, data, 0o600)
}

// activateLocker makes a newly installed locker the session locker in a
// configuration that names none, so installing sysc-lock over an earlier
// setup takes effect without a trip to Settings. The idle lock comes with it
// unless idle.lock is already set: without a locker the shell cannot offer
// idle locking, so no earlier idle choice is being replaced. A configuration
// that already names a locker, or that does not parse, is left untouched.
func activateLocker(path string, perm os.FileMode, a Answers) error {
	if a.Locker == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var cfg map[string]any
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if dec.Decode(&cfg) != nil || cfg == nil {
		return nil
	}
	session, _ := cfg["session"].(map[string]any)
	if locker, _ := session["locker"].(string); locker != "" {
		return nil
	}
	if session == nil {
		session = map[string]any{}
	}
	session["locker"] = a.Locker
	cfg["session"] = session
	if a.IdleLock > 0 {
		idle, _ := cfg["idle"].(map[string]any)
		if idle == nil {
			idle = map[string]any{}
		}
		if _, set := idle["lock"]; !set {
			idle["lock"] = a.IdleLock.String()
			cfg["idle"] = idle
		}
	}
	var out bytes.Buffer
	enc := json.NewEncoder(&out)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(cfg); err != nil {
		return err
	}
	return writeFile(path, out.Bytes(), perm)
}

// writeFile replaces path atomically with data at perm.
func writeFile(path string, data []byte, perm os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".config-*")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return fmt.Errorf("seed %s: %w", path, err)
	}
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), path)
}
