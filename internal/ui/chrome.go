package ui

import (
	"fmt"
	"strings"

	"github.com/Nomadcxx/sysc/internal/i18n"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

type Step int

const (
	StepWizard Step = iota
	StepInstalling
	StepDone
	StepFailed
)

const (
	MinWidth  = 80
	MinHeight = 24
)

// One palette for the canvas, controls, borders, banner, and text inputs.
var (
	Black      = lipgloss.Color("#000000")
	White      = lipgloss.Color("#ffffff")
	Muted      = lipgloss.Color("#a3a3a3")
	baseStyle  = lipgloss.NewStyle().Foreground(White).Background(Black)
	mutedStyle = baseStyle.Foreground(Muted)
	titleStyle = baseStyle.Bold(true)
	boxStyle   = baseStyle.Border(lipgloss.RoundedBorder()).BorderForeground(Muted).
			BorderBackground(Black).Padding(0, 1)
	adviceStyle = boxStyle.Foreground(Muted).Background(lipgloss.Color("#161616")).
			BorderBackground(lipgloss.Color("#161616"))
)

func ContentWidth(width int) int { return max(1, min(width-4, 100)-4) }

// Control keeps inputs and choices visibly separate from the page frame.
func Control(label, value string, width int, focused bool) string {
	style := boxStyle.Width(max(1, width-2))
	if focused {
		style = style.BorderForeground(White)
	}
	if label != "" {
		return mutedStyle.Render(label) + "\n" + style.Render(value)
	}
	return style.Render(value)
}

func Nav(loc i18n.Locale, page Page, step Step) string {
	if step == StepInstalling {
		return mutedStyle.Render(i18n.T(loc, "nav.wait"))
	}
	if step == StepDone || step == StepFailed {
		return mutedStyle.Render("Enter " + i18n.T(loc, "nav.close"))
	}
	var parts []string
	switch page {
	case PageTheme:
		parts = append(parts, "↑↓ "+i18n.T(loc, "nav.preset"), "←→ "+i18n.T(loc, "nav.mode"))
	case PagePlugins:
		parts = append(parts, "↑↓ "+i18n.T(loc, "nav.move"), "Space "+i18n.T(loc, "nav.toggle"))
	case PageWallpaper:
		parts = append(parts, i18n.T(loc, "nav.edit"), "Enter "+i18n.T(loc, "nav.next"))
	case PageWeather:
		parts = append(parts, i18n.T(loc, "nav.type"), "Enter "+i18n.T(loc, "nav.search"))
	case PageConflicts:
		parts = append(parts, "↑↓ "+i18n.T(loc, "nav.move"), "←→ "+i18n.T(loc, "nav.choose"))
	case PageConfirm:
		parts = append(parts, "Enter "+i18n.T(loc, "nav.install"))
	}
	parts = append(parts, "Esc "+i18n.T(loc, "nav.back"), "F9 "+i18n.T(loc, "nav.language"))
	if page == PageWeather || page == PageWallpaper {
		parts = append(parts, "Ctrl+C "+i18n.T(loc, "nav.quit"))
	} else {
		parts = append(parts, "q "+i18n.T(loc, "nav.quit"))
	}
	return mutedStyle.Render(strings.Join(parts, " • "))
}

// View is also used by callers that need only page-level advice.
func View(loc i18n.Locale, title, body string, page Page, step Step, width, height int, beams *BeamsTextEffect) string {
	return ViewWithHelp(loc, title, body, i18n.T(loc, "help."+pageKey(page)), page, step, width, height, beams, 0)
}

// Frame reserves room for advice and navigation before allocating body rows.
// ScrollLimit and rendering use the same measurement, including translations.
type frame struct {
	header, advice, actions, nav  string
	width, contentWidth, bodyRows int
}

func measure(loc i18n.Locale, page Page, step Step, width, height int, beams *BeamsTextEffect, help string) frame {
	f := frame{width: min(width-4, 100), contentWidth: ContentWidth(width)}
	f.header = titleStyle.Render("SYSC") + mutedStyle.Render("  /  "+i18n.T(loc, "ui.setup")+"  /  "+string(loc))
	if height >= 38 {
		f.header = baseStyle.Render("\n" + Banner() + "\n")
		if beams != nil {
			animated := beams.Render()
			if strings.TrimSpace(ansi.Strip(animated)) != "" {
				f.header = animated
			}
		}
	}
	f.advice = adviceStyle.Width(f.width - 2).Render("[?] " + ansi.Wrap(help, max(1, f.contentWidth-4), ""))
	primary := i18n.T(loc, "nav.next")
	if page == PageConfirm {
		primary = i18n.T(loc, "nav.install")
	}
	if page == PageWeather {
		primary = i18n.T(loc, "nav.search") + " / " + i18n.T(loc, "nav.next")
	}
	f.actions = lipgloss.JoinHorizontal(lipgloss.Top, Control("", "Esc  "+i18n.T(loc, "nav.back"), 22, false), "  ", Control("", "Enter  "+primary, min(36, f.width-24), true))
	if step == StepInstalling {
		f.actions = Control("", i18n.T(loc, "nav.wait"), f.width, false)
	}
	if step == StepDone || step == StepFailed {
		f.actions = Control("", "Enter  "+i18n.T(loc, "nav.close"), 30, true)
	}
	f.nav = ansi.Wrap(Nav(loc, page, step), f.width, " ")
	f.bodyRows = max(1, height-lipgloss.Height(f.header)-2-lipgloss.Height(f.advice)-lipgloss.Height(f.actions)-lipgloss.Height(f.nav)-1-2)
	return f
}

func ScrollLimit(loc i18n.Locale, body, help string, page Page, step Step, width, height int, beams *BeamsTextEffect) int {
	f := measure(loc, page, step, width, height, beams, help)
	return max(0, len(strings.Split(ansi.Wrap(body, max(1, f.contentWidth), ""), "\n"))-f.bodyRows)
}

// ViewWithHelp scrolls long content inside a frame; help/actions stay visible.
func ViewWithHelp(loc i18n.Locale, title, body, help string, page Page, step Step, width, height int, beams *BeamsTextEffect, offset int) string {
	if width < MinWidth || height < MinHeight {
		return baseStyle.Width(max(1, width)).Height(max(1, height)).Render(ansi.Wrap(i18n.T(loc, "ui.enlarge"), max(1, width), ""))
	}
	f := measure(loc, page, step, width, height, beams, help)
	lines := strings.Split(ansi.Wrap(body, f.contentWidth, ""), "\n")
	offset = min(max(0, offset), max(0, len(lines)-f.bodyRows))
	end := min(len(lines), offset+f.bodyRows)
	pane := boxStyle.Width(f.width - 2).Height(f.bodyRows).Render(strings.Join(lines[offset:end], "\n"))
	scroll := mutedStyle.Render("PgUp/PgDn " + i18n.T(loc, "nav.scroll"))
	if len(lines) > f.bodyRows {
		scroll += mutedStyle.Render(fmt.Sprintf(" • %d–%d / %d", offset+1, end, len(lines)))
		if end < len(lines) {
			scroll += " ↓"
		}
		if offset > 0 {
			scroll += " ↑"
		}
	}
	screen := lipgloss.JoinVertical(lipgloss.Center, f.header, titleStyle.Render(title), "", pane, f.advice, f.actions, f.nav, scroll)
	return baseStyle.Width(width).Height(height).Align(lipgloss.Center).Render(screen)
}

func pageKey(page Page) string {
	return []string{"theme", "wallpaper", "plugins", "weather", "conflicts", "confirm"}[page]
}
