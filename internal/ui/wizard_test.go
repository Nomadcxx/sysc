package ui

import (
	"testing"

	"github.com/Nomadcxx/sysc/internal/i18n"
)

func TestWizardOrder(t *testing.T) {
	w := NewWizard(i18n.EN, []string{"org.sysc.weather"})
	if w.Page != PageTheme {
		t.Fatalf("start page = %v", w.Page)
	}
	w = w.Next()
	if w.Page != PageWallpaper {
		t.Fatalf("after theme = %v", w.Page)
	}
	w = w.Next()
	if w.Page != PagePlugins {
		t.Fatalf("after wallpaper = %v", w.Page)
	}
	w = w.Next()
	if w.Page != PageWeather {
		t.Fatalf("after plugins = %v", w.Page)
	}
	w.Location = "Berlin"
	w.Latitude = 52.52
	w.Longitude = 13.405
	w = w.Next()
	if w.Page != PageConfirm {
		t.Fatalf("after weather = %v", w.Page)
	}
	if w.Next().Page != PageConfirm {
		t.Fatal("confirm advanced past the end")
	}
}

func TestWeatherBlocksConfirm(t *testing.T) {
	w := NewWizard(i18n.EN, nil)
	w.Page = PageWeather
	if w.Next().Page != PageWeather {
		t.Fatal("empty location advanced past weather")
	}
	w.Location = "Berlin"
	if w.Next().Page != PageWeather {
		t.Fatal("location without coordinates advanced past weather")
	}
	w.Latitude = 52.52
	w.Longitude = 13.405
	if w.Next().Page != PageConfirm {
		t.Fatal("complete weather did not advance")
	}
}

func TestF9CyclesLocale(t *testing.T) {
	w := NewWizard(i18n.EN, nil)
	w = w.CycleLocale()
	if w.Locale != i18n.ZH {
		t.Fatalf("locale = %v", w.Locale)
	}
	if i18n.T(w.Locale, "theme.title") == "" {
		t.Fatal("copy did not resolve after the locale switch")
	}
	if w.Back().Page != PageTheme {
		t.Fatal("back moved past the first page")
	}
}
