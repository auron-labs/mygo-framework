# Native project guidance

- Run `framework dev`, `framework build`, and `framework doctor`; these use the pinned Go MyGo tool. Bun is not required.
- Run `go test ./...`, `CGO_ENABLED=0 go test ./...`, and `go tool mygo vet .` before committing.
- Create windows and Onevra shells inside `mygo.App.WhenReady` and handle `App.Run` errors.
- Keep application state outside view callbacks. Use stable data keys for dynamic widgets and `ui.Handle` for retained widget identity.
- Use Onevra components and the root shell theme. Do not perform disk or network work inside view builders.
- Preserve the cancelable close handler: a failed preference save must leave the window open. Close the database after the event loop stops.
- Preferences are typed settings in SQLite (`preferences.go`). Add schema changes as new Goose migrations in `database/migrations`, numbered after the existing ones.
- Run long work with `jobs.Manager` (`importjob.go` shows the pattern): `Start` returns `jobs.ErrRunning` while a job of that name runs, jobs must honor their context, and views read `Status` and redraw from a `Subscribe` callback via `Window.Invalidate`. `run` shuts the manager down before the database closes.
- This starter uses native views and direct Go calls. Framework web adapter generation and feature scaffolds target the web starter.
- The import flow takes a folder from a drop zone (`Element.DroppedFiles`/`FileDragOver`) or from `mygo.Dialog.Open{Directory: true}`. `Dialog.Open` blocks, so call it from a goroutine and apply the result with `Window.Update`. Persist the choice as `native.import_folder`.
