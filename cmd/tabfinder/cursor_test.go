package main

import (
	"os"
	"testing"
)

// U-DSK-07
func TestUseDesktopCursor(t *testing.T) {
	gsettings := `case "$3" in cursor-theme) printf "'Breeze_Snow'\n";; cursor-size) printf '36\n';; esac`
	unset := func(name string) {
		t.Setenv(name, "")
		os.Unsetenv(name)
	}
	t.Run("sets both from gsettings, quotes stripped", func(t *testing.T) {
		fakeTools(t, map[string]string{"gsettings": gsettings})
		unset("XCURSOR_THEME")
		unset("XCURSOR_SIZE")
		useDesktopCursor()
		if got := os.Getenv("XCURSOR_THEME"); got != "Breeze_Snow" {
			t.Errorf("XCURSOR_THEME = %q", got)
		}
		if got := os.Getenv("XCURSOR_SIZE"); got != "36" {
			t.Errorf("XCURSOR_SIZE = %q", got)
		}
	})
	t.Run("leaves set variables alone", func(t *testing.T) {
		fakeTools(t, map[string]string{"gsettings": gsettings})
		t.Setenv("XCURSOR_THEME", "Mine")
		unset("XCURSOR_SIZE")
		useDesktopCursor()
		if got := os.Getenv("XCURSOR_THEME"); got != "Mine" {
			t.Errorf("XCURSOR_THEME = %q", got)
		}
		if got := os.Getenv("XCURSOR_SIZE"); got != "36" {
			t.Errorf("XCURSOR_SIZE = %q", got)
		}
	})
	t.Run("no gsettings: nothing set", func(t *testing.T) {
		fakeTools(t, nil)
		unset("XCURSOR_THEME")
		unset("XCURSOR_SIZE")
		useDesktopCursor()
		if _, ok := os.LookupEnv("XCURSOR_THEME"); ok {
			t.Error("XCURSOR_THEME set")
		}
		if _, ok := os.LookupEnv("XCURSOR_SIZE"); ok {
			t.Error("XCURSOR_SIZE set")
		}
	})
	t.Run("empty answer: nothing set", func(t *testing.T) {
		fakeTools(t, map[string]string{"gsettings": `printf "''\n"`})
		unset("XCURSOR_THEME")
		unset("XCURSOR_SIZE")
		useDesktopCursor()
		if _, ok := os.LookupEnv("XCURSOR_THEME"); ok {
			t.Error("XCURSOR_THEME set")
		}
	})
}
