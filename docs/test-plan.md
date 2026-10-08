# Test plan

What to test in tabfinder, for an agent to implement. Each test has an ID; tick it off
(`- [x]`) when it exists and passes. Expected behavior is described as the code
behaves today. If a test shows that behavior is wrong, stop and report it
instead of changing the test to match.

## The system under test

| Part | Path | Language | What it does |
|---|---|---|---|
| Tab parsing | `internal/tab` | Go | Reads Guitar Pro 3–7, TuxGuitar and Power Tab files; `Tuning`, tempos, artist/album/title with path fallbacks; parallel `Walk`/`ScanAll` |
| Search | `internal/finder` | Go | Song list, `Filter` and query → matches, artist/tuning suggestions, index file |
| Rows | `internal/rows` | Go | What the apps show of each song (subtitle, tunings, tempo, unreadable, file name for TuxGuitar) and the texts around the list |
| TuxGuitar naming | `internal/tuxguitar` | Go | The file name TuxGuitar can open a tab under; whether the original opens in place |
| CLI | `cmd/tabscan` | Go | TSV/JSON output with filters; `-serve` JSON-lines protocol for the Android app |
| Reorganizer | `cmd/tabreorg` | Go | Moves tabs into `<Artist>/<Album>/<Song>.<ext>`; dry run, apply, move log, undo script, Markdown summary |
| Desktop app | `cmd/tabfinder` | Go, Gio | `session` (state, no Gio) and `ui` (input, then drawing) over `internal/finder` and `internal/rows`; config, cache, folder dialog, opens TuxGuitar |
| Android app | `android/app` | Kotlin, Compose | UI; `Finder.kt` talks to the bundled `tabscan -serve` |

## Existing tests (extend, don't duplicate)

- `internal/tab/names_test.go`, `tuning_test.go`: `TuningOf`, `titleFromFilename`, `BareName`, `TitleCase`, `TempoSummary`, track JSON
- `internal/finder/filter_test.go`: `ParseBPMRange`, `Filter.Matches` incl. string count
- `internal/finder/finder_test.go`: entry order/tunings, artists, `Search`, `SuggestTunings`
- `internal/rows/rows_test.go`: row texts, tempo, counter, library line, scan summary
- `internal/tuxguitar/tuxguitar_test.go`: `FileName`/`OpensInPlace`
- `cmd/tabscan/serve_test.go`: load, search, empty lists as `[]`, unknown op
- `cmd/tabreorg/names_test.go`: `ratio` vs Python difflib, `cleanAlbum`, `nicest`, `songName`, `tabExt`
- `cmd/tabfinder/ui_test.go`: offscreen UI harness (clicks, typing, keys, screenshots); `TestInputs`, `TestReopenSuggestions`
- `cmd/tabfinder/session_test.go`: the desktop app's state without a window
- Android: JVM tests in `android/app/src/test` (`Finder` over the host tabscan, `MainViewModel` over a fake), device tests in `android/app/src/androidTest`.

## How to run

- Go: `mise run test` (= `go test ./...`). Also run `go test -race ./...`.
- Desktop screenshots: `TABFINDER_SHOTS=<dir> go test ./cmd/tabfinder`. This
  needs a GPU (EGL), so tests must pass without it, with screenshots skipped.
- Android JVM tests: `android/gradlew -p android testDebugUnitTest`.
- Android device tests: `android/gradlew -p android connectedDebugAndroidTest`
  (tablet `HVA1QC60`, Android 10, or an emulator with API 29 and one with API 34).
- `mise run test` runs the Go tests and the Android JVM tests. Device tests are a
  separate task (`mise run test-device`), since they need hardware.

## Conventions

- Test data is made up: artists, albums, songs, tab authors and e-mail addresses are invented
  (no real bands, titles or people), since real tabs are copyrighted. The same goes for the repo
  as a whole: nothing of the author's own library is in it. What `tabreorg` knows about that
  (folders to skip, album spelling fixes, junk album and artist names, notes its file names end in) is read from
  `~/.config/tabreorg/config.json` (see `cmd/tabreorg/config.go`); the tests use a made-up
  `testConfig` (`cmd/tabreorg/config_test.go`) and point the built command at it through
  `XDG_CONFIG_HOME`, so they never read the real file.
- Go: table-driven tests, `t.TempDir()`, and `testdata/` per package. Golden files
  for CLI output and summaries, with an `-update` flag to rewrite them.
- Never touch the real `~/.config/tabfinder`, `~/.cache/tabfinder`, `~/Guitar/Tabs`
  or `/sdcard/Tabs`. Point every path at temp dirs (`XDG_CONFIG_HOME`,
  `XDG_CACHE_HOME`, the `dirs` the desktop app is given: `isolate(t)` in
  `cmd/tabfinder`).
- External programs (`tuxguitar`, `kdialog`, `zenity`, `gsettings`, `xrdb`) are
  faked with shell scripts in a temp dir prepended to `PATH`. A fake records its
  arguments to a file the test reads.
- The desktop UI is driven through the harness in `cmd/tabfinder/ui_test.go`.
  Read the comments there: synthetic clicks need increasing timestamps, and
  typing must move the caret like `app.Window` does, or results are wrong.

## Fixtures

### Fixtures F1–F14 (made-up tabs, no real files)

Real tabs are copyrighted and can't be used, so F1–F14 are invented songs ("Salt Lamp" by
"Neon Harbor" and so on) built by `internal/tabfiles` and written to `internal/tab/testdata/library/`
as a small `<Artist>/<Album>/` tree. The table in `internal/tab/fixtures_test.go` holds both
the builder call and the hand-written expectation of each file; `go test ./internal/tab -update`
rewrites the files and `testdata/expected.json` from it.

| ID | Kind | File | What it covers |
|---|---|---|---|
| F1 | GP3 | `Neon Harbor/Glass Tides/Salt Lamp.gp3` | metadata in file; tempo 132 → 96 → 132 in mix tables |
| F2 | GP4 | `Quiet Engines/Slow Orbit/Tin Moon.gp4` | guitar in Drop D, 4-string bass, drums |
| F3 | GP5 (5.10) | `Seventh Floor/Static Garden/Paper Weather.gp5` | 7-string B Standard, Drop C, five tempo changes in 60 bars |
| F4 | GP5 (5.00) as `.gp3` | `Seventh Floor/Static Garden/Misnamed.gp3` | detected by content; no artist/album, so the folders give them |
| F5 | GPX, BCFZ | `Paper Satellites/Cold Start.gpx` | GP6 XML, compressed container (with back-references), drums |
| F6 | GP7 `.gp` | `Orbit Club/Night Shift.gp` | 8-string "Custom", drum kit, fractional tempo 120.5 |
| F7 | TuxGuitar 1 | `Moss Cathedral/Hollow Choir/Lantern.tg` | header only; folders give artist and album |
| F8 | TuxGuitar 2 | `Moss Cathedral/Hollow Choir/Under Glass.tg` | zip with `content.xml`; guitar, bass, drums |
| F9 | Power Tab 2 | `Dust Kites/Raw Takes/Rope Bridge.ptb` | no public release, so the folder gives the album |
| F10 | `.gpx.crdownload` | `Paper Satellites/Half Down.gpx.crdownload` | plain (uncompressed) BCFS container; no album folder |
| F11 | `.zip` around a GP5 | `Neon Harbor/Glass Tides/Zipped.zip` | parsed recursively |
| F12 | cut GP5 | `Seventh Floor/Static Garden/Cut Short.gp5` | first half of F3: header and tracks kept, some tempos, `Error` |
| F13 | GP2 `.gtp` | `Old Machines/Pre-History.gtp` | "Guitar Pro 2 files are not supported" |
| F14 | Power Tab 1.0 | `Old Machines/Ancient Riff.ptb` | "Power Tab 1.0 files are not supported" |

Since the builders are written from the same format knowledge as the parsers, a shared
misreading of a format would go unseen: the real-file check stays on the manual list below.

### Synthetic fixtures (no decision needed)

- Index files (`tabscan -json` lines), like `sampleIndex` in `ui_test.go`. A
  `testlib` helper should build a library in JSON from Go structs: artists with
  case variants ("Inkwell Flamingos"/"INKWELL FLAMINGOS"), a custom tuning, 4–9 string tracks,
  drums, tempo changes, an unreadable entry, non-ASCII ("Die Äther"), long titles.
- A fixture **tree** for `tabreorg` and scanning: files from the table above
  laid out as `Artist/Album/x`, `Artist/x`, `x` at the root, a skipped folder, a
  name clash, an unreadable dir (mode 000; skip when root).
- GP7 and TuxGuitar 2 files can be built in tests (zip + XML) for edge cases:
  missing tracks, no tempo automation, odd tunings.

## Small refactors needed for testability

Make these first, each in its own commit, with existing behavior unchanged.

- R1 `android`: `Finder` hard-codes `nativeLibraryDir/libtabscan.so`,
  `filesDir` and `SharedPreferences`. Add a constructor taking `binary: File`,
  `dataDir: File`, and a small `RootStore` interface (get/set root). JVM tests
  can then run the host build of tabscan (`mise run cli`, `./tabscan`) as the
  real backend.
- R2 `android`: `MainViewModel` takes `Finder`. Extract an interface (`load`,
  `scan`, `search`, `root`) so view-model tests can use a fake with controllable
  delays and failures.
- R3 `android`: give key composables test tags (fields, clear buttons,
  suggestion items, section headers, counter, song rows, prompts).
- R4 `cmd/tabfinder`: inject a clock for the snackbar expiry and the
  memory release timer (`time.AfterFunc` in `setSongs`) so tests don't sleep.

## Unit tests and edge cases

### `internal/tab` (parsing and names)

- [x] U-TAB-01 `parseBytes` dispatch by **content**, not extension: empty →
  "empty file"; `PK\x03\x04` → zip; `BCFZ`/`BCFS` → GPX; `ptab` → PTB; "GUITAR"
  within bytes 1–31 → GP3–5; TuxGuitar magic → TG; anything else → "unknown file format".
- [x] U-TAB-02 Zip: `Content/score.gpif` → gp7; `content.xml` → TuxGuitar 2; a zip
  holding a tab file → parsed recursively; zip in zip; no tab inside → "zip
  contains no tab file"; corrupt zip → error, no panic.
- [x] U-TAB-03 Each fixture F1–F14 against `expected.json`: format, artist,
  album, title, title/artist source (`file`/`path`), tracks (name, instrument,
  drums, pitches, tuning label), tempos (bar and BPM, repeats collapsed).
- [x] U-TAB-04 Truncated/corrupt files (F12, random truncation of every fixture at
  10 points): `Scan` returns a song with `Error` set, never panics, and keeps
  what it read before the cut.
- [x] U-TAB-05 `reader`: `need` past end, `u8/u16/i32` at end of buffer,
  negative/huge length prefixes in `byteSizeString`/`intByteSizeString`.
- [x] U-TAB-06 `decodeText`: UTF-8, Latin-1 (Ä, ö, ß), invalid UTF-8.
- [x] U-TAB-07 `tuningName`: standard and drop for 4, 5, 6, 7 strings; 8 and 9
  strings → "Custom"; empty pitches → ""; drop with other strings off → Custom;
  negative MIDI notes (`noteName` modulo); enharmonic spelling as printed (C#, Eb,
  F#, Ab, Bb).
- [x] U-TAB-08 `titleFromFilename`, beyond the existing cases: `.crdownload` and
  `.zip` suffixes, "Other Artist - Title" when the artist differs, artist given
  as "Last, First", `(ver 2)`/`v3`/`(Pro)`/`(complete)` tags, " - 779" ids,
  junk brackets (`[by x@y.com]`, `(Tabbed By … www.…)`), snake_case → Title Case,
  names that are only junk, empty name.
- [x] U-TAB-09 `applyFallbacks`: no metadata → artist/album from
  `<Artist>/<Album>/` path components, title from the file name; file at root
  (no dirs); one level only (artist, no album); metadata present → path
  ignored; sources set to `file`/`path` correctly.
- [x] U-TAB-10 `Scan` path: relative to root; file outside root → path kept as
  given; slashes normalized.
- [x] U-TAB-11 `Walk`: directory recursion; single file argument (root = its
  dir); explicit root; non-tab files ignored; `IsTabFile` case-insensitive
  extensions incl. `.GP5`; nonexistent argument → error; unreadable subdir →
  error naming the path.
- [x] U-TAB-12 `ScanAll`: same songs as `Walk` in the same order (compare against
  `Walk` on the fixture tree); empty dir → no songs, no error; unreadable dir →
  songs so far plus an error; root that is a file. Run with `-race`.
- [x] U-TAB-13 `addTempo`/`TempoSummary`: repeats of the current tempo dropped,
  return to an earlier tempo kept in the list but shown once in the summary,
  fractional BPM (`120.5`).
- [x] U-TAB-14 `Filter.Matches`, beyond existing cases: name words in any order
  across title + file name (not its extension); case-insensitive; artist substring (not trimmed in
  `finder.Filter`); tuning by name and by notes, enharmonics per word
  ("Drop Db" = "drop c#"); "standard" alone = "E Standard"; `Strings` without
  tuning; tuning and strings must match the **same** track; drums ignored;
  songs without tracks/tempos; `BPM` range boundaries inclusive; `Active()`.
- [x] U-TAB-15 `ParseBPMRange`: whitespace around numbers, `" 100 - 140 "`,
  decimals, `"-"`, `"--5"`, `"100-"`, `"-0"`, negative ("-5" means at most 5),
  min > max → error.
- [x] U-TAB-16 Fuzz `parseBytes` (`FuzzParseBytes`, seeded with all fixtures):
  no panic, returns within 1 s for inputs up to 1 MB.
- [x] U-TAB-17 Fuzz `titleFromFilename` and `ParseBPMRange`: no panic;
  `ParseBPMRange` either errors or returns min ≤ max.

### `internal/finder`

- [x] U-FIN-01 `New`: sort by artist then title, case-insensitive, stable for
  equal keys; empty/nil input → empty library (not nil slices).
- [x] U-FIN-02 `newEntry`: duplicate tunings across tracks collapsed;
  same name with different string counts kept apart; order most strings first;
  drums and tracks without tuning skipped; tuning string without " (" →
  notes ""; BPM formatting (`190`, `120.5`); duplicates removed in first-seen order.
- [x] U-FIN-03 `Tuning.Label`: "Custom" → notes; "Custom" with empty notes → "Custom".
- [x] U-FIN-04 `artistsOf`: case variants merged into the most common spelling;
  tie → alphabetically first; blank/whitespace artists skipped; leading/trailing
  spaces trimmed; A–Z order case-insensitive.
- [x] U-FIN-05 `Query.Filter`: trims name/artist/tuning; blank BPM → no filter;
  invalid BPM → `bpmInvalid` and BPM ignored; `Strings` passed through; `Active`
  true for each single field.
- [x] U-FIN-06 `Search` tunings: computed from songs matching everything
  **except** tuning and strings (picking an artist trims them, typing a tuning
  doesn't); grouping order 6, 7, 8, 9, then 4, 5; most common first; ties by
  label; no matches → `[]`.
- [x] U-FIN-07 `SuggestArtists`: prefix matches before substring matches; exact
  match (any case) excluded; empty input → all artists; capped at
  `MaxSuggestions` (60); never nil.
- [x] U-FIN-08 `SuggestTunings`: matches label or notes, case-insensitive;
  keeps grouping; capped at 60; never nil.
- [x] U-FIN-09 `LoadIndex`: missing file → `nil, nil`; blank lines skipped;
  malformed line → error; line > 64 KB and up to 16 MB accepted; unknown JSON
  fields ignored; file not readable → error.
- [x] U-FIN-10 `ScanIndex`: writes via `.tmp` and rename (no partial index on
  failure, old index kept); creates the parent dir; root unreadable with no
  songs → error and no file written; partial scan → songs, file and error.
  `LoadIndex(ScanIndex(...))` round-trips equal songs.
- [x] U-FIN-11 `tuxguitar.FileName`/`OpensInPlace`: every format mapping (gp3, gp4,
  gp5, gp6→gpx, gp7→gp, tg, ptb); empty format → path's extension lowercased;
  title with dots, slashes, emoji, only symbols → "song"; extension check
  case-insensitive (`.GP5`); `.gpx.crdownload` not in place.

### `cmd/tabscan`

- [x] U-CLI-01 `tsvWriter`: header; tabs and newlines in fields replaced by
  spaces; instrument shown as `Name [Instrument]` only when it differs
  (case-insensitive); empty tuning → `-`; tracks joined with `; `.
- [x] U-SRV-01 `serve`, request by request: `load` with missing index → `{"songs":[]}`;
  `scan` of an empty dir; `scan` with an unreadable subdir → songs plus
  `warning`; `search` before any `load` → empty results, not an error;
  malformed JSON line → `{"error":…}` and the session continues; unknown op;
  empty line; very long line (up to 1 MB); matches are paths in the latest `load`/`scan`.
- [x] U-SRV-02 Every list in every response is `[]`, never `null` (`songs`,
  `matches`, `artists`, `tunings`, a song's `tunings`/`bpms`). Check with a JSON
  scan for `null`.
- [x] U-SRV-03 `rows.Song`: `unreadable` only when nothing could be read
  (`tab.Song.Unreadable`: an error and no format), not for a partly read file, tunings or
  not; `openAs` = `tuxguitar.FileName`; `rows.Tuning.detail` empty when
  label == notes (custom), else the notes.
- [x] U-SRV-04 Fuzz `serve` with random lines: no panic, one output line per
  input line.
- [x] U-SRV-05 The Android app's filter and suggestion cases (E-AND-04, E-AND-05) over the
  protocol, on the app tests' library (`appLibrary`, the same songs as `Fixture` in
  `android/app/src/sharedTest`): each field alone, all together, without the tuning, an
  invalid tempo range (all songs, `bpmInvalid`), no match, a picked tuning with its string
  count and typed on without it; an artist suggested once whatever its spellings, a picked
  artist's songs deciding the tunings, a tuning suggested with its string count.

### `cmd/tabreorg`

- [x] U-REO-01 `key`, `cleanAlbum`, `similar`, `nicest`, `filenameKey`,
  `songKey`, `songName`, `tabExt` (extend existing tests): year
  prefixes/parentheses, articles ("The", "Die"), tuning suffixes in file names
  (`_drop_c`, `7string`, `withbass`…), version tags, junk album names
  (`single`, `s/t`, `unknown`, `(…)`), `albumAliases`, `junkArtists`, `fileSuffixes`.
- [x] U-REO-02 `planner.plan`, one case per rule in its doc comment: files in
  `<Artist>/<Album>/` keep their folder; loose files in an artist folder go to
  the album named in the file, matched loosely against existing album folders,
  else stay; loose root files go to their artist's folder, else stay; rename
  to the song name; same song name twice in one folder → both kept, reported as
  a clash; other collisions → " (2)"; nothing overwritten; skipped folders
  untouched; artist matched to an existing folder by similarity (the "Felix
  Ferien under Die Äther" case); albums without a folder merged by similarity.
- [x] U-REO-03 `moves`: no move for files already in place; moves in the order
  of the placements (scan order), so the same tree always gives the same list.
- [x] U-REO-04 `shellQuote`: quotes, spaces, `$`, backticks, newlines, umlauts.
- [x] U-REO-05 `splitList`: empty, spaces, trailing comma.
- [x] U-REO-06 `writeSummary` (golden file): sections "Files without album",
  "Not renamed: same song name in one folder", "Duplicate songs" (by MD5), dry
  run vs applied wording, counts match.

### `cmd/tabfinder` (desktop, non-UI)

- [x] U-DSK-01 `xdgDir`: env set/unset.
- [x] U-DSK-02 `loadRoot`/`saveRoot`: no config → ""; config.json
  round-trip; corrupt config.json → "" and an error naming the file; directory created with 0755.
- U-DSK-03 *(dropped with the earlier app's `settings.properties`)*
- [x] U-DSK-04 `pickFolder` with fake dialogs on `PATH`: kdialog present → its
  output trimmed; kdialog exits 1 (cancel) → `"", nil`, other failures → error with stderr; only zenity →
  zenity used with `--directory`; neither → error mentioning both; empty start →
  home dir passed.
- [x] U-DSK-05 `openInTuxGuitar` with fake `tuxguitar`: in place for correctly
  named files (original path passed); copy under
  `<cache>/open/<tuxguitar.FileName>` for misnamed ones, overwriting an older copy;
  source missing → error; `tuxguitar` missing → "TuxGuitar is not installed";
  the process is reaped (no zombie after `Wait`).
- [x] U-DSK-06 `prefersDark` with fake `gsettings`: `'prefer-dark'` → dark;
  `'default'` → light; command missing or empty output → not ok (light palette).
- [x] U-DSK-07 `useDesktopCursor`: sets
  `XCURSOR_THEME`/`XCURSOR_SIZE` from gsettings with quotes stripped; leaves
  already-set variables alone.
- [x] U-DSK-08 `palette.badge`: ≤5 bass, 6 guitar, ≥7 extended; light and dark.
- [x] U-DSK-09 `flow` layout: wraps at max width, gap applied, single item
  wider than the row placed alone.
- [x] U-DSK-10 `newShaper`: Barlow Condensed (Medium, SemiBold), Roboto and Go Mono
  resolve by typeface name (shape a string with each, assert the face used).
  No system fonts are loaded.

### Android JVM tests (`android/app/src/test`)

- [x] U-AND-01 `Finder` against the real host tabscan (after R1): `load` with no
  index → empty; `scan` of a temp fixture tree → songs sorted, the
  `summary`, `warning` for an unreadable dir; `search` round-trip; Go returning
  `{"error":…}` → exception with that message; `chooseRoot` deletes the index.
- [x] U-AND-02 `Finder` restart: kill the tabscan process between calls →
  next `search` restarts it, replays `load`, returns correct results; process
  that exits immediately (fake binary) → error naming the last log line.
- [x] U-AND-03 JSON tolerance: `null` lists decode as empty
  (`coerceInputValues`), unknown fields ignored.
- [x] U-AND-04 `Query.active` for each field.
- [x] U-AND-05 `Finder.treeToPath` (Robolectric or plain `Uri` fake):
  `primary:Music/Tabs` → `/storage/emulated/0/Music/Tabs`; `1234-ABCD:Tabs` →
  `/storage/1234-ABCD/Tabs`; tree root `primary:` → storage root; other
  authority (Downloads) → null.
- [x] U-AND-06 `MainViewModel` with a fake finder (after R2), using
  `kotlinx-coroutines-test`: `results` null until loaded; cached empty + root
  set → automatic rescan; `rescan` ignored while scanning or without root;
  `setRoot` clears songs and rescans; scan success message
  "N tabs, M unreadable"; scan warning shown instead; scan failure
  "Scan failed: …"; search failure → message and empty results; fast input
  changes → only the last search's result is applied (`mapLatest`);
  `clearFilters` resets the query; `messageShown` clears the message.
- [x] U-AND-07 The screen (`MainScreenTest`, Robolectric on the tablet's screen size) over the
  real view model, `Finder` and host tabscan scanning `Fixture`: the access and folder prompts
  and their buttons; the list with what tabscan worked out; each filter field alone and
  cleared, all together, no matches cleared with "Clear filters"; picking an artist (field
  filled, tunings trimmed) and a tuning (string count, dropped when typed on); fast typing;
  tapping a song opens it and shows why it couldn't; a rotation (saved state restored, same
  view model) keeps list and filters; after process death (new view model, saved state
  restored) the saved scan is back without scanning; another folder replaces the list.
- [x] U-AND-08 `TuxGuitar.open` (Robolectric): the intent (`ACTION_VIEW`, TuxGuitar's package,
  read permission) with a FileProvider URI of a copy named `openAs` that has the original's
  bytes; a song deleted after the scan → message, no intent; TuxGuitar not installed →
  "TuxGuitar is not installed".
- [x] U-AND-09 The folder picker's answer (`folderPicked`, Robolectric): a folder on the
  device's storage becomes the tab folder and is scanned; one without a path → message;
  none picked → nothing changes.
- [x] U-AND-10 `listContent`: "No tabs found" by the library, not by a search still running.

## End-to-end scenarios

### CLI: `tabscan`

Build once per test run (`go build -o $TMP/tabscan ./cmd/tabscan`), run on the
fixture tree, compare against golden files.

- [x] E-CLI-01 `tabscan <tree>` → TSV golden; exit 0.
- [x] E-CLI-02 `-json` → one JSON object per tab, all fields.
- [x] E-CLI-03 Each filter alone and combined (`-name`, `-artist`, `-tuning`
  by name and notes, `-bpm` all four forms): correct rows, and
  "N of M tabs match" on stderr only when a filter is set.
- [x] E-CLI-04 `-bpm fast` → exit 2, message on stderr.
- [x] E-CLI-05 Several arguments incl. a missing one → the others still
  printed, exit 1.
- [x] E-CLI-06 Single file argument and `-root` changing the relative paths and
  the Artist/Album fallback.
- [x] E-CLI-07 No argument → scans the current directory.
- [x] E-CLI-08 Files with errors → `warn: path: error` on stderr, row still printed.
- [x] E-CLI-09 `-serve` as a process: drive `load`/`scan`/`search` over pipes
  like the Android app; close stdin → exit 0; check it stays responsive across
  100 searches (each < 5 ms on the fixture library).

### CLI: `tabreorg`

- [x] E-REO-01 Dry run on a copy of the fixture tree: no file moved (compare
  tree hashes before/after); summary written to `<root>-reorg-summary.md`
  matching the golden file; stderr "N moves/renames planned, K name clashes; summary: …".
- [x] E-REO-02 `-apply`: files end up where the plan said; nothing overwritten
  (file count and MD5 set unchanged); `<root>-moves-<stamp>.tsv` lists every
  move; stderr says "done"; summary says applied.
- [x] E-REO-03 Run the generated undo script → tree identical to before
  (paths and MD5s).
- [x] E-REO-04 `-rename=false`: folders change, file names don't.
- [x] E-REO-05 `-skip` with custom folders: untouched; default skip list used
  when the flag is absent.
- [x] E-REO-06 Second `-apply` on the result → no moves (idempotent).
- [x] E-REO-07 Wrong arguments (none, two) → usage, exit 2; nonexistent root → exit 1.

### Desktop app (`cmd/tabfinder`, offscreen harness)

Extend the harness with: fake tools on `PATH` (U-DSK-04/05/06), a fixture
library (synthetic index, or the real fixture tree for scans), helpers to find
elements by layout instead of hard-coded coordinates (expose field and row
rectangles from `ui` in tests), and screenshots per step when `TABFINDER_SHOTS`
is set. Assert on `ui` state and, where it matters, on pixels (e.g. the menu
is drawn above the list).

**First run and library**
- [x] E-DSK-01 No config → "Choose your tab folder" prompt; folder and rescan
  buttons enabled/disabled correctly; clicking "Choose folder" calls the fake
  kdialog; picking a folder saves `config.json`, deletes the old index, scans,
  shows the list and "N tabs, M unreadable, in X s".
- [x] E-DSK-02 Cancel the folder dialog → nothing changes.
- [x] E-DSK-03 No dialog program → message "install kdialog or zenity…".
- E-DSK-04 *(dropped with the earlier app's `settings.properties`)*; a corrupt `config.json` is reported on start.
- [x] E-DSK-05 Cached index present → list shown without scanning;
  title shows "N TABS IN <FOLDER>".
- [x] E-DSK-06 Folder set but no index → automatic scan with progress bar;
  rescan button disabled while scanning; list appears after.
- [x] E-DSK-07 Empty folder → "No tabs found" with a Rescan button.
- [x] E-DSK-08 Corrupt index → message "Couldn't read the saved scan…", app usable.
- [x] E-DSK-09 Rescan button → new songs appear, counter updates, snackbar.

**Filtering**
- [x] E-DSK-10 Each field alone narrows the list; counter "x / y" right-aligned
  and padded to the total's width (same pixel width for 0 and y).
- [x] E-DSK-11 All fields combined; clearing one with its ✕ widens the list
  and refocuses that field.
- [x] E-DSK-12 Invalid BPM ("fast") → red outline, filter ignored; valid range
  → outline back to normal.
- [x] E-DSK-13 No matches → "No tabs match" → "Clear filters" empties every field
  and the query.

**Suggestions**
- [x] E-DSK-14 Artist: focusing opens all artists; typing narrows them with
  prefix matches first; exact match disappears; case variants shown once.
- [x] E-DSK-15 Artist: click a suggestion → field set, menu closed, list
  filtered; the field's change event doesn't reopen the menu.
- [x] E-DSK-16 Enter picks the first suggestion; Escape closes; Down and a click
  on the focused field reopen (existing `TestReopenSuggestions`, extend to tuning).
- [x] E-DSK-17 Tuning: grouped under "N STRINGS" headers in the order 6, 7, 8, 9,
  4, 5; notes shown right-aligned; custom tunings show their notes as the label
  and no detail; typing filters by label or notes.
- [x] E-DSK-18 Tuning pick → badge with the string count in the field; list
  limited to that tuning on that string count; typing in the field afterwards
  removes the badge and the string-count filter.
- [x] E-DSK-19 Picking an artist trims the tuning suggestions to that artist's.
- [x] E-DSK-20 Menu above the song list: clicks on menu items, headers and
  dividers never open a song underneath.
- [x] E-DSK-21 More than 60 matches → 60 shown, menu scrolls with the wheel,
  height capped.
- [x] E-DSK-22 Retype after a pick (delete a character, retype it) → the query
  follows the text.

**Song list**
- [x] E-DSK-23 Row content: title, "artist · album" (album missing → no dot),
  tuning tags with badge colors by range, tempo "190" with "BPM" or
  "→ 145 158 …" (at most two more), unreadable → "Couldn't read this file".
- [x] E-DSK-24 Click a correctly named song → fake tuxguitar got the original
  path; misnamed song → got the copy in `<cache>/open`.
- [x] E-DSK-25 tuxguitar missing → snackbar "TuxGuitar is not installed",
  disappears after 4 s (injected clock, R4).
- [x] E-DSK-26 Hover highlights a row; scrolling a 1000-song list renders only
  visible rows (frame time < 16 ms in the harness).

**Appearance and environment**
- [x] E-DSK-27 gsettings dark → dark palette; light → light palette; missing →
  light (screenshot pixel checks on background and accent).
- [x] E-DSK-28 Floating labels: hint inside the empty unfocused field; label on
  the outline when focused or filled; never covering the text (pixel check:
  text row not painted with the panel color).
- [x] E-DSK-29 Window sizes: at 360 dp and 2000 dp width the filter rows lay out
  without overlap.
- [x] E-DSK-30 Memory: after loading 2000 synthetic songs and 3 seconds, live
  heap < 30 MB (`runtime.ReadMemStats` after `FreeOSMemory`).

**Difficulty**
- [x] E-DSK-31 "Difficulty…" opens a popup with a range slider (1–10) per role;
  dragging a thumb or clicking the track narrows the list at once; Escape, Done
  and a click beside the popup close it, and that click opens no song; a click
  on the popup itself doesn't; Reset clears every range.
- [x] E-DSK-32 A chip per role with a range, in a line after the button; its ✕
  drops that range only; "Clear filters" drops them all.
- [x] E-DSK-33 Sort menu in the top bar: A–Z, easiest first, hardest first (songs
  without parts last); the button shows the order; a click beside the open menu
  closes it and changes nothing.
- [x] E-DSK-34 A row shows per part its instrument, a meter of five rising bars
  (two levels each, half a bar for an odd level) colored green for 1 to red for
  10, and the level; right of the tuning tags, or below them when both don't fit
  (360 dp); a song without parts shows none.

### Android app (device tests, `android/app/src/androidTest`)

Only what needs a device runs on one: Android's storage access (`AccessTest`) and one end-to-end
pass through the bundled tabscan, the FileProvider and a restart (`SmokeTest`). The rest of each
scenario is checked lower down, where it's faster and steadier: the matching in Go (U-SRV-05), the
screen and the TuxGuitar hand-over on the JVM with Robolectric (U-AND-07 to U-AND-09). Each item
below says where. The device tests write `Fixture` into the app's external files folder, grant
storage access (API 29: the permission; API 30+: `appops set … MANAGE_EXTERNAL_STORAGE allow`) and
set the folder through the preferences.

- [x] E-AND-01 *(device: `AccessTest`; the prompts and their buttons: U-AND-07)* *(the no-access prompt runs on a fresh install with `withoutAccess=true`, see `mise run test-device`, tapping Allow in the system dialog on API 29; the API 30+ part, the button opening the all-files settings, runs in the same fresh-install run on CI's API 34 emulator: switching the access off from a test would kill the test's process)* No access → "Allow file access"; on API 29 the button requests
  `READ_EXTERNAL_STORAGE`; on API 30+ it opens the all-files settings screen
  (intent verified); after granting and returning, the folder prompt shows.
- [x] E-AND-02 *(device: `SmokeTest`; the list: U-AND-07; the snackbar text: U-AND-06)* Folder set, no index → scan runs (`tabscan -serve` child process
  exists), list appears, snackbar "N tabs, M unreadable".
- [x] E-AND-03 *(device: `SmokeTest`; U-AND-07)* Restart the app → list from the saved index without scanning.
- [x] E-AND-04 *(U-SRV-05 and U-AND-07; one search through the bundled tabscan in `SmokeTest`)* Filters: same cases as E-DSK-10 to E-DSK-13, through the Go backend.
- [x] E-AND-05 *(U-SRV-05 and U-AND-07)* Suggestions: same cases as E-DSK-14 to E-DSK-19 (artist list,
  tuning headers, pick → badge, trimming by artist).
- [x] E-AND-06 *(U-AND-07)* Fast typing (20 characters, no delay) → field text exactly as
  typed, final result matches the last query (regression for the
  "Amber Marshamber" observation, which could not be reproduced).
- [x] E-AND-07 *(device: `SmokeTest`; U-AND-08, with the "not installed" case)* Tap a song → intent to `app.tuxguitar.android.application`,
  `ACTION_VIEW`, read-permission flag, `content://dev.tabsync.tabfinder.files/…`
  URI whose file is named `openAs` and has the original's bytes. Without
  TuxGuitar installed → "TuxGuitar is not installed".
- [x] E-AND-08 *(U-AND-08; the message on screen: U-AND-07)* Song file deleted after the scan → tapping it shows an error
  message, no crash.
- [x] E-AND-09 *(U-AND-02, which kills the real host tabscan between calls)* Kill the tabscan child (`adb shell kill`) → next keystroke still
  returns correct results (restart and replay).
- [x] E-AND-10 *(U-AND-07: a rotation keeps the view model and restores the saved state; process death brings a new view model and restores the saved state; the restart in `SmokeTest` reads the saved scan on a device)* Rotation and process death (`adb shell am kill` in background) →
  list and filters come back without errors.
- [x] E-AND-11 *(U-AND-09 and U-AND-07)* Choose another folder (folder button with a stubbed picker
  result) → old index dropped, new scan.
- [ ] E-AND-12 *(`scripts/perf-device.sh`, written, not run: it drives your real release app)* Performance on the tablet: cold start to list < 1 s with a
  950-song index; scroll jank < 5% (`dumpsys gfxinfo`) on a release build
  compiled with `cmd package compile -m speed` (see `mise.toml`).

## Status and findings

Implemented in this round: everything except E-AND-12 (performance on the tablet). Shared helpers: `internal/tabfiles` (synthetic tab file builders),
`internal/testlib` (fixture library, fixture tree, golden files with `-update`).
Refactors done: R1 (`Finder(binary, dataDir, RootStore)`), R2 (`TabSource`), R3 (test tags), R4
(the desktop app's `clock`, now passed in); plus small ones for testability: `ui.subtitle/counter` (texts now in `internal/rows`)
and the pure `Finder.treeToPath(authority, documentId, primary)`. Debug builds got
`applicationIdSuffix = ".debug"` so device tests never touch the real app's folder and scan.

Bugs the tests found, all fixed (the tests were not changed to match):

1. `ParseBPMRange` accepted `NaN` (now `internal/finder/filter.go`).
2. `titleFromFilename` cut the artist prefix at offsets from a lower-cased copy, wrong for invalid UTF-8 ("Weniger" became "Hr", or a panic). Now `cutPrefixFold` matches on the original string.
3. Android `Finder`: a tabscan that had died before the next call was restarted without replaying `load`, so `search` found no songs. Now `exchange` replays it.
4. Android `Song`: `tunings`/`bpms` had no default, so `coerceInputValues` couldn't read a null as empty. Defaults added.

Still open: E-AND-12 (`scripts/perf-device.sh` drives the real release app).
Device tests pin the tablet to portrait (`DeviceTest`): auto-rotate had turned it to landscape mid-run, which hid song rows and shrank the suggestion menu, failing tests at random.

Differences between this plan and the code, tests follow the code:

- No readable color scheme (`gsettings` missing, empty or failing) gives the **dark** palette, not light (U-DSK-06, E-DSK-27).
- A filter on a tuning needs the whole name ("drop c"); "drop" only suggests (it matches no song).
- The filters aren't saved over process death on Android, only the list (E-AND-10 says both come back).
- A zip inside a zip is only opened when its entry name has a tab extension other than `.zip` (U-TAB-02).
- Loose files with a named album go to a **new** album folder if none exists (not "else stay") (U-REO-02).
- The undo script leaves the folders `-apply` made, empty (E-REO-03 compares files).
- `tab.ScanAll` on a file gives the path `"."`, `Walk` gives its name.
- The artist-folder redirect in `tabreorg` compares loosely (case, articles, punctuation), not by similarity.

## Manual checks (not automatable here)

Keep as a checklist in this file, run before releases:

- Desktop: mouse cursor normal size;
  taskbar, title bar and menu icon present after `mise run desktop-install`.
- Android: launcher icon and themed icon; app works after reboot.
- Both: opening real files of every format in TuxGuitar (the fixtures are made up, so this is the only check against real files).

## Suggested order

1. Refactors R1–R4.
2. Go unit tests (U-TAB, U-FIN, U-SRV, U-CLI, U-REO, U-DSK); fuzz tests.
3. CLI and reorganizer end-to-end tests (E-CLI, E-REO).
4. Desktop end-to-end tests (E-DSK), extending the harness first.
5. Android JVM tests (U-AND), then device tests (E-AND).
6. Wire everything into `mise run test` / `mise run test-device`.
