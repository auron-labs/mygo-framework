package lifecycle

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo"
)

type fakeApp struct {
	ready         []func()
	beforeQuit    []func(*mygo.QuitEvent)
	quitCanceled  []func()
	quit          []func()
	notification  []func(string)
	login         bool
	setLoginErr   error
	setLoginCalls []bool
}

func (a *fakeApp) WhenReady(fn func()) { a.ready = append(a.ready, fn) }
func (a *fakeApp) OnBeforeQuit(fn func(*mygo.QuitEvent)) (off func()) {
	a.beforeQuit = append(a.beforeQuit, fn)
	return func() {}
}

func (a *fakeApp) OnQuitCanceled(fn func()) (off func()) {
	a.quitCanceled = append(a.quitCanceled, fn)
	return func() {}
}

func (a *fakeApp) OnQuit(fn func()) (off func()) {
	a.quit = append(a.quit, fn)
	return func() {}
}

func (a *fakeApp) OnNotificationClick(fn func(string)) (off func()) {
	a.notification = append(a.notification, fn)
	return func() {}
}
func (a *fakeApp) OpenAtLogin() bool { return a.login }
func (a *fakeApp) SetOpenAtLogin(open bool) error {
	a.setLoginCalls = append(a.setLoginCalls, open)
	if a.setLoginErr != nil {
		return a.setLoginErr
	}
	a.login = open
	return nil
}

func (a *fakeApp) readyNow() {
	for _, fn := range a.ready {
		fn()
	}
}

func (a *fakeApp) beforeQuitNow() {
	for _, fn := range a.beforeQuit {
		fn(&mygo.QuitEvent{})
	}
}

func (a *fakeApp) quitCanceledNow() {
	for _, fn := range a.quitCanceled {
		fn()
	}
}

func (a *fakeApp) quitNow() {
	for _, fn := range a.quit {
		fn()
	}
}

func (a *fakeApp) notificationNow(id string) {
	for _, fn := range a.notification {
		fn(id)
	}
}

type fakeWindow struct {
	closeHandlers []func(*mygo.CloseEvent)
	hides         int
}

func (w *fakeWindow) Hide() { w.hides++ }
func (w *fakeWindow) OnClose(fn func(*mygo.CloseEvent)) (off func()) {
	w.closeHandlers = append(w.closeHandlers, fn)
	return func() {}
}

func (w *fakeWindow) close() *mygo.CloseEvent {
	event := &mygo.CloseEvent{}
	for _, fn := range w.closeHandlers {
		fn(event)
	}
	return event
}

type lifecycleHarness struct {
	app           *fakeApp
	helper        *Helper
	closeToTray   bool
	trayErr       error
	trayCreated   bool
	trayOptions   mygo.TrayOptions
	opens         []string
	marshaled     int
	shutdownCalls int
	shutdownErr   error
	reportOps     []string
	reportErrors  []error
}

func newLifecycleHarness(t *testing.T, change func(*lifecycleHarness, *Options)) *lifecycleHarness {
	t.Helper()
	h := &lifecycleHarness{app: &fakeApp{}}
	options := Options{
		TrayID:             "main",
		TrayIcon:           []byte("png"),
		TrayIconIsTemplate: true,
		TrayTooltip:        "Application",
		OpenLabel:          "Open",
		OpenAtLoginLabel:   "Open at login",
		NotificationRoutes: map[string]string{"sample-import": "import"},
		ShutdownTimeout:    time.Second,
	}
	if change != nil {
		change(h, &options)
	}
	options.CloseToTray = func() bool { return h.closeToTray }
	options.Open = func(page string) { h.opens = append(h.opens, page) }
	options.Shutdown = func(context.Context) error {
		h.shutdownCalls++
		return h.shutdownErr
	}
	options.ReportError = func(operation string, err error) {
		h.reportOps = append(h.reportOps, operation)
		h.reportErrors = append(h.reportErrors, err)
	}
	options.NewTray = func(trayOptions mygo.TrayOptions) (any, error) {
		h.trayCreated = true
		h.trayOptions = trayOptions
		if h.trayErr != nil {
			return nil, h.trayErr
		}
		return &struct{}{}, nil
	}
	options.RunOnMain = func(fn func()) {
		h.marshaled++
		fn()
	}
	helper, err := New(h.app, options)
	if err != nil {
		t.Fatal(err)
	}
	h.helper = helper
	return h
}

func TestCloseToTrayHidesAnEligibleUserClose(t *testing.T) {
	h := newLifecycleHarness(t, nil)
	h.closeToTray = true
	h.app.readyNow()
	window := &fakeWindow{}
	h.helper.AttachWindow(window)

	event := window.close()

	if !event.DefaultPrevented() || window.hides != 1 {
		t.Fatalf("prevented=%v hides=%d, want a prevented close and one hide", event.DefaultPrevented(), window.hides)
	}
}

func TestCloseProceedsWhenCloseToTrayIsDisabled(t *testing.T) {
	h := newLifecycleHarness(t, nil)
	h.app.readyNow()
	window := &fakeWindow{}
	h.helper.AttachWindow(window)

	event := window.close()

	if event.DefaultPrevented() || window.hides != 0 {
		t.Fatalf("prevented=%v hides=%d, want the close to proceed", event.DefaultPrevented(), window.hides)
	}
}

func TestCloseProceedsWhenTrayIsUnavailable(t *testing.T) {
	trayErr := errors.New("no app indicator")
	h := newLifecycleHarness(t, func(h *lifecycleHarness, _ *Options) { h.trayErr = trayErr })
	h.closeToTray = true
	h.app.readyNow()
	window := &fakeWindow{}
	h.helper.AttachWindow(window)

	event := window.close()

	if event.DefaultPrevented() || window.hides != 0 {
		t.Fatalf("prevented=%v hides=%d, want the close to proceed", event.DefaultPrevented(), window.hides)
	}
	if h.helper.TrayAvailable() {
		t.Fatal("tray reported available after creation failed")
	}
	if len(h.reportOps) != 1 || h.reportOps[0] != "lifecycle.tray" || !errors.Is(h.reportErrors[0], trayErr) {
		t.Fatalf("reports = %v %v, want the tray error", h.reportOps, h.reportErrors)
	}
}

func TestCloseProceedsWhileApplicationIsQuitting(t *testing.T) {
	h := newLifecycleHarness(t, nil)
	h.closeToTray = true
	h.app.readyNow()
	window := &fakeWindow{}
	h.helper.AttachWindow(window)

	h.app.beforeQuitNow()
	event := window.close()

	if event.DefaultPrevented() || window.hides != 0 {
		t.Fatalf("prevented=%v hides=%d, want the quit close to proceed", event.DefaultPrevented(), window.hides)
	}
}

func TestQuitCanceledLetsALaterUserCloseHideAgain(t *testing.T) {
	h := newLifecycleHarness(t, nil)
	h.closeToTray = true
	h.app.readyNow()
	window := &fakeWindow{}
	h.helper.AttachWindow(window)

	h.app.beforeQuitNow()
	h.app.quitCanceledNow()
	event := window.close()

	if !event.DefaultPrevented() || window.hides != 1 {
		t.Fatalf("prevented=%v hides=%d, want close-to-tray after the canceled quit", event.DefaultPrevented(), window.hides)
	}
}

func TestSaverPreventedCloseIsNeverConvertedIntoAHide(t *testing.T) {
	h := newLifecycleHarness(t, nil)
	h.closeToTray = true
	h.app.readyNow()
	window := &fakeWindow{}
	window.OnClose(func(event *mygo.CloseEvent) { event.PreventDefault() })
	h.helper.AttachWindow(window)

	event := window.close()

	if !event.DefaultPrevented() {
		t.Fatal("the saver's prevented close was not preserved")
	}
	if window.hides != 0 {
		t.Fatalf("hides=%d, want none", window.hides)
	}
}

func TestShutdownRunsOnceSynchronouslyAndReportsItsError(t *testing.T) {
	shutdownErr := errors.New("close database")
	h := newLifecycleHarness(t, func(h *lifecycleHarness, _ *Options) { h.shutdownErr = shutdownErr })
	var deadline time.Time
	h.helper.opts.Shutdown = func(ctx context.Context) error {
		h.shutdownCalls++
		deadline, _ = ctx.Deadline()
		return h.shutdownErr
	}

	h.app.quitNow()

	if h.shutdownCalls != 1 {
		t.Fatalf("shutdown calls = %d before OnQuit returned, want 1 synchronous call", h.shutdownCalls)
	}
	if deadline.IsZero() {
		t.Fatal("shutdown context has no deadline")
	}
	if len(h.reportOps) != 1 || h.reportOps[0] != "lifecycle.shutdown" || !errors.Is(h.reportErrors[0], shutdownErr) {
		t.Fatalf("reports = %v %v, want the shutdown error", h.reportOps, h.reportErrors)
	}
	h.app.quitNow()
	if err := h.helper.Shutdown(t.Context()); !errors.Is(err, shutdownErr) {
		t.Fatalf("fallback shutdown error = %v, want the first error", err)
	}
	if h.shutdownCalls != 1 {
		t.Fatalf("shutdown calls after repeated quit and fallback = %d, want 1", h.shutdownCalls)
	}
}

func TestTrayOpenAndNotificationRouteOpenPagesOnTheMainThread(t *testing.T) {
	h := newLifecycleHarness(t, nil)
	h.app.readyNow()
	if !h.trayCreated || !h.helper.TrayAvailable() {
		t.Fatal("tray was not created")
	}
	if h.trayOptions.ID != "main" || string(h.trayOptions.Icon) != "png" || !h.trayOptions.IconIsTemplate || h.trayOptions.ToolTip != "Application" {
		t.Fatalf("tray options = %+v", h.trayOptions)
	}
	items := h.trayOptions.Menu.Items()
	if len(items) != 4 {
		t.Fatalf("tray menu items = %d, want 4", len(items))
	}
	items[0].Click(items[0], nil)
	if len(h.opens) != 1 || h.opens[0] != "" {
		t.Fatalf("opens after tray item = %q, want the default page", h.opens)
	}
	if items[3].Role != mygo.RoleQuit {
		t.Fatalf("last tray item role = %q, want quit", items[3].Role)
	}

	h.app.notificationNow("sample-import")
	h.app.notificationNow("unknown")

	if len(h.opens) != 2 || h.opens[1] != "import" {
		t.Fatalf("opens after notifications = %q, want the import page once", h.opens)
	}
	if h.marshaled != 2 {
		t.Fatalf("main-thread marshals = %d, want 2", h.marshaled)
	}
}

func TestOpenAtLoginCheckboxUsesTheOperatingSystemStore(t *testing.T) {
	h := newLifecycleHarness(t, func(h *lifecycleHarness, _ *Options) { h.app.login = true })
	h.app.readyNow()
	item := h.trayOptions.Menu.Items()[1]
	if item.Type != mygo.MenuItemCheckbox || !item.IsChecked() {
		t.Fatalf("login item type=%v checked=%v, want a checked checkbox", item.Type, item.IsChecked())
	}

	// The menu toggles the item before Click; a checked item therefore asks to
	// turn the login item off.
	item.SetChecked(false)
	item.Click(item, nil)

	if len(h.app.setLoginCalls) != 1 || h.app.setLoginCalls[0] {
		t.Fatalf("SetOpenAtLogin calls = %v, want [false]", h.app.setLoginCalls)
	}
	if h.app.login {
		t.Fatal("the operating-system store was not updated")
	}
}

func TestOpenAtLoginFailureRevertsTheCheckboxAndReports(t *testing.T) {
	setErr := errors.New("login item denied")
	h := newLifecycleHarness(t, func(h *lifecycleHarness, _ *Options) { h.app.setLoginErr = setErr })
	h.app.readyNow()
	item := h.trayOptions.Menu.Items()[1]

	item.SetChecked(true)
	item.Click(item, nil)

	if len(h.app.setLoginCalls) != 1 || !h.app.setLoginCalls[0] {
		t.Fatalf("SetOpenAtLogin calls = %v, want [true]", h.app.setLoginCalls)
	}
	if item.IsChecked() {
		t.Fatal("failed login-item change stayed checked")
	}
	if len(h.reportOps) != 1 || h.reportOps[0] != "lifecycle.open_at_login" || !errors.Is(h.reportErrors[0], setErr) {
		t.Fatalf("reports = %v %v, want the login-item error", h.reportOps, h.reportErrors)
	}
}

func TestNewValidatesRequiredCallbacksAndLabels(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*Options)
		want   string
	}{
		{name: "open label", change: func(o *Options) { o.OpenLabel = "" }, want: "open label"},
		{name: "login label", change: func(o *Options) { o.OpenAtLoginLabel = "" }, want: "open-at-login label"},
		{name: "close to tray", change: func(o *Options) { o.CloseToTray = nil }, want: "close-to-tray callback"},
		{name: "open", change: func(o *Options) { o.Open = nil }, want: "open callback"},
		{name: "shutdown", change: func(o *Options) { o.Shutdown = nil }, want: "shutdown callback"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			options := Options{
				OpenLabel: "Open", OpenAtLoginLabel: "Open at login",
				CloseToTray: func() bool { return false }, Open: func(string) {}, Shutdown: func(context.Context) error { return nil },
			}
			tc.change(&options)
			_, err := New(&fakeApp{}, options)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
	if _, err := New(nil, Options{}); err == nil {
		t.Fatal("nil application succeeded")
	}
}
