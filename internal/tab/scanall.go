package tab

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"runtime"
	"sync"
)

// ScanAll scans every tab file under root on all CPUs and returns the
// songs in walk order, with paths relative to root. Like Walk, it stops at
// the first unreadable directory and returns what it found so far.
func ScanAll(root string) ([]*Song, error) {
	var paths []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
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
