package datadir

import (
	"errors"
	"path"
	"testing"
)

func TestRedactorOnAPhoneReplacesTheDataDirectoryAndHome(t *testing.T) {
	home := "/var/mobile/Containers/Data/Application/ABC"
	data := path.Join(home, "Library", "Application Support", "Notes")
	redact := redactor("ios",
		func() (string, error) { return data, nil },
		func() (string, error) { return home, nil })

	text := "open " + path.Join(data, "notes.db") + " then " + path.Join(home, "other.txt") + " failed"
	want := "open <app data>/notes.db then ~/other.txt failed"
	if got := redact(text); got != want {
		t.Fatalf("redacted text = %q, want %q", got, want)
	}
}

func TestRedactorOnDesktopReplacesOnlyTheHomeDirectory(t *testing.T) {
	redact := redactor("linux",
		func() (string, error) { return "", errors.New("sandbox directory looked up on desktop") },
		func() (string, error) { return "/home/user", nil })

	if got, want := redact("open /home/user/data/notes.db failed"), "open ~/data/notes.db failed"; got != want {
		t.Fatalf("redacted text = %q, want %q", got, want)
	}
}

func TestRedactorLeavesTextAloneWithoutPaths(t *testing.T) {
	failing := func() (string, error) { return "", errors.New("unavailable") }
	redact := redactor("android", failing, failing)

	if text := "open /data/notes.db failed"; redact(text) != text {
		t.Fatalf("redacted text = %q, want it unchanged", redact(text))
	}
}
