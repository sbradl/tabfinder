package main

import (
	"os"
	"os/exec"
	"strings"
)

// useDesktopCursor points Gio at the desktop's cursor theme and size. Gio
// draws its own cursor on Wayland from XCURSOR_THEME and XCURSOR_SIZE, which
// KDE doesn't set; without them it falls back to tiny built-in X11 cursors.
func useDesktopCursor() {
	for env, key := range map[string]string{"XCURSOR_THEME": "cursor-theme", "XCURSOR_SIZE": "cursor-size"} {
		if os.Getenv(env) != "" {
			continue
		}
		out, err := exec.Command("gsettings", "get", "org.gnome.desktop.interface", key).Output()
		if v := strings.Trim(strings.TrimSpace(string(out)), "'"); err == nil && v != "" {
			os.Setenv(env, v)
		}
	}
}
