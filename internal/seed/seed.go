// Package seed writes the sysc-shell configuration overlay the installer
// chose: theme, wallpaper directory, recommended plugins, and a weather
// widget on the default bar. The shell treats the file as a partial overlay
// over its built-in defaults, but a bar section is replaced wholesale, so the
// default right section is baked in alongside the weather item.
package seed

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Answers are the wizard choices that reach the shell configuration.
type Answers struct {
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

// ConfigJSON builds the overlay. Coordinates and a place label are required:
// a weather item without them fails the shell's own validation.
func ConfigJSON(a Answers) ([]byte, error) {
	if a.Latitude == 0 && a.Longitude == 0 {
		return nil, errors.New("weather coordinates are required")
	}
	if a.Location == "" {
		return nil, errors.New("weather location label is required")
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
func Write(path string, a Answers) error {
	if _, err := os.Stat(path); err == nil {
		return nil
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
	tmp, err := os.CreateTemp(filepath.Dir(path), ".config-*")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return fmt.Errorf("seed %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), path)
}
