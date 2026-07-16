package logging

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"
)

func TestLoggerWritesISO8601DailyFile(t *testing.T) {
	dir := t.TempDir()
	logger, cleanup, err := NewWithConfig(Config{
		Service:       "backend",
		Dir:           dir,
		RetentionDays: 14,
		Now: func() time.Time {
			return time.Date(2026, 7, 16, 12, 34, 56, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatalf("NewWithConfig() error = %v", err)
	}

	logger.Info("hello")
	cleanup()

	content, err := os.ReadFile(filepath.Join(dir, "backend-2026-07-16.log"))
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	if !regexp.MustCompile(`"ts":"\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z`).Match(content) {
		t.Fatalf("log timestamp is not ISO 8601: %s", content)
	}
	if !regexp.MustCompile(`"msg":"hello"`).Match(content) {
		t.Fatalf("log message missing: %s", content)
	}
}

func TestDailyFileWriterRotatesByUTCDate(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 7, 16, 23, 59, 0, 0, time.UTC)
	writer, err := NewDailyFileWriter(Config{
		Service:       "backend",
		Dir:           dir,
		RetentionDays: 14,
		Now: func() time.Time {
			return now
		},
	})
	if err != nil {
		t.Fatalf("NewDailyFileWriter() error = %v", err)
	}
	defer writer.Close()

	if _, err := writer.Write([]byte("day 1\n")); err != nil {
		t.Fatalf("Write(day 1) error = %v", err)
	}
	now = now.Add(2 * time.Minute)
	if _, err := writer.Write([]byte("day 2\n")); err != nil {
		t.Fatalf("Write(day 2) error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "backend-2026-07-16.log")); err != nil {
		t.Fatalf("day 1 file missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "backend-2026-07-17.log")); err != nil {
		t.Fatalf("day 2 file missing: %v", err)
	}
}

func TestDailyFileWriterRemovesExpiredLogs(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "backend-2026-07-01.log"), []byte("old"), 0o644); err != nil {
		t.Fatalf("write old log: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "backend-2026-07-14.log"), []byte("new"), 0o644); err != nil {
		t.Fatalf("write new log: %v", err)
	}

	writer, err := NewDailyFileWriter(Config{
		Service:       "backend",
		Dir:           dir,
		RetentionDays: 7,
		Now: func() time.Time {
			return time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatalf("NewDailyFileWriter() error = %v", err)
	}
	defer writer.Close()

	if _, err := os.Stat(filepath.Join(dir, "backend-2026-07-01.log")); !os.IsNotExist(err) {
		t.Fatalf("expired log should be removed, stat error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "backend-2026-07-14.log")); err != nil {
		t.Fatalf("fresh log should remain: %v", err)
	}
}
