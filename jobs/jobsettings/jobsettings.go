// Package jobsettings persists scheduled-task last-run times in a settings.Store.
package jobsettings

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/auron-labs/mygo-framework/jobs"
	"github.com/auron-labs/mygo-framework/settings"
)

// DefaultPrefix is the key prefix used by New when prefix is empty.
const DefaultPrefix = "schedule."

type lastRunStore struct {
	store  settings.Store
	prefix string
}

// New returns a jobs.LastRunStore keeping each task's last run as an RFC 3339
// UTC string under prefix+name+".last_run".
func New(store settings.Store, prefix string) jobs.LastRunStore {
	if prefix == "" {
		prefix = DefaultPrefix
	}
	return &lastRunStore{store: store, prefix: prefix}
}

func (s *lastRunStore) key(name string) string { return s.prefix + name + ".last_run" }

func (s *lastRunStore) LastRun(ctx context.Context, name string) (time.Time, bool, error) {
	raw, found, err := s.store.Read(ctx, s.key(name))
	if err != nil {
		return time.Time{}, false, fmt.Errorf("read last run of %q: %w", name, err)
	}
	if !found {
		return time.Time{}, false, nil
	}
	var at time.Time
	if err := json.Unmarshal(raw, &at); err != nil {
		return time.Time{}, false, fmt.Errorf("decode last run of %q: %w", name, err)
	}
	return at, true, nil
}

func (s *lastRunStore) SetLastRun(ctx context.Context, name string, at time.Time) error {
	raw, err := json.Marshal(at.UTC())
	if err != nil {
		return fmt.Errorf("encode last run of %q: %w", name, err)
	}
	if err := s.store.Write(ctx, s.key(name), raw); err != nil {
		return fmt.Errorf("write last run of %q: %w", name, err)
	}
	return nil
}
