# Changelog

All notable changes to this project are documented here.
The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

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
