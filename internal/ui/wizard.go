package ui

import (
	"strings"

	"github.com/Nomadcxx/sysc/internal/i18n"
	"github.com/Nomadcxx/sysc/internal/seed"
)

// Page is one wizard screen, in order.
type Page int

const (
	PageTheme Page = iota
	PageWallpaper
	PagePlugins
	PageWeather
	PageConfirm
)

// Wizard collects the install answers before anything runs.
type Wizard struct {
	Locale i18n.Locale
	Page   Page

	Preset       string
	Mode         string
	WallpaperDir string
	Plugins      []string

	Latitude  float64
	Longitude float64
	Location  string

	// PlainNiri is a live niri whose graphical-session.target is inactive.
	// The confirm page warns and the install leaves the shell spawn line.
	PlainNiri bool
}

// NewWizard starts on the theme page with the guided defaults.
func NewWizard(loc i18n.Locale, recommended []string) Wizard {
	return Wizard{
		Locale:       loc,
		Page:         PageTheme,
		Preset:       "standard",
		Mode:         "dark",
		WallpaperDir: "~/Pictures/wallpapers",
		Plugins:      append([]string(nil), recommended...),
	}
}

// Next advances one page. The weather page refuses to advance without a place.
func (w Wizard) Next() Wizard {
	if w.Page == PageWeather && !w.weatherSet() {
		return w
	}
	if w.Page < PageConfirm {
		w.Page++
	}
	return w
}

// Back returns one page, never past the first.
func (w Wizard) Back() Wizard {
	if w.Page > PageTheme {
		w.Page--
	}
	return w
}

// CycleLocale moves to the next UI language.
func (w Wizard) CycleLocale() Wizard {
	w.Locale = i18n.Next(w.Locale)
	return w
}

// CycleMode toggles the theme mode between dark and light.
func (w Wizard) CycleMode() Wizard {
	if w.Mode == "dark" {
		w.Mode = "light"
	} else {
		w.Mode = "dark"
	}
	return w
}

// Answers converts the collected state for the installer.
func (w Wizard) Answers() seed.Answers {
	return seed.Answers{
		Preset:       w.Preset,
		Mode:         w.Mode,
		WallpaperDir: w.WallpaperDir,
		Plugins:      w.Plugins,
		Latitude:     w.Latitude,
		Longitude:    w.Longitude,
		Location:     w.Location,
	}
}

func (w Wizard) weatherSet() bool {
	return w.Location != "" && (w.Latitude != 0 || w.Longitude != 0)
}

// Title is the current page heading.
func (w Wizard) Title() string {
	switch w.Page {
	case PageTheme:
		return i18n.T(w.Locale, "theme.title")
	case PageWallpaper:
		return i18n.T(w.Locale, "wallpaper.title")
	case PagePlugins:
		return i18n.T(w.Locale, "plugins.title")
	case PageWeather:
		return i18n.T(w.Locale, "weather.title")
	default:
		return i18n.T(w.Locale, "confirm.title")
	}
}

// Body is the current page copy plus the collected value.
func (w Wizard) Body() string {
	switch w.Page {
	case PageTheme:
		return i18n.T(w.Locale, "theme.blurb") + "\n\n" + w.Preset + " • " + w.Mode
	case PageWallpaper:
		return i18n.T(w.Locale, "wallpaper.blurb") + "\n\n" + w.WallpaperDir
	case PagePlugins:
		return i18n.T(w.Locale, "plugins.blurb") + "\n\n" + w.pluginsLabel()
	case PageWeather:
		place := w.Location
		if place == "" {
			place = i18n.T(w.Locale, "weather.place")
		}
		return i18n.T(w.Locale, "weather.blurb") + "\n\n" + place
	default:
		body := i18n.T(w.Locale, "confirm.blurb") + "\n" +
			i18n.T(w.Locale, "confirm.preset") + ": " + w.Preset + "\n" +
			i18n.T(w.Locale, "confirm.mode") + ": " + w.Mode + "\n" +
			i18n.T(w.Locale, "confirm.wallpaper") + ": " + w.WallpaperDir + "\n" +
			i18n.T(w.Locale, "confirm.plugins") + ": " + w.pluginsLabel() + "\n" +
			i18n.T(w.Locale, "confirm.location") + ": " + w.Location + "\n" +
			i18n.T(w.Locale, "confirm.files") + ": ~/.local/bin, ~/.config/systemd/user"
		if w.PlainNiri {
			body += "\n\n" + i18n.T(w.Locale, "warn.niri_session")
		}
		return body
	}
}

// pluginsLabel names the chosen plugins, or the localized none label.
func (w Wizard) pluginsLabel() string {
	if len(w.Plugins) == 0 {
		return i18n.T(w.Locale, "plugin.none")
	}
	return strings.Join(w.Plugins, ", ")
}
