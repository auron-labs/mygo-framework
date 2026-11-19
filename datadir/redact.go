package datadir

import (
	"os"
	"runtime"
	"strings"

	"github.com/egoist/mygo"
)

// Redactor returns a function that replaces the paths identifying the user in
// text: the home directory becomes ~, and on phones the application data
// directory becomes <app data>. Resolve it once and reuse it for many strings.
func Redactor() func(string) string {
	return redactor(runtime.GOOS,
		func() (string, error) { return mygo.App.Path(mygo.PathUserData) },
		os.UserHomeDir)
}

// redactor takes the platform and its lookups as arguments so tests can
// exercise the phone branch without a phone. The data directory is replaced
// before the home directory, which contains it in the iOS sandbox.
func redactor(goos string, userData, home func() (string, error)) func(string) string {
	var dataDirectory string
	if isPhone(goos) {
		// mygo.App.Path creates the directory, so it is only consulted on
		// phones, where the sandboxed path cannot be derived another way.
		if directory, err := userData(); err == nil {
			dataDirectory = directory
		}
	}
	homeDirectory, err := home()
	if err != nil {
		homeDirectory = ""
	}

	return func(text string) string {
		if dataDirectory != "" {
			text = strings.ReplaceAll(text, dataDirectory, "<app data>")
		}
		if homeDirectory != "" {
			text = strings.ReplaceAll(text, homeDirectory, "~")
		}
		return text
	}
}
