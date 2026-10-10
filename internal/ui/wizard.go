package ui

import (
	"fmt"
	"slices"
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

	Preset           string
	Mode             string
	WallpaperDir     string
	Plugins          []string
	AvailablePlugins []string
	PluginRow        int
	Suite            []string
	PackageAdviceKey string
	PackageManager   string
	ExistingConfig   bool

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
		Locale:           loc,
		Page:             PageTheme,
		Preset:           "standard",
		Mode:             "dark",
		WallpaperDir:     "~/Pictures/wallpapers",
		Plugins:          append([]string(nil), recommended...),
		AvailablePlugins: append([]string(nil), recommended...),
	}
}

// Next advances one page. The weather page refuses to advance without a place,
// and empty plugin and conflicts pages are skipped.
func (w Wizard) Next() Wizard {
	if w.Page == PageWeather && !w.weatherSet() {
		return w
	}
	for w.Page < PageConfirm {
		w.Page++
		if w.Page == PageConflicts && len(w.Findings) == 0 || w.Page == PagePlugins && len(w.AvailablePlugins) == 0 {
			continue
		}
		break
	}
	return w
}

// Back returns one page, never past the first, skipping empty optional pages.
func (w Wizard) Back() Wizard {
	for w.Page > PageTheme {
		w.Page--
		if w.Page == PageConflicts && len(w.Findings) == 0 || w.Page == PagePlugins && len(w.AvailablePlugins) == 0 {
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
	index, count := int(w.Page)+1, 6
	if len(w.AvailablePlugins) == 0 {
		count--
		if w.Page > PagePlugins {
			index--
		}
	}
	if len(w.Findings) == 0 {
		count--
		if w.Page > PageConflicts {
			index--
		}
	}
	return fmt.Sprintf("%02d / %02d  %s", index, count, i18n.T(w.Locale, pageKey(w.Page)+".title"))
}

// MovePlugin moves focus without changing selection.
func (w Wizard) MovePlugin(delta int) Wizard {
	w.PluginRow = min(max(0, w.PluginRow+delta), max(0, len(w.AvailablePlugins)-1))
	return w
}

func (w Wizard) TogglePlugin() Wizard {
	if len(w.AvailablePlugins) == 0 {
		return w
	}
	id := w.AvailablePlugins[w.PluginRow]
	// Copy before editing: Bubble Tea models are value snapshots.
	selected := append([]string(nil), w.Plugins...)
	if i := slices.Index(selected, id); i >= 0 {
		selected = slices.Delete(selected, i, i+1)
	} else {
		selected = append(selected, id)
	}
	w.Plugins = selected
	return w
}

func (w Wizard) Help() string {
	key := "help." + pageKey(w.Page)
	if w.Page == PageTheme {
		key = "help.preset." + w.Preset
	}
	if w.Page == PagePlugins && len(w.AvailablePlugins) > 0 {
		switch w.AvailablePlugins[w.PluginRow] {
		case "org.sysc.weather":
			key = "help.plugin.weather"
		case "org.sysc.media":
			key = "help.plugin.media"
		}
	}
	if w.Page == PageConflicts && len(w.Findings) > 0 {
		return i18n.T(w.Locale, consequenceKey(w.choiceFor(w.Findings[w.ConflictRow].Name)))
	}
	if w.Page == PageConfirm && w.ExistingConfig {
		return i18n.T(w.Locale, "help.existing")
	}
	return i18n.T(w.Locale, key)
}

func (w Wizard) Body() string { return w.BodyWidth(72) }

// BodyWidth renders real selection state inside bordered controls.
func (w Wizard) BodyWidth(width int) string {
	switch w.Page {
	case PageTheme:
		var rows []string
		for _, preset := range []string{"standard", "compact", "expressive"} {
			marker := "( )"
			if w.Preset == preset {
				marker = "(x)"
			}
			rows = append(rows, marker+"  "+i18n.T(w.Locale, "preset."+preset))
		}
		modes := "(x) " + i18n.T(w.Locale, "mode.dark") + "     ( ) " + i18n.T(w.Locale, "mode.light")
		if w.Mode == "light" {
			modes = "( ) " + i18n.T(w.Locale, "mode.dark") + "     (x) " + i18n.T(w.Locale, "mode.light")
		}
		return Control(i18n.T(w.Locale, "confirm.preset"), strings.Join(rows, "\n"), width, true) + "\n" + Control(i18n.T(w.Locale, "confirm.mode"), modes, width, true)
	case PageWallpaper:
		return i18n.T(w.Locale, "wallpaper.blurb") + "\n\n" + Control(i18n.T(w.Locale, "wallpaper.directory"), w.WallpaperDir, width, true)
	case PagePlugins:
		var rows []string
		for i, id := range w.AvailablePlugins {
			cursor, marker := " ", "[ ]"
			if i == w.PluginRow {
				cursor = ">"
			}
			if slices.Contains(w.Plugins, id) {
				marker = "[x]"
			}
			rows = append(rows, cursor+" "+marker+" "+w.pluginName(id))
		}
		if len(rows) == 0 {
			rows = []string{i18n.T(w.Locale, "plugin.none")}
		}
		return i18n.T(w.Locale, "plugins.blurb") + "\n\n" + Control("", strings.Join(rows, "\n"), width, true)
	case PageWeather:
		place := w.Location
		if place == "" {
			place = i18n.T(w.Locale, "weather.unset")
		} else {
			place += fmt.Sprintf("  %.4f, %.4f", w.Latitude, w.Longitude)
		}
		return Control(i18n.T(w.Locale, "weather.selected"), place, width, false)
	case PageConflicts:
		return w.conflictsBody(width)
	default:
		body := i18n.T(w.Locale, "confirm.preset") + ": " + i18n.T(w.Locale, "preset."+w.Preset) + "\n" +
			i18n.T(w.Locale, "confirm.mode") + ": " + i18n.T(w.Locale, "mode."+w.Mode) + "\n" +
			i18n.T(w.Locale, "confirm.wallpaper") + ": " + w.WallpaperDir + "\n" +
			i18n.T(w.Locale, "confirm.plugins") + ": " + w.pluginsLabel() + "\n" +
			i18n.T(w.Locale, "confirm.location") + ": " + w.Location + "\n\n" +
			i18n.T(w.Locale, "confirm.user") + "\n~/.local/bin • ~/.config/systemd/user\n"
		if len(w.Suite) > 0 {
			body += strings.Join(w.Suite, ", ") + "\n"
		}
		advice := i18n.T(w.Locale, w.PackageAdviceKey)
		if w.PackageAdviceKey == "confirm.system" {
			advice = fmt.Sprintf(advice, w.PackageManager)
		}
		body += "\n" + advice
		if w.PlainNiri {
			body += "\n\n" + i18n.T(w.Locale, "warn.niri_session")
		}
		if handovers := w.handovers(); len(handovers) > 0 {
			body += "\n\n" + strings.Join(handovers, "\n")
		}
		return body
	}
}

func (w Wizard) conflictsBody(width int) string {
	var rows []string
	for i, f := range w.Findings {
		cursor := " "
		if i == w.ConflictRow {
			cursor = ">"
		}
		choices := make([]string, 0, 3)
		for _, c := range []conflict.Choice{conflict.HandOver, conflict.KeepBoth, conflict.SkipSYSC} {
			marker := "( )"
			if w.choiceFor(f.Name) == c {
				marker = "(x)"
			}
			choices = append(choices, marker+" "+choiceLabel(w.Locale, c))
		}
		rows = append(rows, cursor+" "+f.Name+" • "+strings.Join(stateLabels(w.Locale, f), ", "), strings.Join(choices, "  "))
	}
	return Control("", strings.Join(rows, "\n"), width, true)
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
		if len(w.AvailablePlugins) == 0 {
			return i18n.T(w.Locale, "plugin.catalog")
		}
		return i18n.T(w.Locale, "plugin.none")
	}
	names := make([]string, 0, len(w.Plugins))
	for _, id := range w.Plugins {
		names = append(names, w.pluginName(id))
	}
	return strings.Join(names, ", ")
}

func (w Wizard) pluginName(id string) string {
	switch id {
	case "org.sysc.weather":
		return i18n.T(w.Locale, "plugin.weather")
	case "org.sysc.media":
		return i18n.T(w.Locale, "plugin.media")
	default:
		return id
	}
}
