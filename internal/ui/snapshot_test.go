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

// TestChromeFitsOneScreen checks the frame, advice, and navigation at compact
// and tall sizes. The large banner appears only when forms have enough room.
func TestChromeFitsOneScreen(t *testing.T) {
	pages := []Page{PageTheme, PageWallpaper, PagePlugins, PageWeather, PageConflicts, PageConfirm}
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
				stripped := stripANSI(out)
				if !strings.Contains(stripped, "[?]") {
					t.Fatal("contextual advice is missing")
				}
				if height == 24 && strings.Contains(stripped, "SEE YOU IN SPACE COWBOY") {
					t.Fatal("large banner obscures compact forms")
				}
				if height >= 38 && !strings.Contains(stripped, "SEE YOU IN SPACE COWBOY") {
					t.Fatal("tall screen lost its banner")
				}

			}
		}
	}
}

// Long content keeps its frame and advertises keyboard scrolling.
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
	if !strings.Contains(out, "↓") {
		t.Fatalf("tall body has no scroll indicator")
	}
}
