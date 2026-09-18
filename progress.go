package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// CollectImages gathers the images under path before any of them is captioned,
// so a run knows how many there are and can say how far along it is. The walk
// reports as it goes: on a network mount it can take a while on its own, and
// silence there looks like a hang.
func CollectImages(path string, report func(found int)) (paths []string, rootDir string, err error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return nil, "", err
	}

	// A single file uses its own directory as the root.
	if !fileInfo.IsDir() {
		if isImageFile(path) {
			return []string{path}, filepath.Dir(path), nil
		}
		return nil, filepath.Dir(path), nil
	}

	rootDir = path
	lastReport := time.Now()
	err = filepath.Walk(path, func(currentPath string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Continue walking despite errors
		}

		// Skip hidden directories (starting with .)
		if info.IsDir() {
			if strings.HasPrefix(filepath.Base(currentPath), ".") {
				return filepath.SkipDir
			}
			return nil
		}

		if isImageFile(currentPath) {
			paths = append(paths, currentPath)
			if report != nil && time.Since(lastReport) >= 2*time.Second {
				report(len(paths))
				lastReport = time.Now()
			}
		}
		return nil
	})
	return paths, rootDir, err
}

// progress reports where a run has got to. Its lines go to stderr so that
// redirecting stdout still gives you nothing but captions.
type progress struct {
	out     io.Writer
	total   int
	started time.Time

	// captioned counts the images that actually reached the model. Skipped
	// ones cost no time, so counting them would flatter the estimate.
	captioned int
	spent     time.Duration
}

func newProgress(total int) *progress {
	return &progress{out: os.Stderr, total: total, started: time.Now()}
}

// Starting announces the image about to be sent, with an estimate once there is
// enough history to base one on.
func (p *progress) Starting(index int, name string) {
	line := fmt.Sprintf("[%d/%d] %s", index, p.total, name)
	if eta, ok := p.remaining(index); ok {
		line += fmt.Sprintf("  eta %s", formatDuration(eta))
	}
	fmt.Fprintln(p.out, line)
}

// Done records how long an image took, which is what later estimates rest on.
func (p *progress) Done(took time.Duration) {
	p.captioned++
	p.spent += took
}

// remaining estimates the time left from the average so far. Images that were
// skipped are not in that average, so the estimate covers the ones still to be
// captioned rather than the whole list.
func (p *progress) remaining(index int) (time.Duration, bool) {
	if p.captioned == 0 || index > p.total {
		return 0, false
	}
	average := p.spent / time.Duration(p.captioned)
	return average * time.Duration(p.total-index+1), true
}

// Elapsed is how long the run has taken so far.
func (p *progress) Elapsed() time.Duration {
	return time.Since(p.started)
}

// formatDuration writes a duration the way a person reads one, rounded to
// something honest rather than to the nanosecond.
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm%02ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh%02dm", int(d.Hours()), int(d.Minutes())%60)
}
