package logging

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm/logger"
)

func TestGORMHandlerLogsDiagnosticsWithoutSQLOrParameters(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	applicationLogger, closer, err := newLogger(DefaultOptions(true, ""), &output)
	if err != nil {
		t.Fatalf("create application logger: %v", err)
	}
	t.Cleanup(func() {
		if err := closer.Close(); err != nil {
			t.Errorf("close application logger: %v", err)
		}
	})
	handler := NewGORM(applicationLogger)
	details := func() (string, int64) {
		return `INSERT INTO notes (body) VALUES ("private note body")`, 1
	}

	handler.Trace(t.Context(), time.Now().Add(-time.Second), details, nil)
	handler.Trace(t.Context(), time.Now().Add(-time.Second), details, errors.New("write failed"))
	got := output.String()
	for _, want := range []string{
		"msg=\"slow database query\"",
		"operation=gorm.query",
		"component=gorm",
		"rows=1",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("GORM log %q does not contain %q", got, want)
		}
	}
	for _, forbidden := range []string{"INSERT INTO", "private note body", "write failed"} {
		if strings.Contains(got, forbidden) {
			t.Errorf("GORM log contains %q: %q", forbidden, got)
		}
	}
}

func TestGORMHandlerHonorsSilentMode(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	applicationLogger, closer, err := newLogger(DefaultOptions(true, ""), &output)
	if err != nil {
		t.Fatalf("create application logger: %v", err)
	}
	t.Cleanup(func() {
		if err := closer.Close(); err != nil {
			t.Errorf("close application logger: %v", err)
		}
	})
	handler := NewGORM(applicationLogger).LogMode(logger.Silent)
	handler.Trace(t.Context(), time.Now().Add(-time.Second), func() (string, int64) {
		return "SELECT 1", 1
	}, nil)
	if got := output.String(); got != "" {
		t.Fatalf("silent GORM log = %q, want empty", got)
	}
}
