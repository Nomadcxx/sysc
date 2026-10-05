package niri

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func gapOpts(t *testing.T, config string) Options {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config.kdl")
	if err := os.WriteFile(cfg, []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	return Options{ConfigPath: cfg, SidecarPath: filepath.Join(dir, "sysc.kdl"), StateDir: filepath.Join(dir, "state"), Binds: DefaultBinds, Now: time.Unix(1000, 0)}
}

func TestAuditGapNeverWritesShellKdl(t *testing.T) {
	o := gapOpts(t, "log {}\n")
	if err := os.WriteFile(filepath.Join(filepath.Dir(o.ConfigPath), "sysc-shell.kdl"), []byte("theme {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(o); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(filepath.Dir(o.ConfigPath), "sysc-shell.kdl"))
	if string(data) != "theme {}\n" {
		t.Errorf("AUDIT: Apply touched sysc-shell.kdl: %q", data)
	}
}

func TestAuditGapCommentedIncludeStillAdds(t *testing.T) {
	o := gapOpts(t, "// include \"sysc.kdl\"\nlog {}\n")
	r1, err := Apply(o)
	if err != nil {
		t.Fatal(err)
	}
	if !r1.IncludeAdded {
		t.Errorf("commented include: expected real include added")
	}
	data, _ := os.ReadFile(o.ConfigPath)
	n := 0
	for _, line := range splitLines(string(data)) {
		if len(line) > 0 && line[0] != '/' && containsInc(line) {
			n++
		}
	}
	if n != 1 {
		t.Logf("active include count after apply1: %d, content:\n%s", n, data)
	}
	if _, err := Apply(o); err != nil {
		t.Fatal(err)
	}
	data2, _ := os.ReadFile(o.ConfigPath)
	if string(data2) != string(data) {
		t.Errorf("AUDIT: apply2 not idempotent with commented include present")
	}
}

func splitLines(s string) []string {
	var out []string
	cur := ""
	for _, ch := range s {
		if ch == '\n' {
			out = append(out, cur)
			cur = ""
			continue
		}
		cur += string(ch)
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

func containsInc(s string) bool {
	return len(s) > 7 && s[:7] == "include"
}

func TestAuditGapKeyOnlyInCommentNotOccupied(t *testing.T) {
	o := gapOpts(t, "// Mod+Space is just prose\nlog {}\n")
	r, err := Apply(o)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.BindsAdded) != 2 {
		t.Errorf("AUDIT: commented key treated as occupied: added=%v skipped=%v", r.BindsAdded, r.BindsSkipped)
	} else {
		t.Logf("ok: comment-only key not occupied, added=%v", r.BindsAdded)
	}
}

func TestAuditGapFirstBakSacredAcrossTwoApplies(t *testing.T) {
	o := gapOpts(t, "log {}\n")
	foreign := "user sidecar bytes\n"
	bak := o.SidecarPath + ".sysc.bak"
	if err := os.WriteFile(o.SidecarPath, []byte(foreign), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(o); err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(o); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(bak)
	if err != nil || string(data) != foreign {
		t.Errorf("AUDIT: first *.sysc.bak not preserved: err=%v data=%q", err, data)
	}
}
