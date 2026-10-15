package seed

import (
	"os"
	"path/filepath"
	"testing"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestAnswersFromConfig(t *testing.T) {
	cases := map[string]struct {
		body    string
		lat     float64
		lon     float64
		place   string
		wantErr bool
	}{
		"full":         {`{"weather":{"latitude":52.52,"longitude":13.405,"location":"Berlin"}}`, 52.52, 13.405, "Berlin", false},
		"partial":      {`{"weather":{"latitude":52.52}}`, 0, 0, "", true},
		"no weather":   {`{"theme":{"preset":"compact"}}`, 0, 0, "", true},
		"malformed":    {`{`, 0, 0, "", true},
		"empty":        {``, 0, 0, "", true},
		"no location":  {`{"weather":{"latitude":52.52,"longitude":13.405}}`, 0, 0, "", true},
		"wrong shape":  {`{"weather":[]}`, 0, 0, "", true},
		"place only":   {`{"weather":{"location":"Berlin"}}`, 0, 0, "", true},
		"unknown keys": {`{"weather":{"latitude":52.52,"longitude":13.405,"location":"Berlin"},"other":1}`, 52.52, 13.405, "Berlin", false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			a, err := AnswersFromConfig(writeConfig(t, tc.body))
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, want error = %v", err, tc.wantErr)
			}
			if err == nil && (a.Latitude != tc.lat || a.Longitude != tc.lon || a.Location != tc.place) {
				t.Fatalf("got %+v", a)
			}
		})
	}
}

func TestAnswersFromConfigRefusesAMissingFile(t *testing.T) {
	if _, err := AnswersFromConfig(filepath.Join(t.TempDir(), "absent.json")); err == nil {
		t.Fatal("a missing config must refuse: the pipeline needs its coordinates")
	}
}

// The coordinates a config carries must still satisfy the rule ConfigJSON
// enforces on a fresh install, or an update could write a config the shell
// then refuses.
func TestAnswersFromConfigKeepsConfigJSONsRequirements(t *testing.T) {
	path := writeConfig(t, `{"weather":{"latitude":999,"longitude":13.405,"location":"Berlin"}}`)
	a, err := AnswersFromConfig(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if _, err := ConfigJSON(a); err == nil {
		t.Fatal("an out-of-range latitude must not survive into ConfigJSON")
	}
}
