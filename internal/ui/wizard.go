package ui

import (
	"strings"

	"github.com/Nomadcxx/sysc/internal/conflict"
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
	PageConflicts
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

	// Findings are the competing providers detected before the wizard opens.
	// Choices holds the user's decision per finding name; the conflicts page
	// only appears when there is something to decide.
	Findings    []conflict.Finding
	Choices     map[string]conflict.Choice
	ConflictRow int

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

// Next advances one page. The weather page refuses to advance without a place,
// and the conflicts page is skipped when there is nothing to decide.
func (w Wizard) Next() Wizard {
	if w.Page == PageWeather && !w.weatherSet() {
		return w
	}
	for w.Page < PageConfirm {
		w.Page++
		if w.Page == PageConflicts && len(w.Findings) == 0 {
			continue
		}
		break
	}
	return w
}

// Back returns one page, never past the first, skipping an empty conflicts page.
func (w Wizard) Back() Wizard {
	for w.Page > PageTheme {
		w.Page--
		if w.Page == PageConflicts && len(w.Findings) == 0 {
			continue
		}
		break
	}
	return w
}

// MoveConflict moves the highlighted conflicts row, clamped to the list.
func (w Wizard) MoveConflict(delta int) Wizard {
	if len(w.Findings) == 0 {
		return w
	}
	w.ConflictRow += delta
	if w.ConflictRow < 0 {
		w.ConflictRow = 0
	}
	if w.ConflictRow >= len(w.Findings) {
		w.ConflictRow = len(w.Findings) - 1
	}
	return w
}

// CycleChoice cycles the highlighted row through Hand over, Keep both and Skip
// SYSC, backwards when forward is false.
func (w Wizard) CycleChoice(forward bool) Wizard {
	if len(w.Findings) == 0 {
		return w
	}
	if w.ConflictRow < 0 || w.ConflictRow >= len(w.Findings) {
		w.ConflictRow = 0
	}
	order := []conflict.Choice{conflict.HandOver, conflict.KeepBoth, conflict.SkipSYSC}
	name := w.Findings[w.ConflictRow].Name
	cur := w.choiceFor(name)
	idx := 0
	for i, c := range order {
		if c == cur {
			idx = i
		}
	}
	if forward {
		idx = (idx + 1) % len(order)
	} else {
		idx = (idx + len(order) - 1) % len(order)
	}
	if w.Choices == nil {
		w.Choices = map[string]conflict.Choice{}
	}
	w.Choices[name] = order[idx]
	return w
}

func (w Wizard) choiceFor(name string) conflict.Choice {
	if c, ok := w.Choices[name]; ok {
		return c
	}
	return conflict.KeepBoth
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
	case PageConflicts:
		return i18n.T(w.Locale, "conflicts.title")
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
	case PageConflicts:
		return w.conflictsBody()
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
		handovers := w.handovers()
		if len(handovers) > 0 {
			body += "\n\n" + strings.Join(handovers, "\n")
		}
		return body
	}
}

// conflictsBody lists each finding, its state and what the current choice does.
func (w Wizard) conflictsBody() string {
	lines := []string{i18n.T(w.Locale, "conflicts.blurb"), ""}
	for i, f := range w.Findings {
		cursor := " "
		if i == w.ConflictRow {
			cursor = "▸"
		}
		lines = append(lines,
			cursor+" "+f.Name+" → "+choiceLabel(w.Locale, w.choiceFor(f.Name)),
			"  "+strings.Join(stateLabels(w.Locale, f), ", "),
			"  "+i18n.T(w.Locale, consequenceKey(w.choiceFor(f.Name))),
		)
	}
	return strings.Join(lines, "\n")
}

// handovers lists the chosen handovers for the confirm page.
func (w Wizard) handovers() []string {
	lines := make([]string, 0, len(w.Findings))
	for _, f := range w.Findings {
		if w.choiceFor(f.Name) == conflict.HandOver {
			lines = append(lines, f.Name+" → "+choiceLabel(w.Locale, conflict.HandOver))
		}
	}
	return lines
}

// stateLabels describes why a provider was detected.
func stateLabels(loc i18n.Locale, f conflict.Finding) []string {
	var labels []string
	if len(f.PIDs) > 0 || f.BusOwner {
		labels = append(labels, i18n.T(loc, "conflicts.state.running"))
	}
	if f.UnitEnabled {
		labels = append(labels, i18n.T(loc, "conflicts.state.unit"))
	}
	if len(f.NiriLines) > 0 {
		labels = append(labels, i18n.T(loc, "conflicts.state.niri"))
	}
	if len(labels) == 0 {
		labels = append(labels, i18n.T(loc, "conflicts.state.found"))
	}
	return labels
}

// choiceLabel is the conflicts.choice.* label for a choice.
func choiceLabel(loc i18n.Locale, c conflict.Choice) string {
	switch c {
	case conflict.HandOver:
		return i18n.T(loc, "conflicts.choice.handover")
	case conflict.SkipSYSC:
		return i18n.T(loc, "conflicts.choice.skip")
	default:
		return i18n.T(loc, "conflicts.choice.keep")
	}
}

// consequenceKey is the conflicts.consequence.* key for a choice.
func consequenceKey(c conflict.Choice) string {
	switch c {
	case conflict.HandOver:
		return "conflicts.consequence.handover"
	case conflict.SkipSYSC:
		return "conflicts.consequence.skip"
	default:
		return "conflicts.consequence.keep"
	}
}

// pluginsLabel names the chosen plugins, or the localized none label.
func (w Wizard) pluginsLabel() string {
	if len(w.Plugins) == 0 {
		return i18n.T(w.Locale, "plugin.none")
	}
	return strings.Join(w.Plugins, ", ")
}
