package niri

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func auditWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func auditOpts(dir, config string) Options {
	return Options{
		ConfigPath:  filepath.Join(dir, config),
		SidecarPath: filepath.Join(dir, "sysc.kdl"),
		StateDir:    filepath.Join(dir, "state"),
		Binds:       DefaultBinds,
		Now:         time.Unix(1000, 0),
	}
}

// THE headline bug: run 2 sees run 1's own generated sidecar as "occupied"
// and rewrites it with zero binds. Reinstalling silently kills the launcher
// and settings hotkeys.
func TestAuditApplyTwiceKeepsBinds(t *testing.T) {
	dir := t.TempDir()
	opts := auditOpts(dir, "config.kdl")
	auditWrite(t, opts.ConfigPath, "input {}\n")

	r1, err := Apply(opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(r1.BindsAdded) != 2 {
		t.Fatalf("first apply binds = %v", r1.BindsAdded)
	}
	r2, err := Apply(opts)
	if err != nil {
		t.Fatal(err)
	}
	sidecar, _ := os.ReadFile(opts.SidecarPath)
	if len(r2.BindsSkipped) != 0 || !strings.Contains(string(sidecar), "Mod+Space") {
		t.Errorf("AUDIT: second apply stripped own binds (skipped=%v):\n%s", r2.BindsSkipped, sidecar)
	}
}

// A commented-out include line does not match includeRe, so Apply appends a
// second include even though one is already (commented) present... which is
// actually correct behavior for a commented line. But an include followed by
// a trailing comment on the same line IS live and gets duplicated.
func TestAuditIncludeWithTrailingCommentNotDuplicated(t *testing.T) {
	dir := t.TempDir()
	opts := auditOpts(dir, "config.kdl")
	auditWrite(t, opts.ConfigPath, "include \"sysc.kdl\" // added by me\n")

	res, err := Apply(opts)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(opts.ConfigPath)
	if res.IncludeAdded || strings.Count(string(data), `include "sysc.kdl"`) != 1 {
		t.Errorf("AUDIT: live include with trailing comment duplicated:\n%s", data)
	}
}

func TestAuditCRLFFirstBakAndSpawnOnlySyscShell(t *testing.T) {
	dir := t.TempDir()
	opts := auditOpts(dir, "config.kdl")
	auditWrite(t, opts.ConfigPath,
		"input {}\r\nspawn-at-startup \"sysc-shell\"\r\nspawn-at-startup \"gslapper\"\r\n")

	res, err := Apply(opts)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(opts.ConfigPath)
	text := string(data)
	if !strings.Contains(text, `// spawn-at-startup "sysc-shell"`) {
		t.Errorf("sysc-shell spawn not commented:\n%s", text)
	}
	if strings.Contains(text, `// spawn-at-startup "gslapper"`) {
		t.Errorf("AUDIT: foreign spawn touched:\n%s", text)
	}
	if _, err := os.Stat(opts.ConfigPath + ".sysc.bak"); err != nil {
		t.Errorf("first backup missing on changed config")
	}
	if res.IncludeAdded {
		if lfOnly := strings.Contains(text, "include \"sysc.kdl\"\n") && strings.Count(text, "\r\n") == 3; lfOnly {
			t.Errorf("AUDIT: include appended with bare LF into CRLF file")
		}
	}
}

func TestAuditForeignSidecarBackedUpBeforeOverwrite(t *testing.T) {
	dir := t.TempDir()
	opts := auditOpts(dir, "config.kdl")
	auditWrite(t, opts.ConfigPath, "input {}\n")
	auditWrite(t, opts.SidecarPath, "bind { my custom key; }\n")

	if _, err := Apply(opts); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(opts.SidecarPath + ".sysc.bak"); err != nil {
		t.Errorf("foreign sidecar overwritten with no first backup")
	}
}

// Claim 19: "uninstall restores the niri config". Apply comments the user's
// autostart line; Remove must undo it or the spawn stays dead forever.
func TestAuditRemoveRestoresOriginalBytes(t *testing.T) {
	dir := t.TempDir()
	opts := auditOpts(dir, "config.kdl")
	original := "input {}\nspawn-at-startup \"sysc-shell\"\n"
	auditWrite(t, opts.ConfigPath, original)

	if _, err := Apply(opts); err != nil {
		t.Fatal(err)
	}
	if err := Remove(opts); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(opts.ConfigPath)
	if string(data) != original {
		t.Errorf("AUDIT: uninstall did not restore niri config:\ngot:\n%s\nwant:\n%s", data, original)
	}
}

func TestAuditMissingConfigWritesNothing(t *testing.T) {
	dir := t.TempDir()
	opts := auditOpts(dir, "config.kdl")
	if _, err := Apply(opts); err == nil {
		t.Fatal("Apply succeeded without config.kdl")
	}
	if _, err := os.Stat(opts.SidecarPath); err == nil {
		t.Errorf("AUDIT: sidecar created despite missing config refusal")
	}
}
