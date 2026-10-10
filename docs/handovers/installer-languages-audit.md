# Handover: audit installer language PR (correctness + fluency)

Prepared 11 October 2026. Target: `feat/installer-languages` branch of
`Nomadcxx/sysc`, single commit `20d1747` on top of `main` at `29b0791`.

## Objective

Audit the PR that added Spanish, Portuguese, Japanese, Korean and Russian to
the Go installer (9 locales total). Two passes:

1. **Correctness review** of the code/test/doc changes.
2. **Fluency review** of the five new catalogs, ideally by fluent speakers or
   a strong multilingual model (e.g. `ollama run deepseek-v4-pro:cloud`).
   Model review is a screen, not a substitute for human fluent sign-off.

The companion implementation brief is [installer-languages.md](installer-languages.md);
its translation rules and acceptance conditions still apply.

## Checkout

```sh
cd /home/nomadx/sysc/.worktrees/aur-readmes/sysc
git fetch origin && git checkout feat/installer-languages
git diff --stat origin/main...HEAD
```

Diff touches only: `internal/i18n/i18n.go`, `internal/i18n/catalog/{es,pt,ja,ko,ru}.json`,
`internal/i18n/{i18n,audit_i18n_gap}_test.go`, `cmd/sysc/redesign_test.go`,
`cmd/sysc/main.go`, `README.md`. Anything else is out of scope for this PR.

## What the PR does

- Five Locale constants appended to `order` (cycle: en, zh-Hans, de, fr, es, pt, ja, ko, ru);
  `Match` gains base-language cases `es`, `pt`, `ja`, `ko`, `ru`; new exported `Locales()`.
- Five complete catalogs, each 100 keys in the exact English key order; `%s`
  arguments and literal tokens (`niri-session`, `sysc-shell`, `Debian`, `Fedora`,
  docs URL, `gSlapper`, `~/`, key names like PgUp/PgDn) preserved.
- Tests: `ja_JP` now expects JA; match table adds regional spellings and
  `C`/`POSIX`/invalid->EN; every-locale loops replace hard-coded four-locale
  lists; new `%s`-parity test and F9 full-cycle test; screen-fit and conflict
  scroll checks run across all nine locales.
- Docs: both `--lang` help strings list nine codes; README F9 sentence and
  flag table updated. docs-site needed no change (no per-locale claims).

## Correctness checklist

```sh
env SHELL=/bin/sh GOMAXPROCS=2 go test -count=1 -p 1 ./...
gofmt -l cmd internal
git diff --check origin/main...HEAD
```

- English fallback and `T` behavior unchanged; `order` append-only.
- `Match`: underscores, `.UTF-8`/`@` suffixes, region-only fallback all pass.
- F9 cycle visits each locale once, wraps to EN, keeps wizard state.
- No install/uninstall semantics changed; no new dependencies.
- Flag: `pt` is one neutral/Brazilian-leaning register, deliberately not two variants.

## Fluency review procedure

Per locale, feed the reviewer the English catalog plus the target catalog and
ask for issues only — not style rewrites. Report format per finding:
`locale | key | current | problem (misleading/unnatural/register/mixed) | suggested`.

Review focus per language:

| Locale | Watch for |
|---|---|
| `es` | `Espacio` as the Space key; `traspaso` reads naturally for handover; tu-form consistent |
| `pt` | voce-form consistency; `pular`/skip phrasing; `catalogo` vs `diretorio` for directory |
| `ja` | desu-masu consistency; handover phrasing; no over-long advice lines |
| `ko` | particle-suffix hacks around `%s` read acceptably; polite-level mix in short labels is fine |
| `ru` | aspect/case in state badges; «передача» reads naturally for handover |

Also verify no untranslated leftovers, no doubled spaces, valid UTF-8, and
that CJK/Russian strings still fit the 80x24 layout (tests cover this).

Hard rules: changes must be translation-text-only in the five JSON files;
keys, key order, `%s` counts and literal tokens above stay intact; re-run the
test suite after any catalog edit.

## Output

Report: correctness verdict; per-locale fluency verdict (accept / fix list);
which findings were applied; what still needs a human fluent reviewer.
Commit/push only with owner authorization.
