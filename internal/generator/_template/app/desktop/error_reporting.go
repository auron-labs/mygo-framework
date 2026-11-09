package desktop

import "mygo-framework/errorreport"

// ErrorReporting tells pages where to report their errors.
type ErrorReporting struct{}

// NewErrorReporting constructs the service bound to pages.
func NewErrorReporting() *ErrorReporting { return &ErrorReporting{} }

// DSN returns the DSN the application reports errors to, or "" when error
// reporting is off.
func (*ErrorReporting) DSN() string { return errorreport.DSN() }
