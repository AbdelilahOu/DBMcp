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

type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
)

var levelNames = map[LogLevel]string{
	DEBUG: "DEBUG",
	INFO:  "INFO",
	WARN:  "WARN",
	ERROR: "ERROR",
}

type Logger struct {
	slogger  *slog.Logger
	logLevel LogLevel
	logFile  *os.File
}

func ParseLogLevel(level string) LogLevel {
	switch level {
	case "DEBUG", "debug":
		return DEBUG
	case "INFO", "info":
		return INFO
	case "WARN", "warn", "WARNING", "warning":
		return WARN
	case "ERROR", "error":
		return ERROR
	default:
		return INFO
	}
}

func LogLevelString(level LogLevel) string {
	if name, exists := levelNames[level]; exists {
		return name
	}
	return "INFO"
}

func ConfigFromLoggingConfig(logCfg config.LoggingConfig) Config {
	return Config{
		Level:      ParseLogLevel(logCfg.Level),
		OutputFile: logCfg.OutputFile,
		MaxSize:    logCfg.MaxSizeMB,
		Console:    logCfg.Console,
	}
}

type Config struct {
	Level      LogLevel
	OutputFile string
	MaxSize    int64
	Console    bool
}

var globalLogger *Logger

func Initialize(cfg Config) error {
	logger, err := NewLogger(cfg)
	if err != nil {
		return fmt.Errorf("failed to create logger: %w", err)
	}
	globalLogger = logger
	return nil
}

func NewLogger(cfg Config) (*Logger, error) {
	logger := &Logger{
		logLevel: cfg.Level,
	}

	var writers []io.Writer

	if cfg.Console {
		writers = append(writers, os.Stdout)
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
	if len(writers) == 1 {
		writer = writers[0]
	} else {
		writer = io.MultiWriter(writers...)
	}

	opts := &slog.HandlerOptions{
		Level: slog.Level(-4),
	}
	handler := slog.NewTextHandler(writer, opts)
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

func (l *Logger) shouldLog(level LogLevel) bool {
	return level >= l.logLevel
}

func (l *Logger) log(level LogLevel, msg string, fields map[string]interface{}) {
	if !l.shouldLog(level) {
		return
	}

	timestamp := time.Now().UTC().Format("2006-01-02 15:04:05")
	logLine := fmt.Sprintf("[%s] %s: %s", timestamp, levelNames[level], msg)

	for k, v := range fields {
		logLine += fmt.Sprintf(" %s=%v", k, v)
	}

	l.slogger.Info(logLine)
}

func (l *Logger) Debug(msg string, fields ...map[string]interface{}) {
	fieldMap := make(map[string]interface{})
	if len(fields) > 0 {
		fieldMap = fields[0]
	}
	l.log(DEBUG, msg, fieldMap)
}

func (l *Logger) Info(msg string, fields ...map[string]interface{}) {
	fieldMap := make(map[string]interface{})
	if len(fields) > 0 {
		fieldMap = fields[0]
	}
	l.log(INFO, msg, fieldMap)
}

func (l *Logger) Warn(msg string, fields ...map[string]interface{}) {
	fieldMap := make(map[string]interface{})
	if len(fields) > 0 {
		fieldMap = fields[0]
	}
	l.log(WARN, msg, fieldMap)
}

func (l *Logger) Error(msg string, err error, fields ...map[string]interface{}) {
	fieldMap := make(map[string]interface{})
	if len(fields) > 0 {
		fieldMap = fields[0]
	}
	if err != nil {
		fieldMap["error"] = err.Error()
	}
	l.log(ERROR, msg, fieldMap)
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

func LogToolCall(toolName string, params interface{}, result interface{}, err error) {
	if err != nil {
		Error(fmt.Sprintf("Tool call failed: %s", toolName), err)
	} else {
		Info(fmt.Sprintf("Tool call completed: %s", toolName))
	}
}

func LogDatabaseOperation(operation, query string, rowsAffected int64, err error) {
	sanitizedQuery := query
	if len(sanitizedQuery) > 100 {
		sanitizedQuery = sanitizedQuery[:100] + "..."
	}

	if err != nil {
		Error(fmt.Sprintf("%s operation failed: %s", operation, sanitizedQuery), err)
	} else {
		if rowsAffected > 0 {
			Info(fmt.Sprintf("%s operation completed: %s (%d rows affected)", operation, sanitizedQuery, rowsAffected))
		} else {
			Info(fmt.Sprintf("%s operation completed: %s", operation, sanitizedQuery))
		}
	}
}

func LogConnectionEvent(event, connectionName, dbType string, err error) {
	if err != nil {
		Error(fmt.Sprintf("Connection event failed: %s to %s (%s)", event, connectionName, dbType), err)
	} else {
		Info(fmt.Sprintf("Connection event completed: %s to %s (%s)", event, connectionName, dbType))
	}
}

func GetGlobalLogger() *Logger {
	return globalLogger
}

func Shutdown() error {
	if globalLogger != nil {
		return globalLogger.Close()
	}
	return nil
}
