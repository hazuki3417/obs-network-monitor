package errorlog

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	filename         = "error.log"
	previousFilename = "error.previous.log"
	defaultMaxBytes  = 5 * 1024 * 1024
)

type Logger struct {
	directory string
	maxBytes  int64
	now       func() time.Time
	mu        sync.Mutex
}

func New(directory string) *Logger {
	return newLogger(directory, defaultMaxBytes, time.Now)
}

func newLogger(directory string, maxBytes int64, now func() time.Time) *Logger {
	return &Logger{directory: directory, maxBytes: maxBytes, now: now}
}

func (logger *Logger) Printf(format string, values ...any) {
	message := fmt.Sprintf(format, values...)

	logger.mu.Lock()
	defer logger.mu.Unlock()
	if err := logger.write(message); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "write error log: %v; original error: %s\n", err, message)
	}
}

func (logger *Logger) write(message string) error {
	if err := os.MkdirAll(logger.directory, 0o755); err != nil {
		return err
	}
	path := filepath.Join(logger.directory, filename)
	if info, err := os.Stat(path); err == nil && info.Size() >= logger.maxBytes {
		previous := filepath.Join(logger.directory, previousFilename)
		if err := os.Remove(previous); err != nil && !os.IsNotExist(err) {
			return err
		}
		if err := os.Rename(path, previous); err != nil {
			return err
		}
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = fmt.Fprintf(
		file,
		"%s ERROR %s\n",
		logger.now().Format("2006/01/02 15:04:05"),
		message,
	)
	return err
}
