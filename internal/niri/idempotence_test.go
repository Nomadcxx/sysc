package niri

import (
	"os"
	"testing"
)

// `sysc update` re-runs the niri step against a config the installer already
// wrote, so the second apply has to be a no-op down to the byte. If it ever is
// not, update is the change that made the user's niri config drift, so this is
// the assertion the verb depends on rather than a nicety.
func TestApplyTwiceIsByteIdentical(t *testing.T) {
	dir := t.TempDir()
	opts := auditOpts(dir, "config.kdl")
	auditWrite(t, opts.ConfigPath, "input {}\n")
	if _, err := Apply(opts); err != nil {
		t.Fatal(err)
	}
	first := map[string]string{}
	for _, p := range []string{opts.ConfigPath, opts.SidecarPath} {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		first[p] = string(b)
	}
	second, err := Apply(opts)
	if err != nil {
		t.Fatal(err)
	}
	for p, want := range first {
		got, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		if string(got) != want {
			t.Errorf("second apply changed %s:\nfirst:\n%s\nsecond:\n%s", p, want, got)
		}
	}
	// BindsAdded reports the binds the sidecar holds, which is the same two
	// every run. What must not repeat is the include line and the spawn
	// comment: those are the writes the first run made to the user's config.
	if second.IncludeAdded {
		t.Error("second apply added the include line again")
	}
	if second.SpawnCommented != 0 {
		t.Errorf("second apply commented %d spawn lines again", second.SpawnCommented)
	}
	if len(second.BindsSkipped) != 0 {
		t.Errorf("second apply thought the user occupied %v", second.BindsSkipped)
	}
}
