package ui

import (
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

var (
	baseStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff")).Background(lipgloss.Color("#0a0a0a"))
	mutedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#999999")).Background(lipgloss.Color("#0a0a0a"))
	titleStyle = baseStyle.Bold(true)
	boxStyle   = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#666666")).
			Foreground(lipgloss.Color("#ffffff")).
			Background(lipgloss.Color("#0a0a0a")).
			Padding(1, 2)
)

// Nav returns the always-visible bottom help bar for the current step.
func Nav(loc i18n.Locale, step Step) string {
	if step == StepInstalling {
		return mutedStyle.Render(i18n.T(loc, "nav.wait"))
	}
	line := "↑↓ " + i18n.T(loc, "nav.navigate") +
		" • Enter " + i18n.T(loc, "nav.next") +
		" • Esc " + i18n.T(loc, "nav.back") +
		" • F9 " + i18n.T(loc, "nav.language") +
		" • q " + i18n.T(loc, "nav.quit")
	return mutedStyle.Render(line)
}

// View renders the full-screen chrome: beams, title, body, bottom nav.
func View(loc i18n.Locale, title, body string, step Step, width, height int, beams *BeamsTextEffect) string {
	if width < MinWidth || height < MinHeight {
		return baseStyle.Render(i18n.T(loc, "ui.enlarge"))
	}
	parts := make([]string, 0, 4)
	if beams != nil {
		parts = append(parts, beams.Render())
	}
	parts = append(parts, titleStyle.Render(title))
	parts = append(parts, boxStyle.Width(width-8).Render(body))
	parts = append(parts, Nav(loc, step))
	return baseStyle.Render(lipgloss.JoinVertical(lipgloss.Center, parts...))
}
