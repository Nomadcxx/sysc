package units

import (
	"strings"
	"testing"
)

func TestAuditGapRecordedOrders(t *testing.T) {
	var got []string
	err := StartAll(func(args ...string) error { got = append(got, strings.Join(args, " ")); return nil })
	if err != nil {
		t.Fatal(err)
	}
	want := "clipboard walls shell"
	have := orderNames(got, "start")
	if have != want {
		t.Errorf("start order: got %q want %q", have, want)
	}
	got = nil
	if err := StopAll(func(args ...string) error { got = append(got, strings.Join(args, " ")); return nil }); err != nil {
		t.Fatal(err)
	}
	have = orderNames(got, "stop")
	want = "shell walls clipboard"
	if have != want {
		t.Errorf("stop order: got %q want %q", have, want)
	}
}

func orderNames(calls []string, verb string) string {
	var out []string
	for _, c := range calls {
		if strings.HasPrefix(c, verb) {
			f := strings.Fields(c)
			out = append(out, strings.TrimSuffix(strings.TrimPrefix(f[1], "sysc-"), ".service"))
		}
	}
	return strings.Join(out, " ")
}
