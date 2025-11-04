package logger

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"
)

var (
	globalLogger *zap.Logger
	once         sync.Once
	mu           sync.RWMutex
)

// Config represents logger configuration
type Config struct {
	Level   string // debug, info, warn, error
	File    string // log file path (empty for no file output)
	Console bool   // output to console
}

// Init initializes the global logger with the given configuration
// This should be called once at application startup
func Init(cfg Config) error {
	var err error
	once.Do(func() {
		globalLogger, err = buildLogger(cfg)
	})
	return err
}

// Get returns the global logger
// If the logger hasn't been initialized, it returns a no-op logger
func Get() *zap.Logger {
	mu.RLock()
	defer mu.RUnlock()

	if globalLogger == nil {
		// Return a no-op logger if not initialized
		return zap.NewNop()
	}
	return globalLogger
}

// With creates a child logger with additional fields
// This is useful for adding context-specific fields like component name
func With(fields ...zap.Field) *zap.Logger {
	return Get().With(fields...)
}

// Named creates a named logger (adds a "logger" field with the given name)
// This is useful for differentiating logs from different components
func Named(name string) *zap.Logger {
	return Get().Named(name)
}

// Component creates a logger for a specific component
// This adds a "component" field to all logs from this logger
func Component(component string) *zap.Logger {
	return With(zap.String("component", component))
}

// Sync flushes any buffered log entries
// Applications should call this before exiting
func Sync() error {
	mu.RLock()
	defer mu.RUnlock()

	if globalLogger != nil {
		return globalLogger.Sync()
	}
	return nil
}

// customEncoder wraps zapcore.Encoder to merge fields into message
type customEncoder struct {
	zapcore.Encoder
}

// Clone creates a copy of the encoder
func (e *customEncoder) Clone() zapcore.Encoder {
	return &customEncoder{
		Encoder: e.Encoder.Clone(),
	}
}

// EncodeEntry encodes the entry with fields merged into the message
func (e *customEncoder) EncodeEntry(entry zapcore.Entry, fields []zapcore.Field) (*buffer.Buffer, error) {
	// Build field strings
	var fieldParts []string
	for _, field := range fields {
		if field.Type == zapcore.StringType {
			fieldParts = append(fieldParts, fmt.Sprintf("%s=%s", field.Key, field.String))
		} else if field.Type == zapcore.Int32Type || field.Type == zapcore.Int64Type {
			fieldParts = append(fieldParts, fmt.Sprintf("%s=%d", field.Key, field.Integer))
		} else if field.Type == zapcore.BoolType {
			fieldParts = append(fieldParts, fmt.Sprintf("%s=%t", field.Key, field.Integer == 1))
		} else if field.Type == zapcore.ErrorType {
			if field.Interface != nil {
				fieldParts = append(fieldParts, fmt.Sprintf("%s=%v", field.Key, field.Interface))
			}
		} else {
			// For other types, use String() representation
			fieldParts = append(fieldParts, fmt.Sprintf("%s=%v", field.Key, field.Interface))
		}
	}

	// Merge fields into message
	if len(fieldParts) > 0 {
		entry.Message = fmt.Sprintf("%s [%s]", entry.Message, strings.Join(fieldParts, ", "))
	}

	// Encode with no fields (they're now in the message)
	return e.Encoder.EncodeEntry(entry, nil)
}

// buildLogger creates a zap logger with the given configuration
func buildLogger(cfg Config) (*zap.Logger, error) {
	// Parse log level
	level := zapcore.InfoLevel
	if cfg.Level != "" {
		if err := level.UnmarshalText([]byte(cfg.Level)); err != nil {
			return nil, fmt.Errorf("invalid log level %q: %w", cfg.Level, err)
		}
	}

	// Create encoder config for compact single-line format: time|level|caller|msg
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "T",
		LevelKey:       "L",
		NameKey:        zapcore.OmitKey,
		CallerKey:      "C",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "M",
		StacktraceKey:  zapcore.OmitKey,
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05"),
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
		ConsoleSeparator: "|",
	}

	// Build output paths
	outputPaths := []string{}
	if cfg.Console {
		outputPaths = append(outputPaths, "stdout")
	}
	if cfg.File != "" {
		// Ensure log directory exists
		if err := ensureLogDir(cfg.File); err != nil {
			return nil, fmt.Errorf("failed to create log directory: %w", err)
		}
		outputPaths = append(outputPaths, cfg.File)
	}

	// Default to stdout if no output specified
	if len(outputPaths) == 0 {
		outputPaths = []string{"stdout"}
	}

	// Use custom encoder to merge fields into message
	baseEncoder := zapcore.NewConsoleEncoder(encoderConfig)
	encoder := &customEncoder{Encoder: baseEncoder}

	// Build cores for each output
	var cores []zapcore.Core
	for _, path := range outputPaths {
		var writeSyncer zapcore.WriteSyncer
		if path == "stdout" {
			writeSyncer = zapcore.AddSync(os.Stdout)
		} else if path == "stderr" {
			writeSyncer = zapcore.AddSync(os.Stderr)
		} else {
			file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				return nil, fmt.Errorf("failed to open log file %q: %w", path, err)
			}
			writeSyncer = zapcore.AddSync(file)
		}
		cores = append(cores, zapcore.NewCore(encoder, writeSyncer, level))
	}

	// Combine cores
	core := zapcore.NewTee(cores...)

	// Build logger with caller info
	logger := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))

	return logger, nil
}

// ensureLogDir ensures the directory for the log file exists
func ensureLogDir(logFile string) error {
	dir := logFile
	for i := len(dir) - 1; i >= 0; i-- {
		if dir[i] == '/' || dir[i] == '\\' {
			dir = dir[:i]
			break
		}
	}

	if dir == "" || dir == logFile {
		// No directory component
		return nil
	}

	return os.MkdirAll(dir, 0755)
}

// Helper functions for quick logging without getting logger instance

// Debug logs a debug message
func Debug(msg string, fields ...zap.Field) {
	Get().Debug(msg, fields...)
}

// Info logs an info message
func Info(msg string, fields ...zap.Field) {
	Get().Info(msg, fields...)
}

// Warn logs a warning message
func Warn(msg string, fields ...zap.Field) {
	Get().Warn(msg, fields...)
}

// Error logs an error message
func Error(msg string, fields ...zap.Field) {
	Get().Error(msg, fields...)
}

// Fatal logs a fatal message and exits
func Fatal(msg string, fields ...zap.Field) {
	Get().Fatal(msg, fields...)
}
