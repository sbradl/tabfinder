# Changelog

All notable changes to this project are documented here.
The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added

- Difficulty per part: the notes of Guitar Pro 3–7 files are read and each song is split
  into drums, bass, rhythm and lead guitar, each with a level from 1 to 10 and tags such
  as `fast`, `syncopated`, `odd meter`, `polyrhythm`, `stretches` or `double kick`.
- `tabscan`: `-drums`, `-bass`, `-rhythm` and `-lead` filter by a range of levels, `-tag`
  by tags, `-sort easiest|hardest` orders by difficulty. A range of levels only matches
  songs that have that part. TSV has two more columns, `difficulty` and `tags`; JSON has
  the parts.
- Desktop app: each song shows its parts with a meter and the level; "Difficulty…" sets
  a range of levels per part, shown as chips under the fields; a sort menu orders the
  list A–Z, easiest first or hardest first.
- Android app: the same as the desktop app: parts with a meter on each song, "Difficulty…"
  with a range slider per part, chips, and a sort menu in the top bar.

### Changed

- The saved scan (index) starts with its version. An index of an older version is
  scanned again, once, so songs get their parts.
- A scan reads all notes now and takes longer: about 0.8 s instead of 0.2 s for a
  library of 950 songs on a desktop.

### Fixed

- Desktop app: a click beside a field closes its suggestions; only Escape did.

## [1.0.1] - 2026-10-07

### Fixed

- Android: the first scan of a folder no longer shows "Search failed: Child of the scoped
  flow was cancelled". A search dropped for a newer one isn't a failure.
- Android: right after a scan, the list no longer flashes "No tabs found" while the search
  for the new songs is still running.

## [1.0.0] - 2026-10-07

First release.

### Added

- Releases on GitHub: an Android APK signed with a release key, the Linux desktop app with
  an install script, and `tabscan` and `tabreorg` for Linux, macOS and Windows.
- Tab parsing (`internal/tab`) for Guitar Pro 3–7, TuxGuitar and Power Tab: artist, album,
  title, tracks, tunings and tempo changes. Falls back to the folder and file name when
  metadata is missing. Reads misnamed files, `.crdownload` files and single-tab `.zip`
  archives.
- Search (`internal/finder`): filters by name, artist, tuning (by name or by notes) and
  BPM range, with artist and tuning suggestions and a cached index.
- `tabscan`: TSV or JSON output with filters, and a `-serve` JSON-lines protocol for the
  Android app.
- `tabreorg`: reorganizes tabs into `<Artist>/<Album>/<Song>.<ext>`. It does a dry run by
  default and writes a Markdown summary. `-apply` also writes a move log and an undo
  script. Library-specific rules are read from `~/.config/tabreorg/config.json`.
- Desktop app `tabfinder` (Gio): a folder picker (kdialog or zenity), live filtering and
  opening songs in TuxGuitar. `mise run desktop-install` adds it to the application menu.
- Android app (Android 10+) with the same features, backed by a bundled `tabscan`. It
  opens songs in TuxGuitar for Android.
- Go tests, Android JVM tests and device tests, all with made-up test data
  (`docs/test-plan.md`).

### Fixed

- `tabreorg` stops before moving anything if it can't write the move log or the undo
  script. A rename that only changes case can no longer overwrite another file.
- Titles such as `Title - Acoustic Version.gp5` keep their title.
- The name filter no longer matches the file extension.
- A partly readable file is no longer reported as "couldn't read".
- Errors from the folder dialog, a corrupt `config.json` and index writes are now
  reported instead of being ignored.

[Unreleased]: https://github.com/sbradl/tabfinder/compare/v1.0.1...HEAD
[1.0.1]: https://github.com/sbradl/tabfinder/compare/v1.0.0...v1.0.1
[1.0.0]: https://github.com/sbradl/tabfinder/releases/tag/v1.0.0
