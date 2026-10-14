# `sysc update` implementation plan

> For the implementing engineer: REQUIRED SUB-SKILL: Use `superpowers:executing-plans` to implement this plan task by task after the owner approves the paired design.

**Goal:** give an installed suite a `sysc update` verb that fetches the current release installer, verifies it, hands the work to it, and prints a per-component version delta — on the existing stop-swap-stamp pipeline, with no second installer.

**Architecture:** two roles in one binary. The parent (any installer that has the verb) reads the stamp, refuses without one, downloads `releases/latest/download/sysc-linux-<arch>` plus `SHA256SUMS` through the existing fetch client, verifies, and runs the fetched binary with `update --apply`. The child (the freshly fetched binary, the only party that knows the new pin) prints the delta, asks for confirmation, and calls `install.Run`. The parent never swaps a binary and never writes the stamp.

**Tech stack:** Go, stdlib only (`net/http` is already inside `internal/fetch`, `os/exec`, `encoding/json`, `sort`). No new dependency.

Kind: plan. Paired design: `2026-10-15-sysc-update-design.md`, D1–D10. Issues: `sysc-1225`, gh Nomadcxx/sysc#63. Prerequisites: `sysc-744` (report-only newer-release check, separate owner), `sysc-1158` (release and tag publication stays owner-gated). This plan authorizes product code only after the owner approves the design.

## Execution entry and boundaries

- Baseline: `sysc` origin/main `466106d`. Work in a dedicated worktree branched from `origin/main`, never in the primary checkout.
- `docs/plans/` is tracked in this repository; the register row travels with each document.
- Gate per task, from `2026-10-05-sysc-suite-installer-design.md:187`:

```bash
gofmt -l .            # empty
go vet ./...
go test ./internal/update -count=1
go test ./cmd/syc -count=1
go test ./internal/seed -count=1
go test ./internal/niri -count=1
git diff --exit-code -- go.mod go.sum
```

Never `go test ./...`. Never `-race`: the suite's tests run one package at a time by name.

## Task 1. The delta and the checksum reader

Files: `internal/update/update.go` (new), `internal/update/delta_test.go` (new).

Design D3.

RED, in `delta_test.go`:

```go
func TestDelta(t *testing.T) {
	cases := []struct {
		name string
		prev stamp.Stamp
		want []string
	}{
		{"changed", stamp.Stamp{Release: "v0.1.1", Components: map[string]string{"sysc-shell": "v0.1.0"}},
			[]string{"~ sysc-shell  v0.1.0 -> v0.2.0"}},
		{"added", stamp.Stamp{Release: "v0.1.1", Components: map[string]string{}},
			[]string{"+ sysc-shell  v0.2.0"}},
		{"unchanged", stamp.Stamp{Release: "v0.1.2", Components: map[string]string{"sysc-shell": "v0.2.0"}},
			nil},
		{"legacy stamp without versions", stamp.Stamp{Release: "v0.1.0", Components: map[string]string{"sysc-shell": ""}},
			[]string{"? sysc-shell  unknown -> v0.2.0"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rows := Delta(tc.prev, pinFromTestdata(t))
			got := make([]string, len(rows))
			for i, r := range rows {
				got[i] = r.String()
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}
```

`pinFromTestdata` reads `internal/pin/pin.json` so the test follows the real pin instead of a second hand-written pin. Add the removed case (`- sysc-notify  v0.1.1`) with a pin that disables the component.

```go
func TestChecksumFor(t *testing.T) {
	sums := []byte("aaa  sysc-linux-amd64\nbbb  sysc-linux-arm64\n")
	got, err := ChecksumFor(sums, "sysc-linux-amd64")
	if err != nil || got != "aaa" {
		t.Fatalf("got %q %v", got, err)
	}
	if _, err := ChecksumFor(sums, "sysc-linux-x86"); err == nil {
		t.Fatal("missing asset must refuse")
	}
}
```

Add refusal cases: a 63-character digest, a non-hex digest, CRLF line endings, and extra columns.

Expected RED: `undefined: Delta`, `undefined: ChecksumFor`.

GREEN: `Row{Kind, ID, Old, New}` with `String()` producing exactly the shapes above; `Delta` iterates the union of pin component ids and stamp component ids, sorted, skipping pin components marked `Disabled`. `ChecksumFor` splits on whitespace, requires two fields, requires 64 hex characters, and returns the asset name as an error when it is absent.

```bash
go test ./internal/update -run 'TestDelta|TestChecksumFor' -count=1   # PASS
```

Commit `feat: add the update delta and the checksum reader`.

## Task 2. The verb and the delegate

Files: `cmd/syc/main.go` (:52 usage, :719 dispatch), `cmd/syc/update_test.go` (new).

Design D1, D2 (child half), D6.

RED, in `update_test.go`:

```go
func TestRunUpdateRefusesWithoutStamp(t *testing.T) {
	var out, errOut bytes.Buffer
	code := runUpdate([]string{"--apply"}, strings.NewReader(""), &out, &errOut, t.TempDir(), neverRun)
	if code != 1 {
		t.Fatalf("code = %d", code)
	}
	if !strings.Contains(errOut.String(), "No SYSC install stamp at") {
		t.Fatalf("out = %q", errOut.String())
	}
}
```

`neverRun` is the injected `install.Run` seam; a test that reaches it fails loudly. Two more cases: `--apply` at the newest pin prints `already at v0.1.3` and exits 0 with `neverRun` untouched; a declined confirmation prints `Aborted.` and exits 1.

RED expected: `undefined: runUpdate`.

GREEN: add `parseUpdate(args []string, out io.Writer) (options, error)` mirroring `parseUninstall`'s flag set plus `--apply`, a refusal string for `fs.NArg() != 0`, and the third usage line in `parseFlags`. `runUpdate` dispatches before `parseFlags` in `main()`, exactly like `uninstall`:

```go
if len(os.Args) > 1 && os.Args[1] == "update" {
	os.Exit(runUpdate(os.Args[2:], os.Stdin, os.Stdout, mustHome()))
}
```

`runUpdate` reads the stamp from `stateHome()` (existing helper), refuses without one, builds the delta against `pin.Load()`, prints the header `SYSC update <old> -> <new>` and the rows, returns 0 on an empty delta, confirms with `Proceed? [y/N] ` unless `--yes`, recovers answers (Task 4), and calls the injected runner with `installOptions(home, p, answers, yes, i18n.Match(o.Lang), osr, runPackage)`, then `printTasks(out, res)` and `anyFailed(res.Tasks)`.

```bash
go test ./cmd/syc -run 'TestRunUpdate|TestUsage' -count=1   # PASS
```

Commit `feat: add the update verb and its delegate mode`.

## Task 3. The parent: fetch, verify, hand over

Files: `internal/update/run.go` (new), `internal/update/run_test.go` (new).

Design D2 (parent half), D6.

RED:

```go
func TestRunRefusesChecksumMismatch(t *testing.T) {
	dir := t.TempDir()
	client := &fakeClient{sums: "aaa  sysc-linux-amd64\n", body: []byte("not the installer")}
	code := Run(context.Background(), Options{StateDir: dir, Out: io.Discard, Arch: "x86_64",
		Client: client, Base: "https://example.invalid", Run: func(context.Context, string, ...string) int {
			t.Fatal("child must not start after a checksum mismatch")
			return 0
		}})
	if code == 0 {
		t.Fatal("checksum mismatch must exit nonzero")
	}
	if _, err := os.Stat(filepath.Join(dir, "staging")); err == nil {
		t.Fatal("staging must be removed")
	}
}
```

Add: `TestRunRefusesWithoutStamp` (exit 1, `Run` never called), `TestRunPropagatesChildExit` (fake child returns 3, `Run` returns 3), `TestRunRemovesTempDir` on both success and failure, `TestRunSkipsNonAmd64`.

GREEN: `Options{StateDir, Out, ErrOut, Arch, Client, Base, Run}`; `Run` reads the stamp, refuses on absence, refuses non-amd64, creates `filepath.Join(StateDir, "staging", "update")` (removed on every exit), downloads `Base + "/sysc-linux-" + Arch` and `Base + "/SHA256SUMS"` with the existing `fetch.Client` as two `fetch.Asset` values in one `DownloadAll` call, `os.Chmod(path, 0o700)`, and calls the injected `Run(ctx, path, "update", "--apply", "--yes")`. The child's stdout and stderr are inherited, so the delta and task list reach the terminal unchanged. `Base` defaults to `https://github.com/Nomadcxx/sysc/releases/latest/download` and is an option only so the test never touches the network.

```bash
go test ./internal/update -run 'TestRun' -count=1   # PASS
```

Commit `feat: fetch and hand the update to the current installer`.

## Task 4. Answers recovered from the user's config

Files: `internal/seed/answers.go` (new), `internal/seed/answers_test.go` (new), `cmd/syc/update.go` (or the `runUpdate` body).

Design D4.

RED:

```go
func TestAnswersFromConfig(t *testing.T) {
	cases := map[string]struct {
		body    string
		lat     float64
		lon     float64
		place   string
		wantErr bool
	}{
		"full":         {`{"weather":{"latitude":52.52,"longitude":13.405,"location":"Berlin"}}`, 52.52, 13.405, "Berlin", false},
		"partial":      {`{"weather":{"latitude":52.52}}`, 52.52, 0, "", true},
		"no weather":   {`{"theme":{"preset":"compact"}}`, 0, 0, "", true},
		"malformed":    {`{`, 0, 0, "", true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			a, err := AnswersFromConfig(writeTemp(t, tc.body))
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v", err)
			}
			if err == nil && (a.Latitude != tc.lat || a.Longitude != tc.lon || a.Location != tc.place) {
				t.Fatalf("got %+v", a)
			}
		})
	}
}
```

GREEN: `AnswersFromConfig(path string) (Answers, error)` reads the file, decodes into a struct with a `Weather` block holding `*float64` latitude, `*float64` longitude and `Location string`, and requires both coordinates plus a place label — the same requirement `ConfigJSON` enforces, so an update cannot enter the pipeline with weaker inputs than an install. A missing file is an error the caller turns into the named refusal.

```bash
go test ./internal/seed -run 'TestAnswersFromConfig' -count=1   # PASS
go test ./cmd/syc -count=1                                     # PASS
```

Commit `feat: recover installer answers from the shell config`.

## Task 5. Prove the niri apply is idempotent

Files: `internal/niri/niri.go` (unchanged unless the test fails), `internal/niri/idempotent_test.go` (new).

Design D5.

RED:

```go
func TestApplyTwiceIsByteIdentical(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config.kdl")
	os.WriteFile(cfg, []byte("spawn-at-startup \"sysc-shell\"\n"), 0o600)
	opts := Options{ConfigPath: cfg, SidecarPath: filepath.Join(dir, "sysc.kdl"),
		StateDir: filepath.Join(dir, "backups"), Binds: DefaultBinds, Now: fixedNow}
	if _, err := Apply(opts); err != nil {
		t.Fatal(err)
	}
	first, _ := os.ReadFile(cfg)
	if _, err := Apply(opts); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(cfg)
	if !bytes.Equal(first, second) {
		t.Fatalf("second apply rewrote the config:\n%s", second)
	}
}
```

GREEN: run it. If it passes, nothing changes in `internal/niri` and the update path needs no option. If it fails, add `Options.SkipNiri bool` honoured at `internal/install/install.go:570` and set it from update; do not weaken the test and do not special-case the regexes.

```bash
go test ./internal/niri -run 'TestApplyTwice' -count=1   # PASS
```

Commit `test: prove the niri apply is idempotent` (or `feat: let update skip the niri apply` when the switch is needed).

## Task 6. Two catalogue keys in nine languages

Files: `internal/i18n/catalog/*.json` (nine files), `cmd/syc/main.go`.

Design D8.

Add `refuse.stamp` ("No SYSC install stamp at %s. Run sysc to install the suite.") and `update.current` ("Already at %s.") to every catalogue, English first, then the owner's translations. Use `refuse.stamp` in the no-stamp refusal in both roles and `update.current` for the empty-delta line, so the refusal text is the same in the terminal whatever the locale.

```bash
go test ./internal/i18n -run 'TestEveryKeyExistsInAllCatalogs|TestCatalogFormatArgsMatchEnglish' -count=1   # PASS
```

Commit `feat: add the update refusal and current-version strings`.

## Task 7. Review, live matrix, documentation rows

Files: `docs-site/content/docs/start/install.mdx`, `docs-site/content/docs/guides/upgrade.mdx`.

Design D9, D10.

Local review against the orchestration list: ownership stays in `internal/install`; input validation at the checksum boundary (Task 1) and the argv boundary (Task 2) is covered by refusal tests; cancellation means the child inherits the parent's context and a `ctx.Err()` check lands before `DownloadAll`; no goroutine owner is added; the added surface is one package, one exported helper, two keys, one verb.

Live Niri matrix on this machine:

| Action | Required observation |
|---|---|
| `sysc update --help` | usage lists install, uninstall and update; exit 0 |
| `XDG_STATE_HOME=$(mktemp -d) sysc update` | `No SYSC install stamp at <path>. Run sysc to install the suite.`, exit 1, no file written in the temp dir |
| `XDG_STATE_HOME=$(mktemp -d) sysc update --apply` | same refusal |
| `sysc update --bogus` | usage error, exit 2 |
| a real suite update | **not run here**: it would replace the owner's binaries. The table tests cover the swap; record the limit in the handover rather than claiming the run |

Documentation: add the verb to the upgrade guide next to the existing "run the installer release whose pin you want" route, state that it needs an installer that ships the verb, and keep the manual one-liner for older installs. Add one line to the install page so the verb is discoverable from the route a user lands on.

```bash
gofmt -l .            # empty
go vet ./...
git diff --exit-code -- go.mod go.sum
```

Commit `docs: point the upgrade route at the update verb`.

## Rollback

Revert the seven commits. The stamp format, the lock, `install.Run` and every refusal path are unchanged, so there is nothing to migrate. Binaries swapped by a real update roll back through the pipeline's existing `.bak` mechanism or `sysc uninstall`.

## Plan audit applied

Findings from the 2026-08-26 development-orchestration gates, applied while writing this plan:

- **Exact files and anchors.** Every task names its files and the line it edits, read from origin/main `466106d`.
- **Failing checks first.** Each task opens with the literal RED test and states the expected RED message before any production code.
- **Minimal steps.** No task introduces an abstraction the design did not name; the delta, the checksum reader, the answers reader and the verb are the whole change.
- **Commands and expected results.** Each task closes with a focused `go test ./pkg -run '…'` that must pass, and the gate block is the installer repository's own (no `./...`, no `-race`).
- **Commit boundaries.** One task per commit, behaviour paired with its focused test.
- **No live claims.** Task 7 names the live rows and marks the swap row as unrunnable on this machine rather than implying it happened.
- **Stop condition.** Carried from the design (D10) and restated here: if the parent cannot be built from the fetch and exec seams, stop and re-plan around a published pin artifact.