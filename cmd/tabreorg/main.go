// tabreorg reorganizes a tab collection into <Artist>/<Album>/<Song>.<ext>
// using the metadata tabscan reads. It is a dry run unless -apply is given;
// applied runs write a move log and an undo script next to the root.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"tabfinder/internal/tab"
)

func main() {
	apply := flag.Bool("apply", false, "move and rename the files (default: dry run)")
	rename := flag.Bool("rename", true, "name each tab after its song")
	summary := flag.String("summary", "", "summary markdown file (default: <root>-reorg-summary.md next to the root)")
	skip := flag.String("skip", "", "comma-separated folders, relative to the root, left untouched (default: \"skip\" of the config file)")
	configFile := flag.String("config", "", "config file with skip folders, album aliases and junk album names (default: "+configPath()+", optional)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: tabreorg [-apply] [-rename=false] [-summary file] [-skip dirs] [-config file] root\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}
	skipGiven := false
	flag.Visit(func(f *flag.Flag) { skipGiven = skipGiven || f.Name == "skip" })
	path, asked := configPath(), *configFile != ""
	if asked {
		path = *configFile
	}
	c, err := loadConfig(path, asked)
	var albums nameRules
	if err == nil {
		albums, err = newNameRules(c)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "tabreorg:", err)
		os.Exit(1)
	}
	skipDirs := c.Skip
	if skipGiven {
		skipDirs = splitList(*skip)
	}
	if err := run(flag.Arg(0), options{apply: *apply, rename: *rename, summary: *summary, skip: skipDirs, rules: albums}); err != nil {
		fmt.Fprintln(os.Stderr, "tabreorg:", err)
		os.Exit(1)
	}
}

func splitList(s string) []string {
	var out []string
	for _, f := range strings.Split(s, ",") {
		if f = strings.TrimSpace(f); f != "" {
			out = append(out, f)
		}
	}
	return out
}

// options is what a run is asked to do.
type options struct {
	apply   bool   // move the files, not just plan
	rename  bool   // name tabs after their songs
	summary string // summary file; "" for <root>-reorg-summary.md
	skip    []string
	rules   nameRules
}

func run(rootArg string, o options) error {
	root, err := filepath.Abs(rootArg)
	if err != nil {
		return err
	}
	summary := o.summary
	if summary == "" {
		summary = root + "-reorg-summary.md"
	}
	var songs []*tab.Song
	if err := tab.Walk(root, root, func(s *tab.Song) { songs = append(songs, s) }); err != nil {
		return err
	}
	p, err := newPlanner(root, o.skip, o.rules)
	if err != nil {
		return err
	}
	places, clashes := p.plan(songs, o.rename)
	ms := moves(places)
	for _, m := range ms {
		fmt.Printf("%s\t->\t%s\n", m.src, m.dst)
	}
	if len(clashes) > 0 {
		fmt.Printf("\n# Not renamed, same song name in one folder (%d groups):\n", len(clashes))
		for _, c := range clashes {
			fmt.Println()
			for _, f := range c {
				fmt.Println(f)
			}
		}
	}
	if o.apply {
		if err := applyMoves(root, ms); err != nil {
			return err
		}
	}
	if err := writeSummary(summary, outcome{root, p, places, ms, clashes, o.apply}); err != nil {
		return err
	}
	verb := "planned"
	if o.apply {
		verb = "done"
	}
	fmt.Fprintf(os.Stderr, "\n%d moves/renames %s, %d name clashes; summary: %s\n", len(ms), verb, len(clashes), summary)
	return nil
}

// applyMoves performs the moves, refusing to overwrite anything, and writes
// a TSV log plus a shell script that reverts them.
func applyMoves(root string, ms []move) (err error) {
	if len(ms) == 0 {
		return nil
	}
	stamp := time.Now().Format("20060102-150405")
	logFile, err := os.Create(fmt.Sprintf("%s-moves-%s.tsv", root, stamp))
	if err != nil {
		return err
	}
	defer closeChecked(logFile, &err)
	undoName := fmt.Sprintf("%s-undo-%s.sh", root, stamp)
	undo, err := os.OpenFile(undoName, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	defer closeChecked(undo, &err)
	if _, err := fmt.Fprintf(undo, "#!/bin/sh\n# Reverts the moves logged in %s\nset -e\ncd %s\n", logFile.Name(), shellQuote(root)); err != nil {
		return fmt.Errorf("undo script: %w", err)
	}
	if err := moveAll(root, ms, logFile, undo); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "log: %s\nundo: %s\n", logFile.Name(), undoName)
	return nil
}

// moveAll performs the moves, recording each one done in log and undo. It stops at the first
// move it can't make or record: the log and the undo script then cover every move made.
func moveAll(root string, ms []move, log, undo io.Writer) error {
	for _, m := range ms {
		src := filepath.Join(root, filepath.FromSlash(m.src))
		dst := filepath.Join(root, filepath.FromSlash(m.dst))
		if blocked, err := takenByAnother(src, dst); err != nil {
			return err
		} else if blocked {
			return fmt.Errorf("refusing to overwrite %s", dst)
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := os.Rename(src, dst); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(log, "%s\t%s\n", m.src, m.dst); err != nil {
			return fmt.Errorf("move log, after moving %s: %w", m.src, err)
		}
		dir := filepath.Dir(filepath.FromSlash(m.src))
		if _, err := fmt.Fprintf(undo, "mkdir -p %s && mv -n %s %s\n", shellQuote(dir), shellQuote(m.dst), shellQuote(m.src)); err != nil {
			return fmt.Errorf("undo script, after moving %s (move it back by hand): %w", m.src, err)
		}
	}
	return nil
}

// takenByAnother reports whether dst is a file other than src. A rename that only changes
// case may find src itself at dst, on a file system that ignores case.
func takenByAnother(src, dst string) (bool, error) {
	dstInfo, err := os.Lstat(dst)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	srcInfo, err := os.Lstat(src)
	if err != nil {
		return false, err
	}
	return !os.SameFile(srcInfo, dstInfo), nil
}

// closeChecked closes f, and reports a failure in *err unless there is an error already:
// closing is when a write may turn out to have failed.
func closeChecked(f *os.File, err *error) {
	if cerr := f.Close(); cerr != nil && *err == nil {
		*err = cerr
	}
}

func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }
