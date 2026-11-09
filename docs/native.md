# Native Go applications

The framework's packages are usable from MyGo's native `ui.View` as well as
from a webview. The native example stores typed settings in the same SQLite database
package the web starter uses (WAL, foreign keys, embedded Goose migrations),
without the reference app's service MCP server or TypeScript frontend.

The native starter includes a pinned `auron-labs/mygo-mcp` dependency.

Create a native project with Go 1.27.3 or newer and the framework CLI installed:

```sh
framework new my-native-app --native
cd my-native-app
go mod download
framework doctor
framework dev
```

The starter pins MyGo v0.8.3 and Onevra v0.2026.18. It includes a Onevra plain
shell, retained Go view state, and typed settings in SQLite.
The shell stays hidden until its first frame is drawn, so it opens with its
content ready while the preference-saving close handler is already installed.
`framework build` packages it through the pinned `go tool mygo`; no Bun or
TypeScript frontend is required. `go run .` also runs the application.

Toggle **Compact spacing**, choose units and a theme, close the window, and
reopen it. The preferences are stored in `my-native-app/app.db` under
`os.UserConfigDir()`; pass `-data-dir DIR` to use another directory. Startup
opens the database and applies pending migrations from `database/migrations`
before the window is created. Add your own migrations there, numbered after
`00001`. The database is closed after the event loop stops.

The typed keys in `preferences.go` cover compact spacing, time zone, units,
theme, profile name, and last import folder. The window edits compact spacing,
units, and theme; the theme applies immediately. Time zone, profile name, and
the import folder are validated and persisted, ready for your own views.
UI builders only change retained Go state. The close handler saves preferences
before allowing the window to close, including during application quit. If a
save fails, it keeps the window open and displays an error so the user can retry.

```sh
go test ./...
CGO_ENABLED=0 go test ./...
go tool mygo vet .
```

The headless tests cover the preference controls, database creation and
migration, persistence across restarts, validation, and visible save errors. Check resizing, focus, keyboard traversal, and light/dark appearance in
the actual window on your target platform.

The same example is available from this checkout:

```sh
go run ./examples/native
go test ./examples/native
```

Its preference directory is `mygo-framework-native` under `os.UserConfigDir()`.

## Background jobs

`mygo-framework/jobs` runs named, cancellable jobs with progress. It is copied
into native projects and has no UI dependency, so the web starter can import it
too.

```go
manager := jobs.New(ctx) // jobs stop when ctx is canceled
err := manager.Start("export", func(ctx context.Context, report jobs.Report) error {
	for i := 1; i <= total; i++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		report(i, total, "Exported "+name(i))
	}
	return nil
})
manager.Cancel("export")
status, ok := manager.Status("export")
off := manager.Subscribe(func(s jobs.Status) { /* every change, including the final one */ })
err = manager.Shutdown(shutdownCtx)
```

- Only one job per name runs at a time. Starting a name that is still running
  returns `jobs.ErrRunning` and leaves the running job untouched; cancel it or
  wait for it first. A finished job's name can be started again.
- A job ends as `Done` (returned nil), `Canceled` (returned an error wrapping
  `context.Canceled`), or `Failed` (any other error, or a panic, which is
  recovered and reported in `Status.Err`). A job that returns nil after being
  canceled is `Done`.
- `Subscribe` callbacks run on the goroutine that changed the status and must
  not block. The first event for a job is always `Running`, and the last is its
  terminal state. MyGo's `Event` and `Channel` deliver to web pages only, so a
  native view subscribes with a callback: the starter calls `Window.Invalidate`
  (safe from any goroutine) and reads `Status` while building the view. A web
  page can forward the same callback to a `mygo.Event` or a bound method's
  `*mygo.Channel`.
- `Shutdown` rejects new jobs, cancels running ones, and waits for them within
  its context. The native starter calls it with a five-second deadline after
  the event loop stops and before the database closes. Jobs must return
  promptly when their context is canceled.

The starter's import flow runs `sampleImport` in `importjob.go` on a folder,
renders `components.Progress` from `Status.Fraction()` and offers **Cancel
import**; replace the function with real work. See
[Importing a folder](#importing-a-folder) for how the folder is chosen.

## Importing a folder

A native window accepts a folder dragged from Finder, Explorer or a Linux file
manager. MyGo reports the dropped paths to the element that asks for them:

```go
zone := ui.Column(c).FillWidth().Padding(16).Border(1, t.Border)
if zone.FileDragOver() {
	zone.Border(2, t.Accent) // files are over the zone and it would take them
}
if files := zone.DroppedFiles(); len(files) > 0 { // absolute paths, since the last frame
	startImport(files)
}
```

`Element.DroppedFiles` and `Element.FileDragOver` are the whole API; no framework
helper is needed. A window can also take files anywhere with
`Window.OnFileDrop`, which gets the files no element took. Drops on Linux
(`internal/linux/surface_drop.go`) and Windows (`internal/windows/surface_drop.go`,
an OLE drop target) arrive as file events; on macOS the pasteboard's
`public.file-url` drag (`internal/darwin/surface_drag.go`) is turned into the
same event by `windowHandler.SurfaceEvent` in MyGo's `content.go`. Files dropped
elsewhere in the window are ignored. A drop is a list of files and folders, so
check `os.Stat(path)` before treating one as a folder.

The starter wires both entry points to the same job and to the persisted last
import folder (`native.import_folder`, saved when the window closes):

- the drop zone in `importjob.go` takes the first dropped folder and says so
  when only files were dropped;
- **Choose folder...** calls `mygo.Dialog.Open(OpenDialogOptions{Directory:
  true})`. The call blocks until the dialog closes, so it runs in a goroutine
  and returns its result through `Window.Update`;
- **Start import** re-imports the remembered folder, or opens the picker when
  none was chosen.

Only the headless path is tested: `ui.Tester.DropFiles` delivers the same
events MyGo's backends do. No real drag from a file manager and no real
`Dialog.Open` was exercised on any platform; see the
[platform status](#platform-status).

## PDF reports

A native `ui.View` cannot be exported to PDF. MyGo's `Page.PrintToPDF` prints a
web page and returns an error for a window that shows native content
(`errNoPage` in `window.go`), and `ui` has no PDF or print API; the only
native capture is a bitmap (`Window.captureContent`, `Tester.Image`).

`mygo-framework/pdfreport` lays out report PDFs directly. It wraps
[`signintech/gopdf`](https://github.com/signintech/gopdf), is pure Go (builds
with `CGO_ENABLED=0`), and is copied into native projects only, so the web
starter does not depend on it. gopdf was chosen over `go-pdf/fpdf`, which is
archived (last push March 2025, last release September 2023), and over
`phpdave11/gofpdf`, whose recent commits only bump its importer dependency.
gopdf is MIT licensed, was last pushed on 31 October 2026, has the smallest dependency
set (one importer library) and embeds TrueType fonts, so text is selectable
and extractable. The package embeds the Go fonts from `golang.org/x/image`,
which cover Latin scripts; embed another font with gopdf directly for other
scripts.

```go
r, err := pdfreport.New(pdfreport.Options{Title: "Weekly report", Footer: "Acme"})
r.Heading("Weekly report", 20)
r.Paragraph("Text wraps to the page width.")
r.Table([]pdfreport.Column{{Title: "Night", Width: 2}, {Title: "Hours", Width: 1, Right: true}}, rows)
r.LineChart(pdfreport.LineChart{Title: "Hours", Series: []pdfreport.Series{{Name: "Hours", Points: points}}})
data, err := r.Bytes() // write it with os.WriteFile or mygo.Dialog.Save's path
```

A4 portrait, a header with the title and a footer with the text and page number
on every page. Tables and paragraphs continue on the next page, and a table
repeats its header row there; a chart stays whole on one page. Content errors
(ragged table rows, an empty chart, non-finite values) are reported by `Bytes`.
Not included: images, cell wrapping inside tables, other page sizes, pie or bar
charts.

`go run ./examples/report -o report.pdf` writes a one-page report with a table
and a line chart. Its test parses the PDF with `ledongthuc/pdf` (test only),
checks that it has exactly one page and that the table, legend and footer text
extract. The output was also rendered with `pdftoppm` and inspected.

## Large tables and long lists

Onevra's `components.TableVirtualized` and `components.List` build only the rows
on screen, so a frame costs the same at 1,000 and 10,000,000 rows.
`go test -bench . ./examples/tablebench` measures a frame (build, layout and
MyGo's CPU paint, through `ui.Tester`) of a four-column table and a one-column
list with a selection, in a 640x480 viewport, on an i7-9700K:

| Rows | Table, steady frame | Table, first frame | Table, one wheel notch | List, steady frame | List, wheel notch |
| --- | --- | --- | --- | --- | --- |
| 1,000 | 0.27 ms | 1.9 ms | 10.7 ms | 0.13 ms | 5.3 ms |
| 100,000 | 0.27 ms | 1.9 ms | 10.7 ms | 0.13 ms | 5.3 ms |
| 10,000,000 | 0.27 ms | 2.0 ms | 10.8 ms | 0.13 ms | 6.3 ms |

Jumping with `ScrollTo` to the middle costs 1.1 ms (table) and 0.4 ms (list) at
every size. A wheel notch includes MyGo's scroll glide, which spans several
frames, so it is the cost of the whole gesture. `TestOnlyVisibleRowsAreBuilt`
guards the property behind these numbers: at 1k, 10k and 100k rows the builders
run for a bounded number of rows at the top and after scrolling to the middle.

Limits:

- The cost is in the data, not the widget. Your `cell` and `row` callbacks run
  during layout and must index into data already in memory; sorting, filtering
  or querying 100,000 rows belongs in a job, with the view reading the result.
- Row height is uniform in the benchmark. Rows that grow to their tallest
  cell are measured as they scroll into view, which was not measured.
- Numbers are for the CPU renderer on one machine; a GPU frame and a 60 Hz
  display's pacing are not measured. A table with many columns or heavy cells
  (images, rich text) costs proportionally more per visible row.

## Time-series charts

`mygo-framework/timechart` draws continuous time series: a clock-aligned time
(or plain numeric) x-axis, line, step and min/max band series, point and
interval event markers, and a hover crosshair with a readout. Wheel or pinch
zooms around the cursor, dragging pans, and a double click resets; with the
plot focused, the arrow keys pan, `=` and `-` zoom and Home resets. It builds
on MyGo's `ui/chart` scales and `Draw` callbacks and depends on nothing else
in this framework, and it is copied into native projects.

```go
rng := timechart.NewRange(start, end) // Unix seconds; bounds the zoom and pan
flow := &timechart.Chart{
	Range: rng, Label: "Airflow", HideXAxis: true,
	Series: []timechart.Series{{ID: "flow", Label: "Airflow", Provider: provider}},
	Events: []timechart.Event{{Start: t0, End: t1, Label: "Apnea"}, {Start: t2, Label: "Arousal"}},
}
spo2 := &timechart.Chart{Range: rng, Label: "Oxygen", YMin: 80, YMax: 100, Series: ...} // linked
// In the view, every frame:
flow.Build(c)
spo2.Build(c)
// When discarding the charts:
flow.Close()
```

- **Linking.** Charts built with the same `*Range` zoom, pan and show the
  crosshair together. Each row's y-axis column has the same width, so stacked
  plots line up; set `HideXAxis` on all but the last row. `Range.Set`,
  `ZoomAt`, `Pan` and `Reset` move the range from code, and `Crosshair` reads
  the shared position. The range never leaves its bounds and never narrows
  below `Range.MinSpan` (a millionth of the bounds by default).
- **Data.** A `Provider` receives `(ctx, from, to, pixelWidth)` and returns
  ascending `Points` decimated for that width. The chart calls it off the UI
  thread for a window 25% wider than the view on each side, keeps drawing the
  previous points while it runs, drops answers for a view it has since left,
  and requests a frame when an answer arrives. It asks again only when the
  view leaves the fetched window or zooms in far enough to make it coarse.
  Providers should return when `ctx` is canceled and log their own errors: a
  failed window leaves the previous points on screen and is not retried until
  the view moves out of it. `Chart.Loaded` reports when the visible window has
  arrived, which is useful in tests.
- **In-memory samples.** `timechart.Decimate(xs, ys, from, to, pixelWidth)`
  finds the window by binary search and keeps the lowest and highest sample of
  each pixel column in time order, plus one neighbour past each edge. The
  result is at most `2*pixelWidth+2` points whatever the zoom, and narrow
  spikes survive. Use it as the provider for slices; a database-backed
  provider returns the same shape from a grouped query.
- **Series kinds.** `Line` joins points, `Step` holds each value until the
  next, and `Band` fills between the lowest and highest point of each pixel
  column, so the same decimated points serve any kind. Colors default to theme
  tokens by series order; set `Series.Color` or `Event.Color` to override.
- **Axes.** Time ticks are clock-aligned in `Chart.Location` (default local
  time), from milliseconds to days, with at most one label per 90 DIPs;
  midnight is labelled with the date. `Numeric` switches the x-axis to plain
  numbers. The y-axis fits the visible data unless `YMin`/`YMax` fix it;
  `FormatY` formats its labels and the readout.

`examples/timechart` stacks three linked charts over a synthetic eight-hour,
25 Hz night (720,000 samples per channel). Run it with
`go run ./examples/timechart`, or serve it to an MCP client with
`go run ./examples/timechart -mcp 127.0.0.1:8080`.
`go test -bench . ./examples/timechart` measures a full frame (build, layout and
paint on MyGo's CPU renderer, points already fetched) against the 16.6 ms
budget of 60 Hz, and the provider work done off the UI thread. A headless
benchmark does not prove frame pacing on a GPU.

## Signed updates

`mygo.Updater` installs signed updates, and it is off until the build is
configured for it. Add an `updates` section with your public key and a release
location to `mygo.json`:

```json
{ "updates": { "publicKey": "<contents of mygo-update.pub>", "github": "you/my-app" } }
```

Create the key pair with `go tool mygo keygen` and keep `mygo-update.key` out of
the repository, in your CI secrets; `mygo build` signs the update files with it
through `MYGO_UPDATER_PRIVATE_KEY`. Without an `updates` section
`mygo.Updater.Enabled()` is false, so the starter never checks for updates and
contacts no server, and the screen shows nothing about updates. With one, the
starter checks once in the background at launch (`updates.go`). A failed check,
such as being offline, is logged and ignored. When a newer version is found, the
screen offers **Install update**, shows download progress, and then **Restart to
finish updating**. MyGo verifies the archive's signature against the public key
built into the app. Web projects call `mygo.Updater` from their own Go service
in the same way; the build configuration is `updates` in `mygo.config.ts`.

Updating a package-manager install, an AppImage, or an app directory the user
cannot write is left to MyGo, which reports it as a failed install.

## Platform status

What has actually been run, as of 9 November 2026. "Compiled" means
`CGO_ENABLED=0 go build` and `go test -c` of the generated native starter (and
`go build ./...` of this repository) succeeded from a Linux amd64 host for that
target; it says nothing about behaviour. The macOS and Windows runtime cells are
unverified because no such machine was available to this work.

| Area | Linux amd64 | Linux arm64 | macOS amd64, arm64 | Windows amd64 |
| --- | --- | --- | --- | --- |
| Go tests (headless UI, jobs, settings, SQLite, PDF) | Run, passing | Compiled | CI job added, not yet run | CI job added, not yet run |
| Starter and examples build, `CGO_ENABLED=0` | Run | Compiled | Compiled, CI job added | Compiled, CI job added |
| Native window opens, draws, closes cleanly | Run under Xvfb with software GL, exits 0 on SIGTERM | Not verified | Not verified | Not verified |
| SQLite preferences in the real app | Run (the window created `app.db`) | Not verified | Not verified; covered by CI tests once they run | Not verified; covered by CI tests once they run |
| Dark mode and theme source | Headless selection only | Not verified | Not verified | Not verified |
| Folder dropped from a file manager | Headless `Tester.DropFiles` only | Not verified | Not verified | Not verified |
| `Dialog.Open` folder picker | Not verified | Not verified | Not verified | Not verified |
| `mygo build` package | Run: binary, `.deb`, `.tar.gz`, `PKGBUILD` | Not verified | `.app` bundle not verified | Not verified |
| DMG, Windows installer | n/a | n/a | Not verified | Not verified |
| Signing and notarisation | n/a | n/a | Not verified; no signing secrets are configured | Not verified; no signing secrets are configured |

The `cross-platform` job in `.github/workflows/ci.yml` runs `mise run test:go`,
`mise run build:go` and `mise run test:generated` (a fresh native and web
project, its tests, build and `mygo vet`) on `macos-latest` and
`windows-latest`. It has not run yet. Hosted runners have no display session
the application can be assumed to use, so the job does not open a window,
dialog or package; those rows stay unverified until someone runs them on a real
machine. If a window launch becomes possible on a runner, add it as a separate
step instead of folding it into these.

## Inspect native views through MCP

Generated native projects include a headless MCP harness for their real view:

```sh
go run . -mcp 127.0.0.1:8080
```

From this checkout, use `go run ./examples/native -mcp 127.0.0.1:8080`.
Connect an MCP client using Streamable HTTP to `http://127.0.0.1:8080/mcp`.
Ask it to list sessions, inspect the view, click **Toggle compact spacing**,
and capture a screenshot. Stop the harness with Ctrl+C.

The harness reuses the desktop view and Onevra theme with independent in-memory
state; it does not open a window or load or save preferences. Restart to reset it.
Extend `nativeFixture` in `mcp.go` with test state and services as your view grows.
Only an explicit loopback IP and port are accepted. The endpoint has no
authentication, so keep it local.

MyGo MCP operates on isolated native fixtures. It cannot control running windows,
OS dialogs, or webviews. The web starter's authenticated service MCP server is
separate. See the [MyGo MCP agent guide](https://github.com/auron-labs/mygo-mcp/blob/9f095775017aafe64c22e934d2178fa7b6657209/docs/AGENT_GUIDE.md)
for tools and observation rules. The included MCP integration test covers
discovery, inspection, checkbox interaction, screenshots, fixture isolation,
and shutdown over a real loopback connection.

## Reusing the packages in a separate application

This repository is source-first and currently declares `module mygo-framework`.
It is not directly importable with `go get github.com/auron-labs/mygo-framework`.
An external native consumer can use an authenticated sibling checkout:

```sh
gh repo clone auron-labs/mygo-framework ../mygo-framework
go mod edit -require=mygo-framework@v0.0.0
go mod edit -replace=mygo-framework=../mygo-framework
```

The replacement path is relative to the consumer's `go.mod`; adjust it for
nested `source/` layouts. Pin the framework checkout to a reviewed commit
for reproducible builds. Import `mygo-framework/settings` and a store such as
`mygo-framework/settings/sqlite` directly. Alternatively,
copy the source-first packages into the application and change import paths
as with a generated application.

Use MyGo's native `ui.View` for window content and native `mygo.json`
configuration for packaging. Onevra supplies native components and shells;
keep the application's MyGo version compatible with the selected Onevra
version. Go selects the higher MyGo requirement, so verify framework packages
against that version when upgrading.

## Current CLI boundary

`framework new` creates the web starter; `framework new NAME --native` creates
the native starter. For native projects identified by `mygo.json`, `framework
dev`, `build`, and `doctor` skip web adapter generation and delegate through
`go tool mygo`. Web projects retain generation and `bun run mygo` delegation.

The native starter intentionally uses direct Go calls and typed settings.
`framework make feature` and framework web/MCP adapter generation target the
web starter; they are not part of the native starter workflow.

Native dependencies still need their target-platform review. In particular,
MyGo's packager builds with cgo disabled; an application using a cgo-backed
storage engine must use a cgo-enabled Go build or provide a separate packaging
step. The example itself adds no cgo dependency.
