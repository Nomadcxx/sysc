package pin

import (
	"encoding/json"
	"strings"
	"testing"
)

func auditPin(t *testing.T, mut func(map[string]any)) []byte {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal([]byte(`{
"release":"v0.1.0",
"components":[{"id":"sysc-shell","tag":"v0.1.0","unit":"sysc-shell.service","binaries":[{"name":"sysc-shell","assets":{"amd64":{"url":"https://example.invalid/x","sha256":"`+strings.Repeat("1", 64)+`"}}}]},{"id":"sysc-terminal","disabled":true,"reason":"no assets"}],
"gslapper":{"family":"arch","package":"gslapper","version":"1.5.1"},
"recommended":["org.sysc.weather"]}`), &doc); err != nil {
		t.Fatal(err)
	}
	mut(doc)
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func auditFirstBinary(t *testing.T, doc map[string]any) map[string]any {
	t.Helper()
	comps := doc["components"].([]any)
	first := comps[0].(map[string]any)
	bins := first["binaries"].([]any)
	return bins[0].(map[string]any)
}

// Decode validates the six documented cases... and nothing else. These all
// pass Decode despite being invalid or hostile — the validation-gap finding.
func TestAuditDecodeAcceptsHostileValues(t *testing.T) {
	cases := []struct {
		name string
		mut  func(map[string]any)
	}{
		{"path traversal binary name", func(d map[string]any) { auditFirstBinary(t, d)["name"] = "../../.bashrc" }},
		{"http downgrade url", func(d map[string]any) {
			auditFirstBinary(t, d)["assets"].(map[string]any)["amd64"].(map[string]any)["url"] = "http://insecure.example/x"
		}},
		{"non-hex sha", func(d map[string]any) {
			auditFirstBinary(t, d)["assets"].(map[string]any)["amd64"].(map[string]any)["sha256"] = "not-hex-at-all"
		}},
		{"short sha", func(d map[string]any) {
			auditFirstBinary(t, d)["assets"].(map[string]any)["amd64"].(map[string]any)["sha256"] = "dead"
		}},
		{"unit does not exist", func(d map[string]any) {
			d["components"].([]any)[0].(map[string]any)["unit"] = "not-a-real-unit.service"
		}},
		{"duplicate component ids", func(d map[string]any) {
			comps := d["components"].([]any)
			comps = append(comps, comps[0])
			d["components"] = comps
		}},
		{"empty tag", func(d map[string]any) { d["components"].([]any)[0].(map[string]any)["tag"] = "" }},
		{"empty gslapper package", func(d map[string]any) { d["gslapper"].(map[string]any)["package"] = "" }},
	}
	for _, c := range cases {
		if _, err := Decode(auditPin(t, c.mut)); err == nil {
			t.Errorf("AUDIT: Decode accepted hostile pin: %s", c.name)
		}
	}
}

// The six documented rejections must hold (supports claim 15).
func TestAuditDecodeDocumentedRejectionsHold(t *testing.T) {
	cases := []struct {
		name string
		mut  func(map[string]any)
	}{
		{"empty release", func(d map[string]any) { d["release"] = "" }},
		{"empty recommended", func(d map[string]any) { d["recommended"] = []any{} }},
		{"disabled without reason", func(d map[string]any) {
			d["components"].([]any)[1].(map[string]any)["reason"] = ""
		}},
		{"enabled sysc-lock", func(d map[string]any) {
			d["components"].([]any)[1].(map[string]any)["disabled"] = false
			d["components"].([]any)[1].(map[string]any)["id"] = "sysc-lock"
		}},
		{"no binaries", func(d map[string]any) { delete(d["components"].([]any)[0].(map[string]any), "binaries") }},
		{"missing amd64 asset", func(d map[string]any) {
			auditFirstBinary(t, d)["assets"] = map[string]any{"arm64": map[string]any{"url": "u", "sha256": strings.Repeat("1", 64)}}
		}},
		{"empty url", func(d map[string]any) {
			auditFirstBinary(t, d)["assets"].(map[string]any)["amd64"].(map[string]any)["url"] = ""
		}},
		{"empty sha", func(d map[string]any) {
			auditFirstBinary(t, d)["assets"].(map[string]any)["amd64"].(map[string]any)["sha256"] = ""
		}},
	}
	for _, c := range cases {
		if _, err := Decode(auditPin(t, c.mut)); err == nil {
			t.Errorf("documented rejection missing: %s", c.name)
		}
	}
}

// Claim 15 core: placeholder zeros still Decode (must fail closed only at
// download time — recorded, per §7 accepted).
func TestAuditEmbeddedPlaceholderShaLoads(t *testing.T) {
	p, err := Load()
	if err != nil {
		t.Fatalf("embedded Load: %v", err)
	}
	for _, c := range p.Components {
		if c.Disabled {
			continue
		}
		for _, b := range c.Binaries {
			sha := b.Assets["amd64"].SHA256
			if sha != strings.Repeat("0", 64) {
				t.Fatalf("expected placeholder zeros per §7, got %q", sha)
			}
		}
	}
	t.Logf("AUDIT-INFO: embedded pin decodes with all-zero placeholder shas")
}
