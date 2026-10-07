// tabreorg reorganizes a tab collection into <Artist>/<Album>/<Song>.<ext>
// using the metadata tabscan reads. It is a dry run unless -apply is given;
// applied runs write a move log and an undo script next to the root.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"tab-sync/internal/tab"
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
	if err == nil {
		err = setConfig(c)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "tabreorg:", err)
		os.Exit(1)
	}
	skipDirs := cfg.Skip
	if skipGiven {
		skipDirs = splitList(*skip)
	}
	if err := run(flag.Arg(0), *apply, *rename, *summary, skipDirs); err != nil {
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

func run(rootArg string, apply, rename bool, summary string, skip []string) error {
	root, err := filepath.Abs(rootArg)
	if err != nil {
		return err
	}
	if summary == "" {
		summary = root + "-reorg-summary.md"
	}
	var songs []*tab.Song
	if err := tab.Walk(root, root, func(s *tab.Song) { songs = append(songs, s) }); err != nil {
		return err
	}
	p, err := newPlanner(root, skip)
	if err != nil {
		return err
	}
	places, clashes := p.plan(songs, rename)
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
	if apply {
		if err := applyMoves(root, ms); err != nil {
			return err
		}
	}
	if err := writeSummary(summary, root, p, places, ms, clashes, apply); err != nil {
		return err
	}
	verb := "planned"
	if apply {
		verb = "done"
	}
	fmt.Fprintf(os.Stderr, "\n%d moves/renames %s, %d name clashes; summary: %s\n", len(ms), verb, len(clashes), summary)
	return nil
}

// applyMoves performs the moves, refusing to overwrite anything, and writes
// a TSV log plus a shell script that reverts them.
func applyMoves(root string, ms []move) error {
	if len(ms) == 0 {
		return nil
	}
	stamp := time.Now().Format("20060102-150405")
	logFile, err := os.Create(fmt.Sprintf("%s-moves-%s.tsv", root, stamp))
	if err != nil {
		return err
	}
	defer logFile.Close()
	undoName := fmt.Sprintf("%s-undo-%s.sh", root, stamp)
	undo, err := os.OpenFile(undoName, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	defer undo.Close()
	fmt.Fprintf(undo, "#!/bin/sh\n# Reverts the moves logged in %s\nset -e\ncd %s\n", logFile.Name(), shellQuote(root))
	for _, m := range ms {
		src := filepath.Join(root, filepath.FromSlash(m.src))
		dst := filepath.Join(root, filepath.FromSlash(m.dst))
		if !strings.EqualFold(src, dst) {
			if _, err := os.Lstat(dst); err == nil {
				return fmt.Errorf("refusing to overwrite %s", dst)
			}
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := os.Rename(src, dst); err != nil {
			return err
		}
		fmt.Fprintf(logFile, "%s\t%s\n", m.src, m.dst)
		dir := filepath.Dir(filepath.FromSlash(m.src))
		fmt.Fprintf(undo, "mkdir -p %s && mv -n %s %s\n", shellQuote(dir), shellQuote(m.dst), shellQuote(m.src))
	}
	fmt.Fprintf(os.Stderr, "log: %s\nundo: %s\n", logFile.Name(), undoName)
	return nil
}

func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }
