// Package sidebar provides a native window with a replaceable page container
// and a default sidebar-and-content layout built on MyGo. NewShell creates
// the window and applies Onevra's theme before building content. Callers own
// application lifetime, business state, localized text and actions.
// Content builders and Shell methods run synchronously on the UI thread.
// On phones the window has no sidebars: ShellOptions.Tabs becomes a bottom
// tab bar, and routes outside it show as pushed screens with back, reached
// from the top bar's TopActions.
package sidebar
