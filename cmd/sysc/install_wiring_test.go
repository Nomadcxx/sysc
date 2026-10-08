package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Nomadcxx/sysc/internal/i18n"
	"github.com/Nomadcxx/sysc/internal/install"
	"github.com/Nomadcxx/sysc/internal/seed"
	"github.com/Nomadcxx/sysc/internal/ui"
)

type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

func TestInstallFlowWiring(t *testing.T) {
	cases := []struct {
		name    string
		fake    func(seed.Answers, func([]install.Task)) (install.Result, error)
		logPath func(t *testing.T, m *model)
		want    string
	}{
		{
			name: "done",
			fake: func(seed.Answers, func([]install.Task)) (install.Result, error) {
				return install.Result{}, nil
			},
			want: i18n.T(i18n.EN, "done.blurb"),
		},
		{
			name: "panic",
			fake: func(seed.Answers, func([]install.Task)) (install.Result, error) {
				panic("boom")
			},
			want: i18n.T(i18n.EN, "failed.blurb"),
		},
		{
			name: "log-error",
			fake: func(seed.Answers, func([]install.Task)) (install.Result, error) {
				return install.Result{}, nil
			},
			logPath: func(t *testing.T, m *model) {
				// A regular file where the state dir should be makes MkdirAll fail.
				blocker := filepath.Join(t.TempDir(), "blocker")
				if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
					t.Fatal(err)
				}
				m.logPath = filepath.Join(blocker, "installer.log")
			},
			want: i18n.T(i18n.EN, "failed.blurb"),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newModel(i18n.EN, nil, false)
			m.w.Page = ui.PageConfirm
			m.logPath = filepath.Join(t.TempDir(), "installer.log")
			if tc.logPath != nil {
				tc.logPath(t, &m)
			}
			out := &syncBuffer{}
			prog := newInstallProgram(m, tc.fake, tea.WithInput(nil), tea.WithOutput(out))
			done := make(chan error, 1)
			go func() {
				_, err := prog.Run()
				done <- err
			}()
			prog.Send(tea.WindowSizeMsg{Width: 120, Height: 40})
			prog.Send(tea.KeyMsg{Type: tea.KeyEnter})
			waitForOutput(t, out, tc.want)
			prog.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
			select {
			case err := <-done:
				if err != nil {
					t.Fatalf("program exited with %v", err)
				}
			case <-time.After(10 * time.Second):
				t.Fatal("program did not exit")
			}
		})
	}
}

func waitForOutput(t *testing.T, out *syncBuffer, want string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(out.String(), want) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %q in output:\n%s", want, out.String())
}

func TestNewModelLogPathUsesXDGState(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)
	m := newModel(i18n.EN, nil, false)
	want := filepath.Join(dir, "sysc", "installer.log")
	if m.logPath != want {
		t.Fatalf("logPath = %q, want %q", m.logPath, want)
	}
}
