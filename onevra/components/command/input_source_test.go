package command

import (
	"errors"
	"testing"
)

func TestInputSourceSession(t *testing.T) {
	for _, keep := range []bool{false, true} {
		t.Run(map[bool]string{false: "restore", true: "keep"}[keep], func(t *testing.T) {
			var s inputSourceSession
			selected, restored := 0, 0
			selectSource := func(source string) (func() error, error) {
				if source != InputSourceABC {
					t.Fatalf("source=%q", source)
				}
				selected++
				return func() error { restored++; return nil }, nil
			}
			if err := s.begin("", keep, selectSource); err != nil || s.attempted {
				t.Fatal("empty source began a session")
			}
			for range 3 {
				if err := s.begin(InputSourceABC, keep, selectSource); err != nil {
					t.Fatal(err)
				}
			}
			if selected != 1 {
				t.Fatalf("selected %d times", selected)
			}
			for range 2 {
				if err := s.end(); err != nil {
					t.Fatal(err)
				}
			}
			want := 1
			if keep {
				want = 0
			}
			if restored != want {
				t.Fatalf("restored %d times; want %d", restored, want)
			}
			if err := s.begin(InputSourceABC, keep, selectSource); err != nil {
				t.Fatal(err)
			}
			if selected != 2 {
				t.Fatal("reopening did not begin a new session")
			}
		})
	}
}

func TestInputSourceFailureDoesNotRetryUntilReopened(t *testing.T) {
	var s inputSourceSession
	attempts := 0
	selectSource := func(string) (func() error, error) { attempts++; return nil, ErrInputSourceUnavailable }
	if err := s.begin(InputSourceABC, false, selectSource); !errors.Is(err, ErrInputSourceUnavailable) {
		t.Fatal(err)
	}
	_ = s.begin(InputSourceABC, false, selectSource)
	if attempts != 1 {
		t.Fatal("retried failed selection")
	}
	if err := s.end(); err != nil {
		t.Fatal(err)
	}
	_ = s.begin(InputSourceABC, false, selectSource)
	if attempts != 2 {
		t.Fatal("did not retry after reopening")
	}
}

func TestReleaseInputSourceReportsRestoreFailure(t *testing.T) {
	failure := errors.New("restore failed")
	var s State
	s.inputSource.restore = func() error { return failure }
	if err := s.ReleaseInputSource(); !errors.Is(err, failure) || !errors.Is(s.InputSourceError, failure) {
		t.Fatal("lost restoration error")
	}
	if err := s.ReleaseInputSource(); err != nil {
		t.Fatal("repeated restoration", err)
	}
}
