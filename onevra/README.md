# Onevra

Native desktop UI components for MyGo, maintained as packages of
`github.com/auron-labs/mygo-framework`. Use the controls, themes, shells, and
Lucide icons in any MyGo application; framework bootstrap and persistence are
optional.

## Install

Requires Go 1.27.3 or newer. From your application's module:

```sh
go get github.com/auron-labs/mygo-framework@latest
```

Onevra shares the framework's release tags and currently uses MyGo v0.10.0.
Pin a framework tag for reproducible builds. There is no separate Onevra module
or `replace` directive to configure.

## Usage

```go
package main

import (
    "log"

    "github.com/auron-labs/mygo-framework/onevra/components"
    "github.com/auron-labs/mygo-framework/onevra/shells/plain"
    "github.com/auron-labs/mygo-framework/onevra/theme"
    "github.com/egoist/mygo"
    "github.com/egoist/mygo/ui"
)

func main() {
    mygo.App.WhenReady(func() {
        shell := plain.NewShell(plain.ShellOptions{
            Title: "Hello Onevra",
            Theme: theme.Onevra,
            Content: func(c *ui.Context) {
                ui.Column(c).Fill().Padding(24).Children(func() {
                    components.Text(c, "Hello, Onevra!")
                })
            },
        })
        shell.Window().OnClosed(func() { mygo.App.Quit() })
    })
    if err := mygo.App.Run(); err != nil {
        log.Fatal(err)
    }
}
```

Create windows inside `mygo.App.WhenReady`. Applications own mutable state,
persistence, asynchronous work, and application lifetime. Components compose
MyGo primitives; MyGo owns rendering, input, and native integration.

| Package | Purpose |
| --- | --- |
| [`components`](components/doc.go) | Themed controls, lists, tables, menus, and dialogs |
| `components/chart`, `components/graph`, `components/treemap` | Data visualizations |
| `components/chat`, `components/command`, `components/flow` | Chat, command palette, and flow views |
| `components/gesture`, `components/shaders` | Gesture handling and visual effects |
| [`theme`](theme/doc.go) | Optional Onevra palettes and frame theme setup |
| `shells/plain`, `shells/sidebar` | Native window shells |
| [`icons/lucide`](icons/lucide/doc.go) | Lazily parsed Lucide SVG icons |

See the [native application guide](../docs/native.md),
[native example](../examples/native), and [time chart example](../examples/timechart)
for state, persistence, headless testing, and MCP integration.

## Migrate from upstream

Replace the import prefix `github.com/oneo-me/onevra/source` with
`github.com/auron-labs/mygo-framework/onevra` throughout your application,
then run `go mod tidy`. Package names and exported APIs are preserved from the
imported upstream commit. Update all imports together: named types from the
old and new paths are distinct Go types. Dependencies exposing upstream Onevra
types must migrate too.

## Maintenance and attribution

The library was imported from
[oneo-me/onevra at `3af36d2c8d0bc847ab6408e0bfc409b7d8f2486d`](https://github.com/oneo-me/onevra/tree/3af36d2c8d0bc847ab6408e0bfc409b7d8f2486d/source).
This directory contains its components, shells, themes, icons, internal
localization, tests, and icon generator. Upstream's gallery, starter apps,
website, and application packaging assets are outside this library import.

From the framework root, run `go test ./onevra/...`. These packages also run
through the framework's normal tests and builds.

Generated Lucide accessors retain their [pinned provenance](icons/lucide/upstream.json).
To update them, use a clean Lucide checkout at a full commit hash:

```sh
cd onevra
go run ./tools/lucidegen -upstream /path/to/lucide -commit FULL_COMMIT_HASH
```

Append `-check` to verify without writing. The optional `-gallery-out PATH`
writes a catalog for a consumer gallery. Consumers need no Lucide checkout or
generation step.

Original Onevra code retains the [ONEO MIT license](LICENSE). Components adapted
from MyGo retain the [MyGo MIT notice](components/LICENSE.mygo). Icons retain
[Lucide ISC and Feather MIT notices](icons/lucide/LICENSE).
