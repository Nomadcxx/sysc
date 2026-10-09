package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Nomadcxx/sysc/internal/conflict"
	"github.com/Nomadcxx/sysc/internal/geo"
	"github.com/Nomadcxx/sysc/internal/i18n"
	"github.com/Nomadcxx/sysc/internal/install"
	"github.com/Nomadcxx/sysc/internal/ui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestPluginCheckboxesAreIndependent(t *testing.T) {
	m := newModel(i18n.EN, []string{"org.sysc.weather", "org.sysc.media"}, false)
	m.w.Page = ui.PagePlugins
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(model)
	if len(m.w.Plugins) != 2 {
		t.Fatal("moving focus changed plugin selection")
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = next.(model)
	if len(m.w.Plugins) != 1 || m.w.Plugins[0] != "org.sysc.weather" {
		t.Fatalf("space must toggle only focused media plugin: %v", m.w.Plugins)
	}
	if view := m.View(); !strings.Contains(view, "[x]") || !strings.Contains(view, "[ ]") {
		t.Fatalf("checkbox state missing: %s", view)
	}
}

func TestWallpaperInputEditsAndValidates(t *testing.T) {
	m := newModel(i18n.EN, nil, false)
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(model)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlA})
	m = next.(model)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlK})
	m = next.(model)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(model)
	if m.w.Page != ui.PageWallpaper || m.note == "" {
		t.Fatal("empty wallpaper path advanced without an inline error")
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("~/Pictures/custom")})
	m = next.(model)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(model)
	if m.w.Page != ui.PagePlugins || m.w.WallpaperDir != "~/Pictures/custom" {
		t.Fatalf("edited path was not saved: %+v", m.w)
	}
}

// All screens must retain advice/actions within one screen, including CJK text.
func TestInstallerScreensFit(t *testing.T) {
	for _, loc := range []i18n.Locale{i18n.EN, i18n.ZH, i18n.DE, i18n.FR} {
		for _, size := range [][2]int{{80, 24}, {100, 30}, {120, 40}} {
			for page := ui.PageTheme; page <= ui.PageConfirm; page++ {
				for step := ui.StepWizard; step <= ui.StepFailed; step++ {
					m := newModel(loc, []string{"org.sysc.weather", "org.sysc.media"}, false)
					m.width, m.height = size[0], size[1]
					m.w.Page, m.step = page, step
					m.w.Location, m.w.Latitude, m.w.Longitude = "Melbourne", -37.81, 144.96
					m.w.Findings = []conflict.Finding{{Name: "mako"}, {Name: "waybar"}, {Name: "quickshell"}}
					m.w.Suite = []string{"sysc-shell", "sysc-clipboard", "sysc-walls", "sysc-notify", "sysc-tray", "sysc-lock"}
					m.w.PackageAdviceKey = "confirm.system"
					m.w.PackageManager = "yay"
					m.installRes = install.Result{Tasks: []install.Task{{Name: "sysc-shell", Status: install.Done}, {Name: "gslapper", Status: install.Skipped, Reason: "no AUR helper"}}}
					m.focusPage()
					view := m.View()
					if h := lipgloss.Height(view); h != size[1] {
						t.Fatalf("%s %v/%v %v: height %d", loc, page, step, size, h)
					}
					for _, line := range strings.Split(view, "\n") {
						if w := lipgloss.Width(line); w != size[0] {
							t.Fatalf("%s %v/%v %v: width %d", loc, page, step, size, w)
						}
					}
					if !strings.Contains(view, "[?]") || !strings.Contains(view, "╰") {
						t.Fatalf("help/borders missing: %s", view)
					}
					if dir := os.Getenv("SYSC_UI_SNAPSHOTS"); dir != "" && loc == i18n.EN && step == ui.StepWizard {
						if err := os.MkdirAll(dir, 0o755); err != nil {
							t.Fatal(err)
						}
						path := filepath.Join(dir, fmt.Sprintf("screen-%d-%dx%d.ansi", page, size[0], size[1]))
						if err := os.WriteFile(path, []byte(view), 0o644); err != nil {
							t.Fatal(err)
						}
					}
				}
			}
		}
	}
}

func TestScrollReachesFinalWarningAndReturns(t *testing.T) {
	m := newModel(i18n.EN, nil, false)
	m.step = ui.StepDone
	m.installRes.Warnings = []string{strings.Repeat("warning\n", 40) + "FINAL WARNING"}
	for i := 0; i < 30; i++ {
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
		m = next.(model)
	}
	if !strings.Contains(m.View(), "FINAL WARNING") {
		t.Fatal("final warning is unreachable")
	}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	m = next.(model)
	if strings.Contains(m.View(), "FINAL WARNING") {
		t.Fatal("PageUp is stuck after repeated PageDown")
	}
}

func TestPackageHandoffReturnsOutputAndStatus(t *testing.T) {
	for _, status := range []int{0, 7} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			m := newModel(i18n.EN, nil, false)
			m.step = ui.StepInstalling
			out := &syncBuffer{}
			prog := newInstallProgram(m, nil, tea.WithInput(strings.NewReader("")), tea.WithOutput(out))
			done := make(chan error, 1)
			go func() { _, err := prog.Run(); done <- err }()
			reply := make(chan error, 1)
			prog.Send(packageRequestMsg{cmd: exec.Command("sh", "-c", fmt.Sprintf("printf 'helper output\\n'; exit %d", status)), reply: reply})
			select {
			case err := <-reply:
				if (err != nil) != (status != 0) {
					t.Fatalf("helper status %d: %v", status, err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("package handoff did not return")
			}
			prog.Send(installDoneMsg{res: install.Result{}})
			waitForOutput(t, out, i18n.T(i18n.EN, "done.blurb"))
			prog.Send(tea.KeyMsg{Type: tea.KeyEnter})
			select {
			case err := <-done:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("program did not resume")
			}
			if !strings.Contains(out.String(), "helper output") {
				t.Fatal("package stdout was not connected")
			}
		})
	}
}

func TestWeatherLookupPreservesDraftAndDoesNotDuplicateRequests(t *testing.T) {
	m := newModel(i18n.EN, nil, false)
	m.w.Page = ui.PageWeather
	m.focusPage()
	m.lookingUp = true
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("Paris")})
	m = next.(model)
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(model)
	if cmd != nil {
		t.Fatal("a pending lookup started another request")
	}
	next, _ = m.Update(guessMsg{place: geo.Place{City: "Melbourne", Latitude: -37.81, Longitude: 144.96}})
	m = next.(model)
	if m.input.Value() != "Paris" {
		t.Fatal("automatic lookup erased the city being typed")
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = next.(model)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(model)
	if m.input.Value() != "Paris" {
		t.Fatal("back/next erased the draft city")
	}
	next, _ = m.Update(guessMsg{place: geo.Place{City: "Paris", Latitude: 48.85, Longitude: 2.35}, query: "Paris"})
	m = next.(model)
	if m.input.Value() != "" || m.w.Location != "Paris" {
		t.Fatal("searched city was not accepted")
	}
}

func TestLateWeatherErrorDoesNotReplaceWallpaperError(t *testing.T) {
	m := newModel(i18n.EN, nil, false)
	m.w.Page = ui.PageWallpaper
	m.focusPage()
	m.note = "wallpaper error"
	next, _ := m.Update(guessMsg{err: errors.New("network error")})
	m = next.(model)
	if m.note != "wallpaper error" {
		t.Fatal("weather error appeared on the wallpaper page")
	}
}

func TestFocusedConflictStaysVisible(t *testing.T) {
	for _, loc := range []i18n.Locale{i18n.EN, i18n.ZH, i18n.DE, i18n.FR} {
		m := newModel(loc, nil, false)
		m.w.Page = ui.PageConflicts
		for i := 0; i < 12; i++ {
			m.w.Findings = append(m.w.Findings, conflict.Finding{Name: fmt.Sprintf("provider-%02d-", i) + strings.Repeat("x", 45), PIDs: []int{1}, UnitEnabled: true})
		}
		for i := 0; i < 11; i++ {
			next, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
			m = next.(model)
		}
		if !strings.Contains(m.View(), "> provider-11") {
			t.Fatalf("%s focused provider scrolled out of view", loc)
		}
	}
}
