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
			"enabled lock",
			`{"release":"v1","recommended":["a"],"components":[{"id":"sysc-lock","binaries":[{"name":"x","assets":{"amd64":{"url":"u","sha256":"s"}}}]}]}`,
			"sysc-lock",
		},
		{
			"disabled without reason",
			`{"release":"v1","recommended":["a"],"components":[{"id":"x","disabled":true}]}`,
			"reason",
		},
		{
			"empty recommended",
			`{"release":"v1","recommended":[],"components":[]}`,
			"recommended",
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
