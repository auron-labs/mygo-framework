package jobsettings_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/auron-labs/mygo-framework/jobs/jobsettings"
	"github.com/auron-labs/mygo-framework/settings/bbolt"
)

func TestRoundTrip(t *testing.T) {
	store, err := bbolt.Open(filepath.Join(t.TempDir(), "settings.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	runs := jobsettings.New(store, "")
	ctx := context.Background()

	if _, ok, err := runs.LastRun(ctx, "feed"); ok || err != nil {
		t.Fatalf("empty store = %v, %v", ok, err)
	}
	want := time.Date(2026, 3, 4, 5, 6, 7, 0, time.FixedZone("x", 3600))
	if err := runs.SetLastRun(ctx, "feed", want); err != nil {
		t.Fatal(err)
	}
	got, ok, err := runs.LastRun(ctx, "feed")
	if err != nil || !ok || !got.Equal(want) {
		t.Fatalf("got %v, %v, %v", got, ok, err)
	}
	raw, found, _ := store.Read(ctx, "schedule.feed.last_run")
	if !found {
		t.Fatal("expected key schedule.feed.last_run")
	}
	var text string
	if err := json.Unmarshal(raw, &text); err != nil || text != "2026-03-04T04:06:07Z" {
		t.Fatalf("stored %s, %v", raw, err)
	}
}

func TestCorruptValueIsAnError(t *testing.T) {
	store, err := bbolt.Open(filepath.Join(t.TempDir(), "settings.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	ctx := context.Background()
	if err := store.Write(ctx, "p.feed.last_run", json.RawMessage(`"yesterday"`)); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := jobsettings.New(store, "p.").LastRun(ctx, "feed"); err == nil || ok {
		t.Fatalf("ok=%v err=%v, want error", ok, err)
	}
}
