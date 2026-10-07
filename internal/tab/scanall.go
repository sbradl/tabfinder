package tab

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"runtime"
	"sync"
)

// ScanAll scans every tab file under root and returns the songs in walk
// order, with paths relative to root. It stops at the first unreadable
// directory and returns what it found so far along with the error.
func ScanAll(root string) ([]*Song, error) { return scanTree(root, root) }

// scanTree finds the tab files under dir, then scans them on all CPUs.
func scanTree(dir, root string) ([]*Song, error) {
	var paths []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		if !d.IsDir() && IsTabFile(path) {
			paths = append(paths, path)
		}
		return nil
	})
	songs := make([]*Song, len(paths))
	next := make(chan int)
	var wg sync.WaitGroup
	for range runtime.NumCPU() {
		wg.Go(func() {
			for i := range next {
				songs[i] = Scan(paths[i], root)
			}
		})
	}
	for i := range paths {
		next <- i
	}
	close(next)
	wg.Wait()
	return songs, err
}
