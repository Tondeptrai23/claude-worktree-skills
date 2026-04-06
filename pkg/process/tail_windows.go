//go:build windows

package process

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// tailFilesImpl tails multiple log files using polling (Windows).
func tailFilesImpl(logFiles []string, fileColors map[string]interface{}, logger LineLogger) error {
	var wg sync.WaitGroup

	for _, logFile := range logFiles {
		wg.Add(1)
		go func(path string) {
			defer wg.Done()
			tailOneFile(path, fileColors, logger)
		}(logFile)
	}

	// Block forever (like tail -f) — user must Ctrl+C to exit
	wg.Wait()
	return nil
}

func tailOneFile(path string, fileColors map[string]interface{}, logger LineLogger) {
	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot open %s: %v\n", path, err)
		return
	}
	defer f.Close()

	// Seek to end
	f.Seek(0, io.SeekEnd)

	svc := strings.TrimSuffix(filepath.Base(path), ".log")
	color := fileColors[path]

	reader := bufio.NewReader(f)
	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 && err == nil {
			logger(color, svc, line)
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
