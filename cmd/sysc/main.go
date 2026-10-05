package main

import (
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Nomadcxx/sysc/internal/i18n"
	"github.com/Nomadcxx/sysc/internal/ui"
)

type tickMsg time.Time

type model struct {
	loc    i18n.Locale
	title  string
	body   string
	step   ui.Step
	beams  *ui.BeamsTextEffect
	width  int
	height int
}

func (m model) Init() tea.Cmd {
	return tick()
}

func tick() tea.Cmd {
	return tea.Tick(50*time.Millisecond, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		if m.beams != nil {
			m.beams.Resize(msg.Width, msg.Height)
		}
	case tickMsg:
		if m.beams != nil {
			m.beams.Update()
		}
		return m, tick()
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "f9":
			m.loc = i18n.Next(m.loc)
		}
	}
	return m, nil
}

func (m model) View() string {
	return ui.View(m.loc, m.title, m.body, m.step, m.width, m.height, m.beams)
}

func main() {
	loc := i18n.Match(os.Getenv("LANG"))
	m := model{
		loc:    loc,
		title:  i18n.T(loc, "theme.title"),
		body:   i18n.T(loc, "weather.blurb"),
		step:   ui.StepWizard,
		width:  ui.MinWidth,
		height: ui.MinHeight,
	}
	m.beams = ui.NewBeamsTextEffect(m.width, m.height, ui.Banner())
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
