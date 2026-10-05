package distro

import (
	"os"
	"strings"
	"testing"
)

func TestFamilyGate(t *testing.T) {
	cases := []struct {
		id     string
		idLike []string
		want   string
	}{
		{"arch", nil, ""},
		{"cachyos", nil, ""},
		{"manjaro", nil, ""},
		{"endeavouros", nil, ""},
		{"garuda", []string{"arch"}, ""},
		{"fedora", nil, "fedora"},
		{"debian", nil, "debian"},
		{"ubuntu", nil, "ubuntu"},
		{"", nil, "unknown"},
	}
	for _, tc := range cases {
		err := Family(tc.id, tc.idLike)
		if tc.want == "" {
			if err != nil {
				t.Errorf("Family(%q, %v) = %v, want nil", tc.id, tc.idLike, err)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("Family(%q, %v) = %v, want containing %q", tc.id, tc.idLike, err, tc.want)
		}
	}
}

func TestParseOSRelease(t *testing.T) {
	cases := []struct {
		file   string
		id     string
		idLike []string
	}{
		{"testdata/os-release-arch", "arch", nil},
		{"testdata/os-release-derivative", "garuda", []string{"arch"}},
		{"testdata/os-release-fedora", "fedora", []string{"fedora"}},
	}
	for _, tc := range cases {
		data, err := os.ReadFile(tc.file)
		if err != nil {
			t.Fatal(err)
		}
		id, idLike := ParseOSRelease(data)
		if id != tc.id {
			t.Errorf("%s: id = %q, want %q", tc.file, id, tc.id)
		}
		if strings.Join(idLike, ",") != strings.Join(tc.idLike, ",") {
			t.Errorf("%s: idLike = %v, want %v", tc.file, idLike, tc.idLike)
		}
	}
}
