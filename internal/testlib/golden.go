package testlib

import (
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "rewrite golden files")

// Golden compares got with testdata/<name> of the calling package; with
// -update it rewrites the file instead.
func Golden(t testing.TB, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run with -update to create it)", err)
	}
	if string(want) != string(got) {
		t.Errorf("%s differs from the golden file (run with -update to accept):\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
	}
}

// BuildCmd builds the command package pkg (e.g. "./cmd/tabscan") into dir and
// returns the binary's path. Call it from TestMain and share the result.
func BuildCmd(dir, pkg string) (string, error) {
	bin := filepath.Join(dir, filepath.Base(pkg))
	out, err := exec.Command("go", "build", "-o", bin, pkg).CombinedOutput()
	if err != nil {
		return "", &buildError{pkg, string(out), err}
	}
	return bin, nil
}

type buildError struct {
	pkg, out string
	err      error
}

func (e *buildError) Error() string { return "go build " + e.pkg + ": " + e.err.Error() + "\n" + e.out }

// Updating reports whether the tests run with -update: tests that generate
// their own testdata rewrite it instead of comparing.
func Updating() bool { return *update }
