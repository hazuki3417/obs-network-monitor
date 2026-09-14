package errorlog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoggerCreatesFilesOnlyWhenAnErrorIsWritten(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "logs")
	logger := newLogger(directory, defaultMaxBytes, func() time.Time {
		return time.Date(2026, 9, 14, 16, 0, 0, 0, time.UTC)
	})

	if _, err := os.Stat(directory); !os.IsNotExist(err) {
		t.Fatalf("log directory exists before first error: %v", err)
	}
	logger.Printf("measure network: %s", "timeout")

	content, err := os.ReadFile(filepath.Join(directory, filename))
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	if !strings.Contains(text, "2026/09/14 16:00:00 ERROR") ||
		!strings.Contains(text, "measure network: timeout") {
		t.Fatalf("log content = %q", text)
	}
}

func TestLoggerRotatesAtSizeLimit(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "logs")
	logger := newLogger(directory, 10, time.Now)
	logger.Printf("first error")
	logger.Printf("second error")

	if _, err := os.Stat(filepath.Join(directory, previousFilename)); err != nil {
		t.Fatalf("previous log was not created: %v", err)
	}
	content, err := os.ReadFile(filepath.Join(directory, filename))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "second error") {
		t.Fatalf("current log content = %q", content)
	}
}
