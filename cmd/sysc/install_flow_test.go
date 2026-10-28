package main

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Nomadcxx/sysc/internal/i18n"
	"github.com/Nomadcxx/sysc/internal/install"
	"github.com/Nomadcxx/sysc/internal/seed"
	"github.com/Nomadcxx/sysc/internal/ui"
)

func TestConfirmStartsInstallInTUI(t *testing.T) {
	m := newModel(i18n.EN, nil, false)
	m.w.Page = ui.PageConfirm
	m.logPath = filepath.Join(t.TempDir(), "install.log")
	blocked := make(chan struct{})
	m.installer = func(seed.Answers, func([]install.Task)) (install.Result, error) {
		<-blocked
		return install.Result{}, nil
	}
	sends := make(chan tea.Msg, 16)
	m.send = func(msg tea.Msg) { sends <- msg }
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got, ok := next.(model)
	if !ok {
		t.Fatalf("Update returned %T, want model", next)
	}
	if got.step != ui.StepInstalling {
		t.Fatalf("step = %v, want installing", got.step)
	}
	if cmd != nil {
		t.Fatalf("confirm returned cmd, want nil (install runs in the background)")
	}
	close(blocked)
	// Wait for runInstall to finish before the test returns so t.TempDir
	// cleanup never races the goroutine writing its log file.
	deadline := time.After(5 * time.Second)
	for {
		select {
		case msg := <-sends:
			if _, ok := msg.(installDoneMsg); ok {
				return
			}
		case <-deadline:
			t.Fatal("install did not finish")
		}
	}
}

func TestTaskLines(t *testing.T) {
	tasks := []install.Task{
		{Name: "sysc-shell", Status: install.Done},
		{Name: "sysc-walls", Status: install.Skipped, Reason: "disabled"},
		{Name: "sysc-clipboard", Status: install.Failed, Reason: "boom"},
		{Name: "sysc-bar"},
	}
	out := taskLines(tasks, 0, i18n.EN)
	for _, want := range []string{"✓ sysc-shell", "- sysc-walls (disabled)", "✗ sysc-clipboard (boom)", "· sysc-bar"} {
		if !strings.Contains(out, want) {
			t.Errorf("taskLines missing %q in:\n%s", want, out)
		}
	}
}

func TestTaskLinesShowDownloadTicksAndActiveStep(t *testing.T) {
	tasks := []install.Task{
		{Name: "sysc-shell", Version: "v0.2.1", Status: install.Verified},
		{Name: "sysc-walls", Version: "v1.4.1", Status: install.Downloading},
		{Name: "sysc-tray"},
		{Name: "desktop-config", Status: install.Running},
	}
	out := taskLines(tasks, 0, i18n.EN)
	for _, want := range []string{"✓ sysc-shell v0.2.1 (download verified)", "⠋ sysc-walls v1.4.1 (downloading and verifying)", "· sysc-tray", "⠋ Desktop configuration"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing progress %q in:\n%s", want, out)
		}
	}
}

func TestProgressKeepsCurrentStepVisible(t *testing.T) {
	m := newModel(i18n.EN, nil, false)
	m.step, m.width, m.height = ui.StepInstalling, 80, 24
	var tasks []install.Task
	for i := 0; i < 15; i++ {
		tasks = append(tasks, install.Task{Name: "completed step", Status: install.Done})
	}
	tasks = append(tasks, install.Task{Name: "sysc-walls.service", Status: install.Running})
	next, _ := m.Update(progressMsg{tasks: tasks})
	got := next.(model)
	if got.scroll == 0 || !strings.Contains(got.View(), "sysc-walls.service") {
		t.Fatal("current service step is hidden below the viewport")
	}
	tasks[len(tasks)-1].Status = install.Failed
	next, _ = got.Update(installDoneMsg{res: install.Result{Tasks: tasks}, err: errors.New("service failed")})
	got = next.(model)
	if !strings.Contains(got.View(), "sysc-walls.service") {
		t.Fatal("failure screen hid the failed service")
	}
}

func TestDoneAndFailedStepsQuit(t *testing.T) {
	for _, step := range []ui.Step{ui.StepDone, ui.StepFailed} {
		m := newModel(i18n.EN, nil, false)
		m.step = step
		_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
		if cmd == nil {
			t.Fatalf("step %v: q did not quit", step)
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Fatalf("step %v: q returned %T, want quit", step, cmd())
		}
	}
}

func TestInstallingIgnoresKeys(t *testing.T) {
	m := newModel(i18n.EN, nil, false)
	m.step = ui.StepInstalling
	for _, key := range []tea.KeyMsg{
		{Type: tea.KeyRunes, Runes: []rune("q")},
		{Type: tea.KeyEscape},
	} {
		next, cmd := m.Update(key)
		got := next.(model)
		if cmd != nil || got.step != ui.StepInstalling {
			t.Fatalf("key %v: cmd=%v step=%v, want ignored while installing", key, cmd, got.step)
		}
	}
}

func TestCtrlCCancelsInstallThenFails(t *testing.T) {
	m := newModel(i18n.EN, nil, false)
	m.step = ui.StepInstalling
	cancelled := false
	m.cancel = func() { cancelled = true }

	// First Ctrl+C cancels the running install and keeps the program alive so
	// install.Run can unwind and restart units; it must not quit yet.
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	got := next.(model)
	if !cancelled {
		t.Fatal("first ctrl+c did not cancel the install context")
	}
	if !got.cancelling {
		t.Fatal("first ctrl+c did not record the cancel")
	}
	if cmd != nil {
		if _, ok := cmd().(tea.QuitMsg); ok {
			t.Fatal("first ctrl+c quit while the install was still unwinding")
		}
	}

	// Second Ctrl+C is the forced escape hatch.
	_, cmd = got.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("second ctrl+c did not quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("second ctrl+c returned %T, want quit", cmd())
	}
}

func TestInstallExitCode(t *testing.T) {
	failedTask := install.Task{Name: "sysc-shell", Status: install.Failed, Reason: "boom"}
	tests := []struct {
		name string
		m    model
		want int
	}{
		{"success", model{step: ui.StepDone}, 0},
		{"quit mid-install", model{step: ui.StepInstalling}, 130},
		{"cancelled", model{step: ui.StepFailed, installErr: context.Canceled}, 130},
		{"error", model{step: ui.StepFailed, installErr: errors.New("boom")}, 1},
		{"failed task", model{step: ui.StepDone, installRes: install.Result{Tasks: []install.Task{failedTask}}}, 1},
	}
	for _, tc := range tests {
		if got := installExit(tc.m); got != tc.want {
			t.Errorf("%s: installExit = %d, want %d", tc.name, got, tc.want)
		}
	}
}

func TestThemeArrowsChangeMode(t *testing.T) {
	m := newModel(i18n.EN, nil, false)
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	got := next.(model)
	if got.w.Mode != "light" {
		t.Fatalf("left: mode = %q, want light", got.w.Mode)
	}
	next, _ = got.Update(tea.KeyMsg{Type: tea.KeyRight})
	got = next.(model)
	if got.w.Mode != "dark" {
		t.Fatalf("right: mode = %q, want dark", got.w.Mode)
	}
	if got.w.Page != ui.PageTheme {
		t.Fatalf("arrows changed the page to %v", got.w.Page)
	}
}

func TestWeatherArrowsMoveCursor(t *testing.T) {
	m := newModel(i18n.EN, nil, false)
	m.w.Page = ui.PageWeather
	m.input.Focus()
	m.input.SetValue("abc")
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	got := next.(model)
	if got.input.Position() != 2 {
		t.Fatalf("left: cursor at %d, want 2", got.input.Position())
	}
	if got.w.Mode != "dark" {
		t.Fatalf("left changed the theme mode to %q", got.w.Mode)
	}
}
