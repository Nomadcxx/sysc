package pin

import (
	"regexp"
	"strings"
	"testing"

	"github.com/Nomadcxx/sysc/internal/units"
)

var hex64 = regexp.MustCompile(`^[0-9a-f]{64}$`)

// The embedded pin is the supply chain. This guards the cut: every shipped
// component must name a real unit or declare itself binary-only, point at
// its own public release asset, and carry a plausible digest.
func TestEmbeddedPinIsPublishable(t *testing.T) {
	p, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if p.Release != "v0.1.1" {
		t.Errorf("release = %q, want v0.1.1", p.Release)
	}
	enabled := 0
	for _, c := range p.Components {
		if c.Disabled {
			if c.Reason == "" {
				t.Errorf("%s: disabled without a reason", c.ID)
			}
			continue
		}
		enabled++
		if _, ok := unitForName(c.Unit); !ok && !c.BinaryOnly {
			t.Errorf("%s: no matching unit template", c.ID)
		}
		if c.Tag == "" || len(c.Binaries) == 0 {
			t.Fatalf("%s: enabled without tag or binaries", c.ID)
		}
		for _, b := range c.Binaries {
			a, ok := b.Assets["amd64"]
			if !ok {
				t.Fatalf("%s/%s: no amd64 asset", c.ID, b.Name)
			}
			want := "https://github.com/Nomadcxx/" + c.ID + "/releases/download/" + c.Tag + "/" + b.Name
			if a.URL != want {
				t.Errorf("url = %q, want %q", a.URL, want)
			}
			if !hex64.MatchString(a.SHA256) || strings.Trim(a.SHA256, "0") == "" {
				t.Errorf("%s/%s: sha256 %q is not a real digest", c.ID, b.Name, a.SHA256)
			}
		}
	}
	if enabled != 7 {
		t.Errorf("enabled components = %d, want 7", enabled)
	}
}

func unitForName(name string) (units.Unit, bool) {
	for _, u := range units.All {
		if u.Name == name {
			return u, true
		}
	}
	return units.Unit{}, false
}
