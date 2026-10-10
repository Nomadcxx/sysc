package pin

import (
	"os"
	"strings"
	"testing"
)

func TestDecodeRequiresAssetsAndSHA(t *testing.T) {
	data, err := os.ReadFile("testdata/ok.json")
	if err != nil {
		t.Fatal(err)
	}
	p, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode(ok.json): %v", err)
	}
	if p.Release != "v0.1.0" || len(p.Components) != 5 || len(p.Recommended) == 0 {
		t.Fatalf("decoded pin = %+v", p)
	}
	if p.GSlapper.Package != "gslapper" || p.GSlapper.Version != "1.5.1" {
		t.Fatalf("gslapper = %+v", p.GSlapper)
	}

	bad, err := os.ReadFile("testdata/missing-sha.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decode(bad); err == nil {
		t.Fatal("Decode(missing-sha.json) succeeded, want error")
	}

	cases := []struct {
		name string
		json string
		want string
	}{
		{
			"missing amd64 asset",
			`{"release":"v1","recommended":["a"],"components":[{"id":"x","binaries":[{"name":"x","assets":{"arm64":{"url":"u","sha256":"s"}}}]}]}`,
			"amd64",
		},
		{
			"enabled lock with invalid asset",
			`{"release":"v1","recommended":["a"],"components":[{"id":"sysc-lock","binaries":[{"name":"x","assets":{"amd64":{"url":"u","sha256":"s"}}}]}]}`,
			"https",
		},
		{
			"disabled without reason",
			`{"release":"v1","recommended":["a"],"components":[{"id":"x","disabled":true}]}`,
			"reason",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Decode([]byte(tc.json))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Decode() error = %v, want containing %q", err, tc.want)
			}
		})
	}
}

func TestDecodeAllowsNoRecommendedPlugins(t *testing.T) {
	if _, err := Decode([]byte(`{"release":"v1","recommended":[],"components":[]}`)); err != nil {
		t.Fatal(err)
	}
}

func TestBinaryOnlyComponentCannotNameService(t *testing.T) {
	data := `{"release":"v1","recommended":["a"],"components":[{"id":"sysc-terminal","tag":"v0.1.0","binary_only":true,"unit":"sysc-shell.service","binaries":[{"name":"sysc-terminal","assets":{"amd64":{"url":"https://example.invalid/sysc-terminal","sha256":"` + strings.Repeat("a", 64) + `"}}}]}]}`
	if _, err := Decode([]byte(data)); err == nil {
		t.Fatal("binary-only component accepted a service")
	}
}

func TestEmbeddedTerminalAndCatalogSelection(t *testing.T) {
	p, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Recommended) != 0 {
		t.Fatalf("pin enables plugins without installing them: %v", p.Recommended)
	}
	for _, c := range p.Components {
		if c.ID != "sysc-terminal" {
			continue
		}
		if c.Disabled || !c.BinaryOnly || c.Unit != "" || c.Tag != "v0.1.0" || len(c.Binaries) != 1 {
			t.Fatalf("terminal needs a pinned executable and no service: %+v", c)
		}
		return
	}
	t.Fatal("terminal is missing")
}

// The companion rows ship disabled until their repos tag a release. This
// fixture is the switch: when the pin rows flip to enabled, they must decode
// and name the units the installer ships.
func TestEnabledCompanionFixtureValidates(t *testing.T) {
	data, err := os.ReadFile("testdata/companions-enabled.json")
	if err != nil {
		t.Fatal(err)
	}
	p, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode(companions-enabled.json): %v", err)
	}
	want := map[string]string{
		"sysc-notify": "sysc-notify.service",
		"sysc-tray":   "sysc-tray.service",
	}
	for _, c := range p.Components {
		unit, ok := want[c.ID]
		if !ok {
			continue
		}
		if c.Disabled {
			t.Errorf("%s: fixture row is disabled", c.ID)
		}
		if c.Unit != unit {
			t.Errorf("%s: unit = %q, want %q", c.ID, c.Unit, unit)
		}
		if _, ok := unitForName(c.Unit); !ok {
			t.Errorf("%s: no matching unit template", c.ID)
		}
		delete(want, c.ID)
	}
	for id := range want {
		t.Errorf("%s: missing from fixture", id)
	}
}

func TestNativePackagePinsRequireHTTPSAndSHA(t *testing.T) {
	for _, asset := range []string{
		`{"url":"https://example.invalid/gslapper.deb","sha256":""}`,
		`{"url":"http://example.invalid/gslapper.deb","sha256":"` + strings.Repeat("a", 64) + `"}`,
		`{"url":"https://example.invalid/gslapper.deb","sha256":"not-a-hash"}`,
	} {
		data := `{"release":"v1","recommended":["a"],"gslapper":{"package":"gslapper","assets":{"debian13":{"amd64":` + asset + `}}}}`
		if _, err := Decode([]byte(data)); err == nil {
			t.Fatalf("accepted unverified native package: %s", asset)
		}
	}
}
