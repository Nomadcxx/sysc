package install

import (
	"errors"
	"testing"

	"github.com/Nomadcxx/sysc/internal/i18n"
)

func TestCJKFontTask(t *testing.T) {
	ja := i18n.JA
	var got string
	base := Options{
		Loc:              &ja,
		CJKFont:          "noto-fonts-cjk",
		InstallSystemPkg: func(pkg string) error { got = pkg; return nil },
		HasCJKFont:       func(i18n.Locale) bool { return false },
	}
	en := i18n.EN
	if cjkFontTask(Options{Loc: &en, CJKFont: "x", InstallSystemPkg: base.InstallSystemPkg}) != nil {
		t.Fatal("non-CJK locale should add no task")
	}
	if task := cjkFontTask(base); task == nil || task.Status != Done || got != "noto-fonts-cjk" {
		t.Fatalf("missing font should install: got %+v, installed %q", task, got)
	}
	opts := base
	opts.HasCJKFont = func(i18n.Locale) bool { return true }
	if task := cjkFontTask(opts); task == nil || task.Status != Skipped {
		t.Fatalf("present font should skip: got %+v", task)
	}
	if task := cjkFontTask(Options{Loc: &ja}); task == nil || task.Reason == "" {
		t.Fatalf("unsupported host should skip with a reason: got %+v", task)
	}
	opts = base
	opts.InstallSystemPkg = func(string) error { return errors.New("boom") }
	if task := cjkFontTask(opts); task == nil || task.Status != Skipped || task.Reason != "boom" {
		t.Fatalf("install failure should skip with error reason: got %+v", task)
	}
}
