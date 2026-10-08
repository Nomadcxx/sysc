package niri

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Nomadcxx/sysc/internal/backup"
	"github.com/Nomadcxx/sysc/internal/stamp"
)

// HandoverMarker prefixes a spawn-at-startup line SYSC commented out during a
// conflict handover. The stamp records the full before/after lines, so the
// marker itself is for humans.
const HandoverMarker = "// sysc-handover: "

// File is one config file in the include tree: the root config first, then
// every file reachable through include lines, in walk order.
type File struct {
	Path string
	Text string
}

// IncludeTree returns the root config and every included file, resolving
// targets the way niri does. The SYSC sidecar is excluded: its contents are
// SYSC's, not the user's config. Unreadable includes are skipped like niri
// would skip an optional include that is absent; a missing root is an error.
func IncludeTree(opts Options) ([]File, error) {
	data, err := os.ReadFile(opts.ConfigPath)
	if err != nil {
		return nil, err
	}
	root := File{Path: filepath.Clean(opts.ConfigPath), Text: string(data)}

	sidecar := filepath.Clean(opts.SidecarPath)
	sidecarResolved := sidecar
	if r, err := filepath.EvalSymlinks(sidecar); err == nil {
		sidecarResolved = r
	}

	// ponytail: 32-file cap; raise it if a real config nests deeper.
	const maxIncludeFiles = 32
	files := []File{root}
	queue := []File{root}
	visited := map[string]bool{}
	if p, err := filepath.EvalSymlinks(opts.ConfigPath); err == nil {
		visited[p] = true
	}
	seen := 1
	for len(queue) > 0 && seen < maxIncludeFiles {
		f := queue[0]
		queue = queue[1:]
		for _, target := range includeTargets(f.Text) {
			inc := resolveInclude(filepath.Dir(f.Path), target)
			if inc == "" {
				continue
			}
			clean := filepath.Clean(inc)
			if clean == sidecar {
				continue
			}
			resolved := clean
			if r, err := filepath.EvalSymlinks(clean); err == nil {
				resolved = r
			}
			if resolved == sidecarResolved || visited[resolved] {
				continue
			}
			visited[resolved] = true
			incData, err := os.ReadFile(clean)
			if err != nil {
				continue
			}
			seen++
			child := File{Path: clean, Text: string(incData)}
			files = append(files, child)
			queue = append(queue, child)
		}
	}
	return files, nil
}

// Spawn is one live spawn-at-startup line whose program matches a name.
type Spawn struct {
	Name   string // the matched name
	Path   string // file containing the line
	LineNo int    // 0-based index within the file
	Line   string // exact line without its terminator
}

// Spawns returns every live spawn-at-startup line in the include tree whose
// program is one of names. The program is compared by file base name, so
// both `spawn-at-startup "mako"` and `spawn-at-startup "/usr/bin/mako"` match.
func Spawns(opts Options, names []string) ([]Spawn, error) {
	want := map[string]bool{}
	for _, n := range names {
		want[n] = true
	}
	files, err := IncludeTree(opts)
	if err != nil {
		return nil, err
	}
	var out []Spawn
	for _, f := range files {
		for i, line := range strings.Split(f.Text, "\n") {
			trimmed := strings.TrimRight(line, "\r")
			name, ok := spawnProgram(trimmed)
			if !ok || !want[name] {
				continue
			}
			out = append(out, Spawn{Name: name, Path: f.Path, LineNo: i, Line: trimmed})
		}
	}
	return out, nil
}

// spawnProgram returns the program of a live spawn-at-startup line.
func spawnProgram(line string) (string, bool) {
	rest, ok := strings.CutPrefix(strings.TrimLeft(line, " \t"), "spawn-at-startup")
	if !ok || (rest != "" && rest[0] != ' ' && rest[0] != '\t') {
		return "", false
	}
	rest = strings.TrimLeft(rest, " \t")
	if rest == "" || rest[0] != '"' {
		return "", false
	}
	end := strings.IndexByte(rest[1:], '"')
	if end < 0 {
		return "", false
	}
	prog := unescapeKDL(rest[1 : 1+end])
	return filepath.Base(prog), true
}

// PlannedLine is the reversible stamp record for a Spawn before CommentLine
// runs, so a handover can be recorded before any file changes.
func PlannedLine(sp Spawn) stamp.HandoverLine {
	indent := sp.Line[:len(sp.Line)-len(strings.TrimLeft(sp.Line, " \t"))]
	return stamp.HandoverLine{
		Path:      sp.Path,
		Original:  sp.Line,
		Commented: indent + HandoverMarker + strings.TrimLeft(sp.Line, " \t"),
	}
}

// CommentLine comments out one spawn line with HandoverMarker and returns the
// stamp record that RestoreLine can replay. The line is located by index and
// verified against line, so a config edited between detection and handover is
// a named error instead of a wrong edit. Line endings are preserved.
func CommentLine(path string, lineNo int, line string) (stamp.HandoverLine, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return stamp.HandoverLine{}, err
	}
	lines := strings.Split(string(data), "\n")
	if lineNo < 0 || lineNo >= len(lines) {
		return stamp.HandoverLine{}, fmt.Errorf("niri: %s line %d is out of range", path, lineNo+1)
	}
	live := strings.TrimRight(lines[lineNo], "\r")
	if live != line {
		return stamp.HandoverLine{}, fmt.Errorf("niri: %s line %d changed since detection", path, lineNo+1)
	}
	commented := PlannedLine(Spawn{Path: path, Line: live}).Commented
	if _, err := backup.FirstBak(path); err != nil {
		return stamp.HandoverLine{}, err
	}
	terminator := ""
	if strings.HasSuffix(lines[lineNo], "\r") {
		terminator = "\r"
	}
	lines[lineNo] = commented + terminator
	if err := writeAtomic(path, []byte(strings.Join(lines, "\n"))); err != nil {
		return stamp.HandoverLine{}, err
	}
	return stamp.HandoverLine{Path: path, Commented: commented, Original: live}, nil
}

// RestoreLine puts a commented spawn line back exactly as it was. The stamped
// commented text must still be present: a line the user edited after handover
// is left alone and reported. Other stamped lines are unaffected.
func RestoreLine(hl stamp.HandoverLine) error {
	data, err := os.ReadFile(hl.Path)
	if err != nil {
		return err
	}
	text := string(data)
	if !strings.Contains(text, hl.Commented) {
		return fmt.Errorf("niri: %s no longer contains %q", hl.Path, hl.Commented)
	}
	text = strings.Replace(text, hl.Commented, hl.Original, 1)
	return writeAtomic(hl.Path, []byte(text))
}
