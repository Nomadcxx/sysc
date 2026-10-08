package main

import (
	"path/filepath"
	"strings"
	"testing"

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
	m.send = func(tea.Msg) {}
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
}

func TestTaskLines(t *testing.T) {
	tasks := []install.Task{
		{Name: "sysc-shell", Status: install.Done},
		{Name: "sysc-walls", Status: install.Skipped, Reason: "disabled"},
		{Name: "sysc-clipboard", Status: install.Failed, Reason: "boom"},
		{Name: "sysc-bar"},
	}
	out := taskLines(tasks, 0)
	for _, want := range []string{"✓ sysc-shell", "- sysc-walls (disabled)", "✗ sysc-clipboard (boom)", "⠋ sysc-bar"} {
		if !strings.Contains(out, want) {
			t.Errorf("taskLines missing %q in:\n%s", want, out)
		}
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
