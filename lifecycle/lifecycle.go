// Package lifecycle coordinates the desktop lifetime of a MyGo application:
// its tray icon, close-to-tray behavior, login-item menu item, notification
// routes, and synchronous shutdown hook.
package lifecycle

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo"
)

// DefaultShutdownTimeout bounds the shutdown work started from OnQuit when
// Options does not set a tighter bound.
const DefaultShutdownTimeout = 10 * time.Second

// Application is the part of mygo.App the helper uses. It is an interface so
// tests can drive the lifecycle without a native event loop.
type Application interface {
	WhenReady(func())
	OnBeforeQuit(func(*mygo.QuitEvent)) (off func())
	OnQuitCanceled(func()) (off func())
	OnQuit(func()) (off func())
	OnNotificationClick(func(string)) (off func())
	OpenAtLogin() bool
	SetOpenAtLogin(bool) error
}

// Window is the part of a mygo.Window the helper uses.
type Window interface {
	Hide()
	OnClose(func(*mygo.CloseEvent)) (off func())
}

// TrayFactory creates a tray. The returned value is retained by the helper;
// the helper does not otherwise need to interact with it.
type TrayFactory func(mygo.TrayOptions) (any, error)

// NotificationRoutes maps deterministic notification ids to the page ids passed
// to Options.Open.
type NotificationRoutes map[string]string

// Options configures a Helper. CloseToTray, Open and Shutdown are required.
// NewTray and RunOnMain are MyGo seams and default to mygo.NewTray and
// mygo.RunOnMain; tests replace them.
type Options struct {
	// TrayID names the tray icon across launches. TrayIcon is a PNG, ideally
	// 16x16 points, and TrayIconIsTemplate lets the platform tint it.
	TrayID             string
	TrayIcon           []byte
	TrayIconIsTemplate bool
	TrayTooltip        string

	// OpenLabel is the tray menu item that opens the application. OpenAtLoginLabel
	// is the checkbox backed by the operating system's login-item state.
	OpenLabel        string
	OpenAtLoginLabel string

	// CloseToTray decides whether a user close hides the window instead of
	// closing it. Open is called on the main thread with a route's page id; the
	// tray's Open item uses the empty page. Shutdown owns bounded, synchronous
	// cleanup and is invoked once from OnQuit.
	CloseToTray func() bool
	Open        func(page string)
	Shutdown    func(context.Context) error

	// ShutdownTimeout bounds Shutdown when the helper invokes it from OnQuit.
	// Zero uses DefaultShutdownTimeout.
	ShutdownTimeout time.Duration
	// ReportError reports tray, login-item and shutdown failures. The operation
	// is a dotted lifecycle operation name.
	ReportError func(operation string, err error)
	// NotificationRoutes maps deterministic notification ids to page ids. Routes
	// are copied when the helper is constructed, so clicks can still be routed
	// after a process restart.
	NotificationRoutes NotificationRoutes

	// NewTray and RunOnMain are seams for tests. They are not application
	// settings; leave them nil in production.
	NewTray   TrayFactory
	RunOnMain func(func())
}

// Helper owns lifecycle orchestration. The application keeps ownership of its
// window, saver, jobs, database and labels.
type Helper struct {
	app    Application
	opts   Options
	routes map[string]string

	quitting      atomic.Bool
	trayAvailable atomic.Bool
	tray          any

	shutdownOnce sync.Once
	shutdownErr  error
}

// New registers the helper's quit and notification handlers. Call it before
// mygo.App.Run, then call AttachWindow for the main window once that window
// exists. Tray creation itself is deferred to the application's WhenReady.
func New(app Application, opts Options) (*Helper, error) {
	if app == nil {
		return nil, errors.New("lifecycle: application is required")
	}
	if opts.OpenLabel == "" {
		return nil, errors.New("lifecycle: open label is required")
	}
	if opts.OpenAtLoginLabel == "" {
		return nil, errors.New("lifecycle: open-at-login label is required")
	}
	if opts.CloseToTray == nil {
		return nil, errors.New("lifecycle: close-to-tray callback is required")
	}
	if opts.Open == nil {
		return nil, errors.New("lifecycle: open callback is required")
	}
	if opts.Shutdown == nil {
		return nil, errors.New("lifecycle: shutdown callback is required")
	}
	if opts.ShutdownTimeout <= 0 {
		opts.ShutdownTimeout = DefaultShutdownTimeout
	}
	if opts.NewTray == nil {
		opts.NewTray = func(options mygo.TrayOptions) (any, error) {
			return mygo.NewTray(options)
		}
	}
	if opts.RunOnMain == nil {
		opts.RunOnMain = mygo.RunOnMain
	}
	routes := make(map[string]string, len(opts.NotificationRoutes))
	for id, page := range opts.NotificationRoutes {
		routes[id] = page
	}
	h := &Helper{app: app, opts: opts, routes: routes}
	app.OnBeforeQuit(func(*mygo.QuitEvent) { h.quitting.Store(true) })
	app.OnQuitCanceled(func() { h.quitting.Store(false) })
	app.OnQuit(h.quit)
	app.OnNotificationClick(h.notificationClicked)
	app.WhenReady(h.ready)
	return h, nil
}

// AttachWindow converts eligible user closes of window into hides. Register
// the application's save-flushing close handler before this method: an already
// prevented close is never converted into a hide. The returned function removes
// the helper's listener.
func (h *Helper) AttachWindow(window Window) (off func()) {
	if window == nil {
		return func() {}
	}
	return window.OnClose(func(event *mygo.CloseEvent) {
		if event.DefaultPrevented() || h.quitting.Load() || !h.trayAvailable.Load() || !h.closeToTray() {
			return
		}
		event.PreventDefault()
		window.Hide()
	})
}

// Open marshals an open request for page onto the main thread. The empty page
// is the application's default page.
func (h *Helper) Open(page string) {
	h.opts.RunOnMain(func() { h.opts.Open(page) })
}

// Shutdown runs the configured shutdown callback once and returns the first
// invocation's result. Call it from deferred fallback cleanup for startup
// failures and paths that return without an application quit; after OnQuit it
// does not run the callback again.
func (h *Helper) Shutdown(ctx context.Context) error {
	if ctx == nil {
		return errors.New("lifecycle: shutdown context is required")
	}
	h.shutdownOnce.Do(func() {
		h.shutdownErr = h.opts.Shutdown(ctx)
	})
	return h.shutdownErr
}

// TrayAvailable reports whether the tray was created. On platforms or Linux
// desktops without tray support it stays false, so closing the window keeps its
// normal quit behavior rather than stranding a hidden window.
func (h *Helper) TrayAvailable() bool { return h.trayAvailable.Load() }

func (h *Helper) ready() {
	// Phones have neither a system tray nor a close-to-tray window model.
	if runtime.GOOS == "ios" || runtime.GOOS == "android" {
		return
	}
	openItem := &mygo.MenuItem{Label: h.opts.OpenLabel, Click: func(*mygo.MenuItem, *mygo.Window) {
		h.Open("")
	}}
	loginItem := &mygo.MenuItem{
		Label:   h.opts.OpenAtLoginLabel,
		Type:    mygo.MenuItemCheckbox,
		Checked: h.app.OpenAtLogin(),
		Click: func(item *mygo.MenuItem, _ *mygo.Window) {
			enabled := item.IsChecked()
			if err := h.app.SetOpenAtLogin(enabled); err != nil {
				item.SetChecked(!enabled)
				h.report("lifecycle.open_at_login", err)
			}
		},
	}
	menu := mygo.NewMenu([]*mygo.MenuItem{
		openItem,
		loginItem,
		mygo.Separator(),
		{Role: mygo.RoleQuit},
	})
	tray, err := h.opts.NewTray(mygo.TrayOptions{
		ID:             h.opts.TrayID,
		Icon:           h.opts.TrayIcon,
		IconIsTemplate: h.opts.TrayIconIsTemplate,
		ToolTip:        h.opts.TrayTooltip,
		Menu:           menu,
	})
	if err != nil {
		h.report("lifecycle.tray", err)
		return
	}
	h.tray = tray
	h.trayAvailable.Store(true)
}

func (h *Helper) notificationClicked(id string) {
	page, ok := h.routes[id]
	if !ok {
		return
	}
	h.Open(page)
}

func (h *Helper) quit() {
	ctx, cancel := context.WithTimeout(context.Background(), h.opts.ShutdownTimeout)
	defer cancel()
	if err := h.Shutdown(ctx); err != nil {
		h.report("lifecycle.shutdown", err)
	}
}

func (h *Helper) closeToTray() bool {
	return h.opts.CloseToTray != nil && h.opts.CloseToTray()
}

func (h *Helper) report(operation string, err error) {
	if err == nil || h.opts.ReportError == nil {
		return
	}
	h.opts.ReportError(operation, err)
}
