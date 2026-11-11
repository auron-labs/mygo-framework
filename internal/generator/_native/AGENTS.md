# Native project guidance

- Run `framework dev`, `framework build`, and `framework doctor`; these use the pinned Go MyGo tool. Bun is not required.
- Run `go test ./...`, `CGO_ENABLED=0 go test ./...`, and `go tool mygo vet .` before committing.
- Create windows and Onevra shells inside `mygo.App.WhenReady` and handle `App.Run` errors.
- Keep application state outside view callbacks. State shared with other goroutines or persisted lives in a `state.Cell` (`main.go` shows the pattern): the view reads `Get()` once per build, controls bind to local copies and commit with `OnChange` and `Update`, and `Subscribe` plus `win.Invalidate` redraws. Subscribers run on the writer's goroutine and must not block; wrap main-thread-only APIs in `mygo.RunOnMain`. Plain fields are for UI-only values. Use stable data keys for dynamic widgets and `ui.Handle` for retained widget identity.
- Use Onevra components and the root shell theme. Do not perform disk or network work inside view builders.
- Preferences save automatically through `state.Autosave`. Preserve the cancelable close handler: it flushes the saver, and a failed save must leave the window open. Close the saver, then the database, after the event loop stops.
- Preferences are typed settings in SQLite (`preferences.go`). Add schema changes as new Goose migrations in `database/migrations`, numbered after the existing ones.
- Run long work with `jobs.Manager` (`importjob.go` shows the pattern): `Start` returns `jobs.ErrRunning` while a job of that name runs, jobs must honor their context, and views read `Status` and redraw from a `Subscribe` callback via `Window.Invalidate`. `run` shuts the manager down before the database closes.
- This starter uses native views and direct Go calls. Framework web adapter generation targets the web starter; `framework make feature NAME` writes a native view, store, migration and test here.
- Put user-visible strings in `locales/en.json` and call the generated `locales` functions; run `go tool mygo generate` after editing a catalog. Product names stay untranslated.
- Updates are opt-in through an `updates` section in `mygo.json`; never commit `mygo-update.key`. Without it `mygo.Updater.Enabled()` is false and nothing is contacted.
- `profiles` opens one SQLite file per validated profile name; never join a user-supplied name into a path yourself.
- Errors reach Sentry only through `errorreport` and only when `SENTRY_DSN` is set. Report at boundaries (`run`, jobs); call `errorreport.Flush()` before any new exit path. Do not add tracing or replay.
- The import flow takes a folder from a drop zone (`Element.DroppedFiles`/`FileDragOver`) or from `mygo.Dialog.Open{Directory: true}`. `Dialog.Open` blocks, so call it from a goroutine and apply the result with `Window.Update` when it touches plain fields (write shared state through its cell). Persist the choice as `native.import_folder`.
- `pdfreport` writes report PDFs (text, tables, line charts, header and footer); native windows cannot print themselves to PDF. Use `components.TableVirtualized` or `components.List` for long data and keep the data in memory or behind a job, since only visible rows are built.
