package main

import (
	"fmt"
	"os"
	"testing"

	"tabfinder/internal/testlib"
)

// tabscanBin is the built command, for the end-to-end tests.
var tabscanBin string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "tabscan-test")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if tabscanBin, err = testlib.BuildCmd(dir, "tabfinder/cmd/tabscan"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.RemoveAll(dir)
		os.Exit(1)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
