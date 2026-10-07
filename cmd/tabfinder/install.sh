#!/usr/bin/env bash
# Builds the desktop app into ~/.local/bin and adds it to the application menu
# as dev.tabsync.tabfinder (the Android app's ID). The ID is also the window
# class, which is how KDE matches the window to its menu entry and icon.
set -euo pipefail
cd "$(dirname "$0")/../.."

id=dev.tabsync.tabfinder
data="${XDG_DATA_HOME:-$HOME/.local/share}"
bin="$HOME/.local/bin/tabfinder"

go build -trimpath -ldflags="-s -w -X gioui.org/app.ID=$id" -o "$bin" ./cmd/tabfinder
icon="$data/icons/hicolor/scalable/apps/$id.svg"
install -Dm644 cmd/tabfinder/tabfinder.svg "$icon"
mkdir -p "$data/applications"
# Icon is a path, not a theme name: KWin looks the entry's icon up in the theme, and a running KWin keeps the icon-theme.cache it
# read at login, so a newly installed theme icon only shows after logging in again.
cat > "$data/applications/$id.desktop" <<DESKTOP
[Desktop Entry]
Type=Application
Name=TabFinder
Comment=Find guitar tabs by artist, tuning and tempo
Exec=$bin
Icon=$icon
Categories=AudioVideo;Audio;Music;
StartupWMClass=$id
DESKTOP
# Earlier installs used the bare name.
rm -f "$data/applications/tabfinder.desktop" "$data/icons/hicolor/scalable/apps/tabfinder.svg"

# Qt (so Plasma) trusts an existing icon-theme.cache and wouldn't see the new icon.
if [ -f "$data/icons/hicolor/icon-theme.cache" ] && command -v gtk-update-icon-cache >/dev/null; then
	gtk-update-icon-cache -f -t "$data/icons/hicolor"
fi
if command -v kbuildsycoca6 >/dev/null; then
	kbuildsycoca6 >/dev/null 2>&1 || true
fi
echo "Installed $bin"
