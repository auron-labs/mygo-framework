package command

import "errors"

// InputSourceABC identifies Apple's ABC keyboard input source on macOS.
const InputSourceABC = "com.apple.keylayout.ABC"

// ErrInputSourceUnavailable means there is no active native input context,
// the source is not enabled, or the platform does not support source selection.
var ErrInputSourceUnavailable = errors.New("command: input source unavailable")

type inputSourceSession struct {
	attempted bool
	waited    bool
	restore   func() error
}

func (s *inputSourceSession) begin(source string, keep bool, selectSource func(string) (func() error, error)) error {
	if s.attempted || source == "" {
		return nil
	}
	s.attempted = true
	restore, err := selectSource(source)
	if err == nil && !keep {
		s.restore = restore
	}
	return err
}

func (s *inputSourceSession) end() error {
	restore := s.restore
	*s = inputSourceSession{}
	if restore != nil {
		return restore()
	}
	return nil
}

// ReleaseInputSource ends this view's input-source session. CommandDialog
// calls it when closed. Embedded hosts must call it when removing the view;
// all hosts should also call it before destroying the owning window. Call on
// the UI thread. It is safe to call repeatedly. A manual input-source change or
// a different active native context is never overwritten during restoration.
func (s *State) ReleaseInputSource() error {
	err := s.inputSource.end()
	if err != nil {
		s.InputSourceError = err
	}
	return err
}
