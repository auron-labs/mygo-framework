// Package logging configures the application's structured logs.
package logging

import (
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

// Options controls the application's log destinations and file retention.
type Options struct {
	Development bool
	Directory   string
	MaxSizeMB   int
	MaxBackups  int
	MaxAgeDays  int
	Compress    bool
}

// DefaultOptions returns the initial file-retention policy.
func DefaultOptions(development bool, directory string) Options {
	return Options{
		Development: development,
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
	handlerOptions := &slog.HandlerOptions{ReplaceAttr: redactSensitive}
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
