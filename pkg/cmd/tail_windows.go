//go:build windows

package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// tailFiles tails multiple log files using a pure Go polling approach.
func tailFiles(logFiles []string, fileColors map[string]Color) error {
	var wg sync.WaitGroup

	for _, logFile := range logFiles {
		wg.Add(1)
		go func(path string) {
			defer wg.Done()
			tailOneFile(path, fileColors)
		}(logFile)
	}

	// Block forever (like tail -f) — user must Ctrl+C to exit
	wg.Wait()
	return nil
}

func tailOneFile(path string, fileColors map[string]Color) {
	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot open %s: %v\n", path, err)
		return
	}
	defer f.Close()

	// Seek to end
	f.Seek(0, io.SeekEnd)

	svc := strings.TrimSuffix(filepath.Base(path), ".log")
	color, ok := fileColors[path]
	if !ok {
		color = ColorCyan
	}

	buf := make([]byte, 4096)
	for {
		n, err := f.Read(buf)
		if n > 0 {
			// Print service header and content
			fmt.Print(SprintColor(color, "[%s] ", svc))
			fmt.Print(string(buf[:n]))
		}
		if err == io.EOF {
			time.Sleep(200 * time.Millisecond)
			continue
		}
		if err != nil {
			return
		}
	}
}
