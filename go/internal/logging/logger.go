package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
)

type Level int

const (
	LevelDebug Level = iota - 4
	LevelInfo  Level = 0
	LevelWarn  Level = 4
	LevelError Level = 8
)

type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
	With(args ...any) Logger
	WithContext(ctx context.Context) Logger
}

type Config struct {
	Level      Level
	Format     string // "json", "text", "dev"
	Output     io.Writer
	AddSource  bool
	TimeFormat string
}

type logger struct {
	slog   *slog.Logger
	config *Config
}

// Regex patterns for sensitive data filtering
var sensitivePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(api[_-]?key|token|secret|password|auth)["\s]*[:=]["\s]*([^\s"&]+)`),
	regexp.MustCompile(`(?i)authorization:\s*bearer\s+([^\s]+)`),
	regexp.MustCompile(`(?i)x-mediabrowser-token:\s*([^\s"&]+)`),
}

// NewLogger creates a new structured logger with the given configuration
func NewLogger(config *Config) Logger {
	if config == nil {
		config = &Config{
			Level:      LevelInfo,
			Format:     "text",
			Output:     os.Stdout,
			AddSource:  false,
			TimeFormat: time.RFC3339,
		}
	}

	if config.Output == nil {
		config.Output = os.Stdout
	}

	var handler slog.Handler

	opts := &slog.HandlerOptions{
		Level:     slog.Level(config.Level),
		AddSource: config.AddSource,
	}

	switch config.Format {
	case "json":
		handler = slog.NewJSONHandler(config.Output, opts)
	case "dev":
		// Pretty development format
		handler = NewDevHandler(config.Output, opts)
	default:
		handler = slog.NewTextHandler(config.Output, opts)
	}

	sl := slog.New(handler)

	return &logger{
		slog:   sl,
		config: config,
	}
}

func (l *logger) Debug(msg string, args ...any) {
	l.slog.Debug(l.sanitize(msg), l.sanitizeArgs(args)...)
}

func (l *logger) Info(msg string, args ...any) {
	l.slog.Info(l.sanitize(msg), l.sanitizeArgs(args)...)
}

func (l *logger) Warn(msg string, args ...any) {
	l.slog.Warn(l.sanitize(msg), l.sanitizeArgs(args)...)
}

func (l *logger) Error(msg string, args ...any) {
	l.slog.Error(l.sanitize(msg), l.sanitizeArgs(args)...)
}

func (l *logger) With(args ...any) Logger {
	return &logger{
		slog:   l.slog.With(l.sanitizeArgs(args)...),
		config: l.config,
	}
}

func (l *logger) WithContext(ctx context.Context) Logger {
	return &logger{
		slog:   l.slog.With(extractContextFields(ctx)...),
		config: l.config,
	}
}

// sanitize removes sensitive information from log messages
func (l *logger) sanitize(msg string) string {
	for _, pattern := range sensitivePatterns {
		msg = pattern.ReplaceAllStringFunc(msg, func(match string) string {
			parts := strings.SplitN(match, ":", 2)
			if len(parts) == 2 {
				return parts[0] + ": [REDACTED]"
			}
			parts = strings.SplitN(match, "=", 2)
			if len(parts) == 2 {
				return parts[0] + "=[REDACTED]"
			}
			return "[REDACTED]"
		})
	}
	return msg
}

// sanitizeArgs removes sensitive information from structured log arguments
func (l *logger) sanitizeArgs(args []any) []any {
	sanitized := make([]any, len(args))
	for i, arg := range args {
		if str, ok := arg.(string); ok {
			sanitized[i] = l.sanitize(str)
		} else {
			sanitized[i] = arg
		}
	}
	return sanitized
}

// extractContextFields extracts common context fields for structured logging
func extractContextFields(ctx context.Context) []any {
	var fields []any

	// Extract request ID if available
	if requestID := ctx.Value("request_id"); requestID != nil {
		fields = append(fields, "request_id", requestID)
	}

	// Extract user ID if available
	if userID := ctx.Value("user_id"); userID != nil {
		fields = append(fields, "user_id", userID)
	}

	// Extract session ID if available
	if sessionID := ctx.Value("session_id"); sessionID != nil {
		fields = append(fields, "session_id", sessionID)
	}

	return fields
}

// DevHandler is a custom handler for development-friendly logging
type DevHandler struct {
	opts   *slog.HandlerOptions
	output io.Writer
}

func NewDevHandler(output io.Writer, opts *slog.HandlerOptions) *DevHandler {
	return &DevHandler{
		opts:   opts,
		output: output,
	}
}

func (h *DevHandler) Enabled(ctx context.Context, level slog.Level) bool {
	minLevel := slog.LevelInfo
	if h.opts.Level != nil {
		minLevel = h.opts.Level.Level()
	}
	return level >= minLevel
}

func (h *DevHandler) Handle(ctx context.Context, record slog.Record) error {
	// Format: [LEVEL] message key=value
	levelStr := strings.ToUpper(record.Level.String())

	var levelColor string
	switch record.Level {
	case slog.LevelDebug:
		levelColor = "\033[36m" // Cyan
	case slog.LevelInfo:
		levelColor = "\033[32m" // Green
	case slog.LevelWarn:
		levelColor = "\033[33m" // Yellow
	case slog.LevelError:
		levelColor = "\033[31m" // Red
	default:
		levelColor = "\033[0m" // Default
	}

	resetColor := "\033[0m"

	timeStr := record.Time.Format("15:04:05")

	line := fmt.Sprintf("%s[%s %s]%s %s",
		levelColor, timeStr, levelStr, resetColor, record.Message)

	record.Attrs(func(attr slog.Attr) bool {
		line += fmt.Sprintf(" %s=%v", attr.Key, attr.Value)
		return true
	})

	line += "\n"

	_, err := h.output.Write([]byte(line))
	return err
}

func (h *DevHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	// For simplicity, return the same handler
	// In a full implementation, you'd create a new handler with these attrs
	return h
}

func (h *DevHandler) WithGroup(name string) slog.Handler {
	// For simplicity, return the same handler
	// In a full implementation, you'd create a new handler with this group
	return h
}

// Global logger instance
var defaultLogger Logger

// SetDefault sets the default global logger
func SetDefault(l Logger) {
	defaultLogger = l
}

// Default returns the default global logger
func Default() Logger {
	if defaultLogger == nil {
		defaultLogger = NewLogger(nil)
	}
	return defaultLogger
}

// Convenience functions using the default logger
func Debug(msg string, args ...any) {
	Default().Debug(msg, args...)
}

func Info(msg string, args ...any) {
	Default().Info(msg, args...)
}

func Warn(msg string, args ...any) {
	Default().Warn(msg, args...)
}

func Error(msg string, args ...any) {
	Default().Error(msg, args...)
}

// FiberMiddleware returns Fiber middleware for request logging with context
func FiberMiddleware(logger Logger) fiber.Handler {
	return func(c fiber.Ctx) error {
		start := time.Now()

		// Generate request ID
		requestID := generateRequestID()
		c.Locals("request_id", requestID)

		// Create context with request ID
		// In Fiber v3, we use Locals to store request-scoped data
		c.Locals("request_id", requestID)

		// Continue to next handler
		err := c.Next()

		// Log request completion
		duration := time.Since(start)
		status := c.Response().StatusCode()

		logLevel := "Info"
		if status >= 500 {
			logLevel = "Error"
		} else if status >= 400 {
			logLevel = "Warn"
		}

		logArgs := []any{
			"method", c.Method(),
			"path", c.Path(),
			"status", status,
			"duration_ms", duration.Milliseconds(),
			"request_id", requestID,
			"ip", c.IP(),
			"user_agent", c.Get("User-Agent"),
		}

		msg := fmt.Sprintf("%s %s - %d", c.Method(), c.Path(), status)

		switch logLevel {
		case "Error":
			logger.Error(msg, logArgs...)
		case "Warn":
			logger.Warn(msg, logArgs...)
		default:
			logger.Info(msg, logArgs...)
		}

		return err
	}
}

func generateRequestID() string {
	return fmt.Sprintf("req_%d", time.Now().UnixNano())
}
