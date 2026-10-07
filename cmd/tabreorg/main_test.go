package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"tab-sync/internal/testlib"
)

// tabreorgBin is the built command, for the end-to-end tests.
var tabreorgBin string

// configHome is an XDG_CONFIG_HOME holding the test config, so the end-to-end runs never read the real one.
var configHome string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "tabreorg-test")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if tabreorgBin, err = testlib.BuildCmd(dir, "tab-sync/cmd/tabreorg"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.RemoveAll(dir)
		os.Exit(1)
	}
	configHome = filepath.Join(dir, "config")
	js, _ := json.Marshal(testConfig)
	if err := os.MkdirAll(filepath.Join(configHome, "tabreorg"), 0o755); err == nil {
		err = os.WriteFile(filepath.Join(configHome, "tabreorg", "config.json"), js, 0o644)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
