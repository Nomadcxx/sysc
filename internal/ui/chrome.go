package ui

import (
	"strings"

	"github.com/Nomadcxx/sysc/internal/i18n"
	"github.com/charmbracelet/lipgloss"
)

// Step selects the bottom help bar for the current screen.
type Step int

const (
	StepWizard Step = iota
	StepInstalling
	StepDone
	StepFailed
)

// MinWidth and MinHeight are the smallest terminal the chrome renders in.
const (
	MinWidth  = 80
	MinHeight = 24
)

// greet-parity monochrome palette: base #1a1a1a, primary white, muted #666666.
var (
	baseStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff")).Background(lipgloss.Color("#1a1a1a"))
	mutedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#666666")).Background(lipgloss.Color("#1a1a1a"))
	titleStyle = baseStyle.Bold(true)
	boxStyle   = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#ffffff")).
			Foreground(lipgloss.Color("#ffffff")).
			Background(lipgloss.Color("#1a1a1a")).
			Padding(1, 2)
)

// Nav returns the always-visible bottom help bar for the current page and step.
// It advertises only keys that work on that page (#26, #27).
func Nav(loc i18n.Locale, page Page, step Step) string {
	switch step {
	case StepInstalling:
		return mutedStyle.Render(i18n.T(loc, "nav.wait"))
	case StepDone, StepFailed:
		return mutedStyle.Render("Enter " + i18n.T(loc, "nav.close"))
	}
	parts := make([]string, 0, 6)
	switch page {
	case PageTheme:
		parts = append(parts, "↑↓ "+i18n.T(loc, "nav.preset"))
	case PagePlugins:
		parts = append(parts, "↑↓ "+i18n.T(loc, "nav.toggle"))
	case PageWeather:
		parts = append(parts, i18n.T(loc, "nav.type"), "Enter "+i18n.T(loc, "nav.search"))
	case PageConfirm:
		parts = append(parts, "Enter "+i18n.T(loc, "nav.install"))
	default:
		parts = append(parts, "Enter "+i18n.T(loc, "nav.next"))
	}
	parts = append(parts,
		"Esc "+i18n.T(loc, "nav.back"),
		"F9 "+i18n.T(loc, "nav.language"),
	)
	if page == PageWeather {
		parts = append(parts, "Ctrl+C "+i18n.T(loc, "nav.quit"))
	} else {
		parts = append(parts, "q "+i18n.T(loc, "nav.quit"))
	}
	return mutedStyle.Render(strings.Join(parts, " • "))
}

// View renders the full-screen chrome: beams, title, body, bottom nav.
func View(loc i18n.Locale, title, body string, page Page, step Step, width, height int, beams *BeamsTextEffect) string {
	if width < MinWidth || height < MinHeight {
		return baseStyle.Render(i18n.T(loc, "ui.enlarge"))
	}
	parts := make([]string, 0, 4)
	if beams != nil {
		parts = append(parts, beams.Render())
	}
	parts = append(parts, titleStyle.Render(title))
	parts = append(parts, boxStyle.Width(width-8).Render(body))
	lines := strings.Split(lipgloss.JoinVertical(lipgloss.Center, parts...), "\n")
	for len(lines) < height-1 {
		lines = append(lines, "")
	}
	// ponytail: clip unusually tall bodies so the chrome always fits one screen.
	if len(lines) > height-1 {
		lines = lines[:height-1]
	}
	lines = append(lines, Nav(loc, page, step))
	return baseStyle.Render(strings.Join(lines, "\n"))
}
