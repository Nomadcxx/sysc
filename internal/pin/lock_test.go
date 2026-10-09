package pin

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestEnabledLockPin(t *testing.T) {
	p := Pin{Release: "v0.1.0", Recommended: []string{"org.sysc.weather"}, Components: []Component{{ID: "sysc-lock", Tag: "v0.1.0", Unit: "sysc-lock-session.service", Binaries: []Binary{{Name: "sysc-lock", Assets: map[string]Asset{"amd64": {URL: "https://example.invalid/sysc-lock", SHA256: strings.Repeat("1", 64)}}}}}}}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decode(data); err != nil {
		t.Fatal(err)
	}
}
