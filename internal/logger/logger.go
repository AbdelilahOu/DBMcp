package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/AbdelilahOu/DBMcp/internal/config"
)

type Logger struct {
	slogger *slog.Logger
	logFile *os.File
}

type Config struct {
	Level      slog.Level
	OutputFile string
	MaxSize    int64
	Console    bool
}

var globalLogger *Logger

func ParseLogLevel(level string) slog.Level {
	switch level {
	case "DEBUG", "debug":
		return slog.LevelDebug
	case "WARN", "warn", "WARNING", "warning":
		return slog.LevelWarn
	case "ERROR", "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func ConfigFromLoggingConfig(logCfg config.LoggingConfig) Config {
	return Config{
		Level:      ParseLogLevel(logCfg.Level),
		OutputFile: logCfg.OutputFile,
		MaxSize:    logCfg.MaxSizeMB,
		Console:    logCfg.Console,
	}
}

func Initialize(cfg Config) error {
	logger, err := NewLogger(cfg)
	if err != nil {
		return fmt.Errorf("failed to create logger: %w", err)
	}
	globalLogger = logger
	return nil
}

func NewLogger(cfg Config) (*Logger, error) {
	logger := &Logger{}

	var writers []io.Writer

	if cfg.Console {
		writers = append(writers, os.Stderr)
	}

	if cfg.OutputFile != "" {
		dir := filepath.Dir(cfg.OutputFile)
		if dir != "." && dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return nil, fmt.Errorf("failed to create log directory: %w", err)
			}
		}

		if err := rotateLogIfNeeded(cfg.OutputFile, cfg.MaxSize*1024*1024); err != nil {
			return nil, fmt.Errorf("failed to rotate log: %w", err)
		}

		file, err := os.OpenFile(cfg.OutputFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %w", err)
		}
		logger.logFile = file
		writers = append(writers, file)
	}

	var writer io.Writer
	switch len(writers) {
	case 0:
		writer = io.Discard
	case 1:
		writer = writers[0]
	default:
		writer = io.MultiWriter(writers...)
	}

	handler := slog.NewTextHandler(writer, &slog.HandlerOptions{Level: cfg.Level})
	logger.slogger = slog.New(handler)

	return logger, nil
}

func rotateLogIfNeeded(filename string, maxSize int64) error {
	info, err := os.Stat(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	if info.Size() >= maxSize {
		timestamp := time.Now().Format("20060102-150405")
		backupName := fmt.Sprintf("%s.%s", filename, timestamp)
		if err := os.Rename(filename, backupName); err != nil {
			return fmt.Errorf("failed to rotate log file: %w", err)
		}
	}

	return nil
}

func (l *Logger) Close() error {
	if l.logFile != nil {
		return l.logFile.Close()
	}
	return nil
}

func toAttrs(fields map[string]interface{}) []any {
	attrs := make([]any, 0, len(fields)*2)
	for k, v := range fields {
		attrs = append(attrs, k, v)
	}
	return attrs
}

func (l *Logger) Debug(msg string, fields ...map[string]interface{}) {
	if len(fields) > 0 {
		l.slogger.Debug(msg, toAttrs(fields[0])...)
	} else {
		l.slogger.Debug(msg)
	}
}

func (l *Logger) Info(msg string, fields ...map[string]interface{}) {
	if len(fields) > 0 {
		l.slogger.Info(msg, toAttrs(fields[0])...)
	} else {
		l.slogger.Info(msg)
	}
}

func (l *Logger) Warn(msg string, fields ...map[string]interface{}) {
	if len(fields) > 0 {
		l.slogger.Warn(msg, toAttrs(fields[0])...)
	} else {
		l.slogger.Warn(msg)
	}
}

func (l *Logger) Error(msg string, err error, fields ...map[string]interface{}) {
	attrs := make(map[string]interface{})
	if len(fields) > 0 {
		attrs = fields[0]
	}
	if err != nil {
		attrs["error"] = err.Error()
	}
	l.slogger.Error(msg, toAttrs(attrs)...)
}

func Debug(msg string, fields ...map[string]interface{}) {
	if globalLogger != nil {
		globalLogger.Debug(msg, fields...)
	}
}

func Info(msg string, fields ...map[string]interface{}) {
	if globalLogger != nil {
		globalLogger.Info(msg, fields...)
	}
}

func Warn(msg string, fields ...map[string]interface{}) {
	if globalLogger != nil {
		globalLogger.Warn(msg, fields...)
	}
}

func Error(msg string, err error, fields ...map[string]interface{}) {
	if globalLogger != nil {
		globalLogger.Error(msg, err, fields...)
	}
}

func LogToolCall(toolName string, err error) {
	if err != nil {
		Error("tool call failed", err, map[string]interface{}{"tool": toolName})
	} else {
		Info("tool call completed", map[string]interface{}{"tool": toolName})
	}
}

func LogDatabaseOperation(operation, query string, rowsAffected int64, err error) {
	if len(query) > 100 {
		query = query[:100] + "..."
	}
	fields := map[string]interface{}{
		"operation": operation,
		"query":     query,
	}
	if err != nil {
		Error("database operation failed", err, fields)
		return
	}
	if rowsAffected > 0 {
		fields["rows_affected"] = rowsAffected
	}
	Info("database operation completed", fields)
}

func LogConnectionEvent(event, connectionName, dbType string, err error) {
	fields := map[string]interface{}{
		"event":      event,
		"connection": connectionName,
		"db_type":    dbType,
	}
	if err != nil {
		Error("connection event failed", err, fields)
	} else {
		Info("connection event completed", fields)
	}
}

func Shutdown() error {
	if globalLogger != nil {
		return globalLogger.Close()
	}
	return nil
}
