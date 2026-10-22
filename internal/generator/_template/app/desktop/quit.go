package desktop

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/egoist/mygo"
)

// QuitStage keeps MyGo alive while bounded runtime cleanup runs.
type QuitStage struct {
	cancelApp context.CancelFunc
	shutdown  func(context.Context) error
	resume    func()
	report    func(error)
	timeout   time.Duration

	mu       sync.Mutex
	running  bool
	resolved bool
	once     sync.Once
}

// NewQuitStage constructs the application's cancelable pre-quit stage.
func NewQuitStage(
	cancelApp context.CancelFunc,
	shutdown func(context.Context) error,
	resume func(),
	report func(error),
	timeout time.Duration,
) *QuitStage {
	return &QuitStage{
		cancelApp: cancelApp,
		shutdown:  shutdown,
		resume:    resume,
		report:    report,
		timeout:   timeout,
	}
}

// BeforeQuit prevents the first quit while cleanup runs off the UI thread.
func (s *QuitStage) BeforeQuit(event *mygo.QuitEvent) {
	if event.DefaultPrevented() {
		return
	}

	s.mu.Lock()
	if s.resolved {
		s.resolved = false
		s.mu.Unlock()
		return
	}
	event.PreventDefault()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()

	s.once.Do(s.cancelApp)
	go s.finish()
}

func (s *QuitStage) finish() {
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()

	result := make(chan error, 1)
	go func() {
		result <- s.shutdown(ctx)
	}()

	var err error
	select {
	case err = <-result:
	case <-ctx.Done():
		err = ctx.Err()
	}
	if err != nil && s.report != nil && !errors.Is(err, context.Canceled) {
		s.report(err)
	}

	s.mu.Lock()
	s.running = false
	s.resolved = true
	s.mu.Unlock()
	s.resume()
}
