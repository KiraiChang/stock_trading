package logging

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	defaultLogDir        = "/app/logs/backend"
	defaultRetentionDays = 14
)

type Config struct {
	Service       string
	Dir           string
	RetentionDays int
	Now           func() time.Time
}

type DailyFileWriter struct {
	mu            sync.Mutex
	dir           string
	service       string
	retentionDays int
	now           func() time.Time
	currentDate   string
	file          *os.File
}

func New(service string) (*zap.Logger, func(), error) {
	cfg := Config{
		Service:       service,
		Dir:           envString("LOG_DIR", defaultLogDir),
		RetentionDays: envInt("LOG_RETENTION_DAYS", defaultRetentionDays),
		Now:           time.Now,
	}
	return NewWithConfig(cfg)
}

func NewWithConfig(cfg Config) (*zap.Logger, func(), error) {
	if cfg.Service == "" {
		cfg.Service = "app"
	}
	if cfg.Dir == "" {
		cfg.Dir = defaultLogDir
	}
	if cfg.RetentionDays <= 0 {
		cfg.RetentionDays = defaultRetentionDays
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}

	fileWriter, err := NewDailyFileWriter(cfg)
	if err != nil {
		return nil, nil, err
	}

	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	encoder := zapcore.NewJSONEncoder(encoderCfg)

	// app log 一律走 stdout（與 Python 服務一致、符合 12-factor；docker compose logs 也會收），
	// 另外鏡射到每日輪替的持久化檔案。
	core := zapcore.NewTee(
		zapcore.NewCore(encoder, zapcore.Lock(os.Stdout), zap.InfoLevel),
		zapcore.NewCore(encoder, zapcore.AddSync(fileWriter), zap.InfoLevel),
	)
	logger := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
	cleanup := func() {
		_ = logger.Sync()
		_ = fileWriter.Close()
	}
	return logger, cleanup, nil
}

func NewDailyFileWriter(cfg Config) (*DailyFileWriter, error) {
	if err := os.MkdirAll(cfg.Dir, 0o755); err != nil {
		return nil, err
	}
	w := &DailyFileWriter{
		dir:           cfg.Dir,
		service:       cfg.Service,
		retentionDays: cfg.RetentionDays,
		now:           cfg.Now,
	}
	if err := w.cleanupOldFiles(); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *DailyFileWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.ensureFile(); err != nil {
		return 0, err
	}
	return w.file.Write(p)
}

func (w *DailyFileWriter) Sync() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	return w.file.Sync()
}

func (w *DailyFileWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.closeFile()
}

func (w *DailyFileWriter) ensureFile() error {
	date := w.now().UTC().Format("2006-01-02")
	if w.file != nil && w.currentDate == date {
		return nil
	}
	if err := w.closeFile(); err != nil {
		return err
	}
	path := filepath.Join(w.dir, fmt.Sprintf("%s-%s.log", w.service, date))
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	w.file = file
	w.currentDate = date
	// 保留清理為 best-effort：清舊檔失敗（例如權限）不應讓寫 log 失敗、丟掉這行 log。
	_ = w.cleanupOldFiles()
	return nil
}

func (w *DailyFileWriter) closeFile() error {
	if w.file == nil {
		return nil
	}
	err := w.file.Close()
	w.file = nil
	w.currentDate = ""
	return err
}

func (w *DailyFileWriter) cleanupOldFiles() error {
	cutoff := w.now().UTC().AddDate(0, 0, -w.retentionDays)
	entries, err := os.ReadDir(w.dir)
	if err != nil {
		return err
	}
	prefix := w.service + "-"
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, ".log") {
			continue
		}
		datePart := strings.TrimSuffix(strings.TrimPrefix(name, prefix), ".log")
		date, err := time.Parse("2006-01-02", datePart)
		if err != nil {
			continue
		}
		if date.Before(cutoff) {
			if err := os.Remove(filepath.Join(w.dir, name)); err != nil {
				return err
			}
		}
	}
	return nil
}

func envString(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}
