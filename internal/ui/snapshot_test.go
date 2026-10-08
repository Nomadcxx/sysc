package ui

import (
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/Nomadcxx/sysc/internal/i18n"
)

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string { return ansiPattern.ReplaceAllString(s, "") }

// TestChromeFitsOneScreen locks the one-screen contract (#28 b, i) at the two
// reference sizes: the output is exactly as tall as the terminal, never wider,
// always keeps a rounded body box and the page footer, and the beams region
// above the title stays BannerHeight() rows (deterministic before the first
// animation tick).
func TestChromeFitsOneScreen(t *testing.T) {
	pages := []Page{PageTheme, PageWallpaper, PagePlugins, PageWeather, PageConfirm}
	steps := []Step{StepWizard, StepInstalling, StepDone, StepFailed}
	sizes := [][2]int{{80, 24}, {120, 40}}

	for _, size := range sizes {
		width, height := size[0], size[1]
		for _, page := range pages {
			for _, step := range steps {
				beams := NewBeamsTextEffect(width, BannerHeight(), Banner())
				out := View(i18n.EN, "T", "body", page, step, width, height, beams)
				lines := strings.Split(out, "\n")
				if len(lines) != height {
					t.Fatalf("%dx%d %v/%v: rendered %d lines, want exactly %d", width, height, page, step, len(lines), height)
				}
				for i, l := range lines {
					if w := lipgloss.Width(l); w > width {
						t.Fatalf("%dx%d %v/%v: line %d is %d cells wide", width, height, page, step, i, w)
					}
				}
				if !strings.Contains(out, "╭") || !strings.Contains(out, "╰") {
					t.Fatalf("%dx%d %v/%v: missing rounded body box", width, height, page, step)
				}
				if !strings.Contains(out, Nav(i18n.EN, page, step)) {
					t.Fatalf("%dx%d %v/%v: footer missing from view", width, height, page, step)
				}
				stripped := strings.Split(stripANSI(out), "\n")
				for i := 0; i < BannerHeight(); i++ {
					if strings.TrimSpace(stripped[i]) != "" {
						t.Fatalf("%dx%d %v/%v: beams row %d not blank: %q", width, height, page, step, i, stripped[i])
					}
				}
				if got := strings.TrimSpace(stripped[BannerHeight()]); got != "T" {
					t.Fatalf("%dx%d %v/%v: title expected after %d beams rows, got %q", width, height, page, step, BannerHeight(), got)
				}
			}
		}
	}
}

// TestChromeTallBodyKeepsBoxOnOneScreen covers the real install screen, whose
// task list can overflow the shortest supported terminal: the body must be
// truncated with a marker instead of clipping the box border off screen.
func TestChromeTallBodyKeepsBoxOnOneScreen(t *testing.T) {
	width, height := 80, 24
	beams := NewBeamsTextEffect(width, BannerHeight(), Banner())
	body := strings.Repeat("task\n", 40) + "task"
	out := View(i18n.EN, "T", body, PageConfirm, StepInstalling, width, height, beams)
	lines := strings.Split(out, "\n")
	if len(lines) != height {
		t.Fatalf("rendered %d lines, want exactly %d", len(lines), height)
	}
	if !strings.Contains(out, "╰") {
		t.Fatalf("bottom border clipped on a tall body:\n%s", stripANSI(out))
	}
	if !strings.Contains(out, "…") {
		t.Fatalf("tall body was not truncated")
	}
}
