// Package l10n holds Onevra's own strings: the names components and Shells
// give their internal controls. It is a MyGo library catalog in namespace
// "onevra", so its messages follow the language the app shows, and an app
// changes or translates them with keys "onevra.<key>" in its own catalogs.
package l10n

import (
	"embed"
	"io/fs"
	"strings"

	"github.com/egoist/mygo/i18n"
)

// Namespace prefixes the keys with which applications override these messages.
const Namespace = "onevra"

//go:embed *.json
var files embed.FS

var catalog = func() *i18n.Catalog {
	sub, err := fs.Sub(files, ".")
	if err != nil {
		panic(err)
	}
	c, err := i18n.NewLibraryCatalog(sub, "en", Namespace)
	if err != nil {
		panic(err)
	}
	return c
}()

// Messages says Onevra's strings in one language.
type Messages struct{ locale string }

// In returns the messages in locale, the language a window shows
// (ui.Context.Locale); "" is English.
func In(locale string) Messages { return Messages{locale} }

// Text returns the message of key, which takes no arguments.
func (m Messages) Text(key string) string { return catalog.Format(m.locale, key, nil) }

// Label returns the message of key naming a control after an application
// label, trimmed so that an empty label leaves no stray space.
func (m Messages) Label(key, label string) string {
	return strings.TrimSpace(catalog.Format(m.locale, key, map[string]any{"label": label}))
}

// Format returns the message of key with its arguments.
func (m Messages) Format(key string, args map[string]any) string {
	return catalog.Format(m.locale, key, args)
}

// Count returns the message of key for count, a plural-aware integer.
func (m Messages) Count(key string, count int) string {
	return catalog.Format(m.locale, key, map[string]any{"count": count})
}
