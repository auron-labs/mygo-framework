# Native project guidance

- Run `framework dev`, `framework build`, and `framework doctor`; these use the pinned Go MyGo tool. Bun is not required.
- Run `go test ./...`, `CGO_ENABLED=0 go test ./...`, and `go tool mygo vet .` before committing.
- Create windows and Onevra shells inside `mygo.App.WhenReady` and handle `App.Run` errors.
- Keep application state outside view callbacks. Use stable data keys for dynamic widgets and `ui.Handle` for retained widget identity.
- Use Onevra components and the root shell theme. Do not perform disk or network work inside view builders.
- Preserve the cancelable close handler: a failed preference save must leave the window open. Close the database after the event loop stops.
- Preferences are stored in SQLite. Add schema changes as new Goose migrations in `database/migrations`, numbered after the existing ones.
- This starter uses native views and direct Go calls. Framework web adapter generation and feature scaffolds target the web starter.
