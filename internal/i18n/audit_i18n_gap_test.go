package i18n

import "testing"

func TestAuditGapLangMatch(t *testing.T) {
	cases := map[string]Locale{
		"zh_CN.UTF-8": ZH, "zh-Hans-CN": ZH, "de_DE.UTF-8": DE,
		"fr_FR": FR, "en_US.UTF-8": EN, "ja_JP.UTF-8": JA,
		"es_ES@utf8": ES, "pt-BR": PT, "ko_KR.eucKR": KO, "ru-RU": RU, "": EN,
	}
	for in, want := range cases {
		if got := Match(in); got != want {
			t.Errorf("Match(%q)=%q want %q", in, got, want)
		}
	}
}
