// Package logging configures the application's structured logs.
package logging

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/natefinch/lumberjack.v2"
)

const logFilename = "app.log"

// LevelSuccess marks an operation that finished as intended. It ranks between
// slog.LevelInfo and slog.LevelWarn, so it is written whenever info is.
const LevelSuccess = slog.LevelInfo + 2

// Options controls the application's log destinations, level and file retention.
type Options struct {
	Development bool
	// Level is the lowest level written: debug, info, success, warn or error.
	Level      slog.Level
	Directory  string
	MaxSizeMB  int
	MaxBackups int
	MaxAgeDays int
	Compress   bool
}

// DefaultOptions returns the initial level and file-retention policy: debug
// and above in development, info and above otherwise.
func DefaultOptions(development bool, directory string) Options {
	level := slog.LevelInfo
	if development {
		level = slog.LevelDebug
	}
	return Options{
		Development: development,
		Level:       level,
		Directory:   directory,
		MaxSizeMB:   10,
		MaxBackups:  3,
		MaxAgeDays:  28,
		Compress:    true,
	}
}

// New writes development logs to stderr. Packaged logs also go to a rotating
// file in the application's log directory.
func New(options Options) (*slog.Logger, io.Closer, error) {
	return newLogger(options, os.Stderr)
}

func newLogger(options Options, stderr io.Writer) (*slog.Logger, io.Closer, error) {
	handlerOptions := &slog.HandlerOptions{Level: options.Level, ReplaceAttr: replaceAttr}
	if options.Development {
		return slog.New(slog.NewTextHandler(stderr, handlerOptions)), nopCloser{}, nil
	}
	if options.Directory == "" {
		return nil, nil, errors.New("configure rotating logs: directory is required")
	}
	if options.MaxSizeMB <= 0 || options.MaxBackups < 0 || options.MaxAgeDays < 0 {
		return nil, nil, errors.New("configure rotating logs: invalid retention values")
	}
	if err := os.MkdirAll(options.Directory, 0o700); err != nil {
		return nil, nil, fmt.Errorf("create log directory: %w", err)
	}

	file := &lumberjack.Logger{
		Filename:   filepath.Join(options.Directory, logFilename),
		MaxSize:    options.MaxSizeMB,
		MaxBackups: options.MaxBackups,
		MaxAge:     options.MaxAgeDays,
		Compress:   options.Compress,
	}
	handler := slog.NewTextHandler(io.MultiWriter(stderr, file), handlerOptions)

	return slog.New(handler), file, nil
}

// Success logs that an operation finished as intended.
func Success(ctx context.Context, logger *slog.Logger, message string, attrs ...slog.Attr) {
	logger.LogAttrs(ctx, LevelSuccess, message, attrs...)
}

// replaceAttr names the success level and redacts sensitive values.
func replaceAttr(groups []string, attr slog.Attr) slog.Attr {
	if len(groups) == 0 && attr.Key == slog.LevelKey {
		if level, ok := attr.Value.Any().(slog.Level); ok && level == LevelSuccess {
			return slog.String(slog.LevelKey, "SUCCESS")
		}
	}
	return redactSensitive(groups, attr)
}

func redactSensitive(groups []string, attr slog.Attr) slog.Attr {
	if sensitiveKey(attr.Key) {
		return slog.String(attr.Key, "[REDACTED]")
	}
	for _, group := range groups {
		if sensitiveKey(group) {
			return slog.String(attr.Key, "[REDACTED]")
		}
	}

	return attr
}

func sensitiveKey(key string) bool {
	key = strings.NewReplacer("_", "", "-", "", ".", "").Replace(strings.ToLower(key))
	switch key {
	case "authorization", "cookie", "password", "payload", "requestbody", "responsebody", "secret", "settings", "token":
		return true
	default:
		return false
	}
}

type nopCloser struct{}

func (nopCloser) Close() error { return nil }
