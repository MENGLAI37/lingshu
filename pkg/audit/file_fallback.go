package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/lingshu/lingshu/pkg/logger"
)

const (
	DefaultFallbackDir       = "audit_fallback"
	DefaultFallbackFilePrefix = "audit_events"
	DefaultMaxFileSize       = 10 * 1024 * 1024
	DefaultMaxFiles          = 10
)

type FileFallback struct {
	dir           string
	filePrefix    string
	maxFileSize   int64
	maxFiles      int

	currentFile   *os.File
	currentSize   int64
	fileIndex     int

	mu            sync.Mutex
	enabled       bool
}

func NewFileFallback(dir string) (*FileFallback, error) {
	if dir == "" {
		dir = DefaultFallbackDir
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("resolve fallback dir: %w", err)
	}

	if err := os.MkdirAll(absDir, 0755); err != nil {
		return nil, fmt.Errorf("create fallback dir: %w", err)
	}

	fb := &FileFallback{
		dir:         absDir,
		filePrefix:  DefaultFallbackFilePrefix,
		maxFileSize: DefaultMaxFileSize,
		maxFiles:    DefaultMaxFiles,
		enabled:     true,
	}

	if err := fb.openLatestFile(); err != nil {
		logger.Warn("Failed to open latest audit fallback file", "error", err)
	}

	logger.Info("Audit file fallback initialized",
		"dir", absDir,
		"max_file_size", fb.maxFileSize,
		"max_files", fb.maxFiles,
	)

	return fb, nil
}

func (fb *FileFallback) openLatestFile() error {
	fb.mu.Lock()
	defer fb.mu.Unlock()

	matches, err := filepath.Glob(filepath.Join(fb.dir, fb.filePrefix+"_*.jsonl"))
	if err != nil {
		return fmt.Errorf("glob fallback files: %w", err)
	}

	if len(matches) == 0 {
		return fb.createNewFile()
	}

	latestFile := matches[len(matches)-1]
	info, err := os.Stat(latestFile)
	if err != nil {
		return fb.createNewFile()
	}

	if info.Size() >= fb.maxFileSize {
		return fb.createNewFile()
	}

	f, err := os.OpenFile(latestFile, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open latest file: %w", err)
	}

	fb.currentFile = f
	fb.currentSize = info.Size()
	fb.fileIndex = len(matches) - 1

	return nil
}

func (fb *FileFallback) createNewFile() error {
	if fb.currentFile != nil {
		_ = fb.currentFile.Close()
	}

	filename := fmt.Sprintf("%s_%d_%04d.jsonl",
		fb.filePrefix,
		time.Now().Unix(),
		fb.fileIndex,
	)
	filepath := filepath.Join(fb.dir, filename)

	f, err := os.OpenFile(filepath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("create fallback file: %w", err)
	}

	fb.currentFile = f
	fb.currentSize = 0
	fb.fileIndex++

	fb.cleanupOldFiles()

	return nil
}

func (fb *FileFallback) cleanupOldFiles() {
	matches, err := filepath.Glob(filepath.Join(fb.dir, fb.filePrefix+"_*.jsonl"))
	if err != nil {
		return
	}

	if len(matches) <= fb.maxFiles {
		return
	}

	toDelete := len(matches) - fb.maxFiles
	for i := 0; i < toDelete && i < len(matches); i++ {
		_ = os.Remove(matches[i])
	}
}

func (fb *FileFallback) WriteBatch(events []AuditEvent) error {
	if !fb.enabled {
		return fmt.Errorf("file fallback is disabled")
	}
	if len(events) == 0 {
		return nil
	}

	fb.mu.Lock()
	defer fb.mu.Unlock()

	if fb.currentFile == nil {
		if err := fb.createNewFile(); err != nil {
			return fmt.Errorf("create fallback file: %w", err)
		}
	}

	for _, event := range events {
		data, err := json.Marshal(event)
		if err != nil {
			logger.Warn("Failed to marshal audit event for fallback", "error", err)
			continue
		}

		line := append(data, '\n')

		if fb.currentSize+int64(len(line)) > fb.maxFileSize {
			if err := fb.createNewFile(); err != nil {
				return fmt.Errorf("rotate fallback file: %w", err)
			}
		}

		n, err := fb.currentFile.Write(line)
		if err != nil {
			return fmt.Errorf("write fallback line: %w", err)
		}

		fb.currentSize += int64(n)
	}

	if err := fb.currentFile.Sync(); err != nil {
		logger.Warn("Failed to sync fallback file", "error", err)
	}

	return nil
}

func (fb *FileFallback) Write(event *AuditEvent) error {
	if event == nil {
		return fmt.Errorf("event is nil")
	}
	return fb.WriteBatch([]AuditEvent{*event})
}

func (fb *FileFallback) ReadAll() ([]AuditEvent, error) {
	fb.mu.Lock()
	defer fb.mu.Unlock()

	matches, err := filepath.Glob(filepath.Join(fb.dir, fb.filePrefix+"_*.jsonl"))
	if err != nil {
		return nil, fmt.Errorf("glob fallback files: %w", err)
	}

	var allEvents []AuditEvent

	for _, match := range matches {
		events, err := fb.readFile(match)
		if err != nil {
			logger.Warn("Failed to read fallback file", "file", match, "error", err)
			continue
		}
		allEvents = append(allEvents, events...)
	}

	return allEvents, nil
}

func (fb *FileFallback) readFile(path string) ([]AuditEvent, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	var events []AuditEvent
	lines := splitLines(data)

	for _, line := range lines {
		if len(line) == 0 {
			continue
		}

		var event AuditEvent
		if err := json.Unmarshal(line, &event); err != nil {
			logger.Warn("Failed to unmarshal fallback event", "error", err)
			continue
		}
		events = append(events, event)
	}

	return events, nil
}

func (fb *FileFallback) Clear() error {
	fb.mu.Lock()
	defer fb.mu.Unlock()

	if fb.currentFile != nil {
		_ = fb.currentFile.Close()
		fb.currentFile = nil
	}

	matches, err := filepath.Glob(filepath.Join(fb.dir, fb.filePrefix+"_*.jsonl"))
	if err != nil {
		return fmt.Errorf("glob fallback files: %w", err)
	}

	for _, match := range matches {
		_ = os.Remove(match)
	}

	fb.currentSize = 0
	fb.fileIndex = 0

	return nil
}

func (fb *FileFallback) Close() error {
	fb.mu.Lock()
	defer fb.mu.Unlock()

	if fb.currentFile != nil {
		if err := fb.currentFile.Sync(); err != nil {
			logger.Warn("Failed to sync fallback file on close", "error", err)
		}
		_ = fb.currentFile.Close()
		fb.currentFile = nil
	}

	fb.enabled = false

	return nil
}

func (fb *FileFallback) IsEnabled() bool {
	fb.mu.Lock()
	defer fb.mu.Unlock()
	return fb.enabled
}

func (fb *FileFallback) GetDir() string {
	return fb.dir
}

func splitLines(data []byte) [][]byte {
	var lines [][]byte
	start := 0
	for i, b := range data {
		if b == '\n' {
			lines = append(lines, data[start:i])
			start = i + 1
		}
	}
	if start < len(data) {
		lines = append(lines, data[start:])
	}
	return lines
}
