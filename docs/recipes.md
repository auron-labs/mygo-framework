# Capability recipes

These task guides use the reference application in this repository. Run the Go
commands from the repository root. Framework packages are imported from
`github.com/auron-labs/mygo-framework`; application packages start with the
`module` value from `go.mod`. After importing a framework package for the first
time, run `go mod tidy` to add its dependencies.

The generated starter already demonstrates a complete Notes feature. Start with
that code when a recipe needs bootstrap registration, validation, or a database
transaction.

## Add a complete feature

Use the feature scaffold when the feature owns a model and migration:

```sh
framework make feature Bookmark
bun run generate
```

The command creates these hand-written files and refreshes the four
framework-owned Go files:

```text
app/models/bookmark.go
app/requests/bookmark.go
app/services/bookmark.go
app/bootstrap/bookmark.go
database/migrations/<sequence>_create_bookmark_records.sql
```

Implement the service and frontend without editing generated files. If an
exposed method signature or annotation changes, refresh both generator layers:

```sh
framework generate
bun run generate
framework generate --check
```

Method-body-only edits do not need `framework generate`.

In a native project (one with `mygo.json`) the same command writes
`bookmark.go`, `bookmark_test.go` and a migration instead, and there is nothing
to regenerate; see [Adding a feature](native.md#adding-a-feature).

## Report errors to Sentry

Set `SENTRY_DSN` in the environment the app is launched from:

```sh
SENTRY_DSN=https://<key>@<host>/<project> framework dev
```

Without it the Go and browser SDKs stay uninitialised and send nothing. The Go
side initialises in `main` with `errorreport.Init`; `logging.ReportUnexpected`,
failed and panicked `jobs` and startup failures report through
`errorreport.Capture`. The page asks the bound `ErrorReporting.dsn()` service
for the DSN at startup, so no DSN is compiled into the frontend. Call
`errorreport.Flush()` before any new exit path.

## Save a typed setting

Define keys once and register them with one `settings.Values` instance. The
SQLite store borrows the application's existing `database.SQL` pool; it does
not own or close that pool.

For example, add `app/bootstrap/settings.go`:

```go
package bootstrap

import (
	"fmt"

	"github.com/auron-labs/mygo-framework/database"
	"github.com/auron-labs/mygo-framework/settings"
	settingssqlite "github.com/auron-labs/mygo-framework/settings/sqlite"

	"github.com/samber/do/v2"
)

var noteSortOrder = settings.NewKey("notes.sort_order", "oldest", func(value string) error {
	if value != "oldest" && value != "newest" {
		return fmt.Errorf("sort order must be oldest or newest")
	}
	return nil
})

func provideSettings(injector do.Injector) (*settings.Values, error) {
	database, err := do.Invoke[*database.Database](injector)
	if err != nil {
		return nil, fmt.Errorf("resolve database for settings: %w", err)
	}
	store, err := settingssqlite.New(database.SQL)
	if err != nil {
		return nil, err
	}
	return settings.New(store, noteSortOrder)
}
```

Register `provideSettings` in `app/bootstrap.New` next to the database provider.
Resolve it from `resolveRequired` if startup must fail when its definitions are
invalid. Inject `*settings.Values` into the service that owns the preference,
then save and read it with the request context:

```go
if err := settings.Set(ctx, values, noteSortOrder, "newest"); err != nil {
	return fmt.Errorf("save note sort order: %w", err)
}

sortOrder, err := settings.Get(ctx, values, noteSortOrder)
if err != nil {
	return fmt.Errorf("read note sort order: %w", err)
}
```

The embedded `00002_create_mygo_framework_settings.sql` migration owns the
table. Do not create it with GORM `AutoMigrate`. Missing values return the key's
declared default; invalid stored JSON returns an error.

To let a command-line flag or environment variable override a stored value,
read it yourself, wrap it with `settings.NewOverride(key, value)`, and pass the
store through `settings.NewOverlay(store, flags, environment)` before calling
`settings.New`. Reads then resolve flags, environment values, the stored value,
and the key's default, in that order. Overrides are never saved: `Set` and
`Delete` still change the stored value, but the override wins while it is in
force.

To keep settings in their own file rather than the application database, use
`settingsbbolt.Open(path)` from `github.com/auron-labs/mygo-framework/settings/bbolt` as the store and
close it on shutdown. It fails after one second if another process holds the
file.

## Read an OS-protected secret

Create the MyGo secret adapter after desktop startup. Use a stable application
and environment namespace, then inject the `secrets.SecretStore` interface into
the service that needs it:

```go
store, err := mygosecrets.New(profile.Name, "development")
if err != nil {
	return fmt.Errorf("create secret store: %w", err)
}

token, err := store.Get(ctx, "sync-token")
if errors.Is(err, secrets.ErrNotFound) {
	return errors.New("configure the sync token first")
}
if err != nil {
	return fmt.Errorf("read sync token: %w", err)
}
```

The snippet needs these imports:

```go
import (
	"errors"
	"fmt"

	"github.com/auron-labs/mygo-framework/app/profile"
	"github.com/auron-labs/mygo-framework/secrets"
	mygosecrets "github.com/auron-labs/mygo-framework/secrets/mygo"
)
```

Use `store.Set` to provision the value and `store.Delete` to remove it. The
adapter never falls back to plaintext. Where the platform has no credential
store, such as Linux without a Secret Service on the session bus, calls fail
with an error for which `errors.Is(err, errors.ErrUnsupported)` holds; keep the
secret elsewhere or ask for it each time. Keep the in-memory adapter for tests
and handle a locked credential store as an operational error. Do not assume MyGo's OS secret API works from the GUI-independent
`appctl` process.

## Call an HTTP JSON API

Construct one configured client in bootstrap and inject it into services. Every
request receives the caller's context and an operation name for safe metrics:

```go
client, err := httpclient.New(httpclient.Options{
	BaseURL: "https://api.example.com",
	Timeout: 10 * time.Second,
	Logger:  logger,
})
if err != nil {
	return fmt.Errorf("create catalog client: %w", err)
}

var result struct {
	Name string `json:"name"`
}
_, err = client.Request(ctx, "catalog.get").
	SetResult(&result).
	Get("/items/42")
if err != nil {
	return fmt.Errorf("get catalog item: %w", err)
}
```

Add these imports:

```go
import (
	"fmt"
	"time"

	"github.com/auron-labs/mygo-framework/httpclient"
)
```

The default timeout is 30 seconds and retries are off. Enable bounded retries
only with `RetryOptions`. Reads may retry; writes remain single-attempt unless
`Writes` is explicitly true. Do not log authorization headers or response
bodies.

## Submit a form from the frontend

`src/framework/request.ts` and `src/framework/form.ts` wrap generated MyGo
methods for the page. `runRequest` turns a call into `{ ok: true, data }` or
`{ ok: false, problem }`, and `createForm` tracks values, submission, and
errors:

```ts
import { createForm } from "./framework/form";
import { runRequest } from "./framework/request";
import { Notes, type CreateNoteInput, type CreateNoteOutput } from "./mygo";

const noteForm = createForm<CreateNoteInput, CreateNoteOutput>({
  initialValues: { body: "" },
  submit: (values) => runRequest(() => Notes.create({ body: values.body })),
});

noteForm.subscribe((state) => {
  // Render state.values, state.submitting, state.fieldErrors and state.formError.
});
input.addEventListener("input", () => noteForm.setField("body", input.value));
```

Backend problems keep their code, message, and field errors. Field errors for
fields the form does not have, and a problem with no field errors, become
`formError`. A failure to reach the backend becomes an `unavailable` problem
with a generic message. While a submission is pending, `submit()` returns the
same promise to every caller. After `reset()` or `dispose()`, a late result no
longer changes the form state, but the Go request still runs. See `src/main.ts`
for the complete Notes form.

## Define and apply a migration

Create the next stable five-digit migration:

```sh
framework make migration add_note_pinned
```

Edit the generated SQL file:

```sql
-- +goose Up
ALTER TABLE notes ADD COLUMN pinned INTEGER NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE notes DROP COLUMN pinned;
```

Desktop startup applies pending forward migrations before publishing readiness.
For an offline check, close the desktop process and run:

```sh
go run ./cmd/appctl db status
go run ./cmd/appctl db up
go run ./cmd/appctl db status
```

Use `--data-directory PATH` before `db` for a portable test profile. `db status`
does not create a missing database or apply a migration. Never rewrite an
applied migration; add the next migration instead.

## Add a focused scaffold or appctl command

`framework make service|request|model NAME` writes one hand-written file in the
matching `app/` directory, and `make migration` works as shown above.
`framework make command NAME` writes a Cobra command constructor in
`app/commands`; register it on the application-owned root in
`internal/appctl/root.go`:

```go
root.AddCommand(commands.NewBackupCommand(func(ctx context.Context) error {
	return runBackup(ctx)
}))
```

Command scaffolds are not wired by `framework generate`; registration stays
explicit and offline commands still expect the desktop process to be closed.

## Expose a service method as an MCP tool

MCP exposure is opt-in. The generated Bookmark service already has the required
method shape: `context.Context`, one named request struct, one named output
struct, and `error`. Add the MCP marker below its existing command marker:

```go
// Create validates and stores one bookmark.
// framework:command bookmark.create
// framework:mcp bookmarks_create readonly=false destructive=false
```

Regenerate both adapter layers:

```sh
framework generate
bun run generate
framework generate --check
```

Start a development launch with an explicit loopback port and one-time
credential disclosure:

```sh
MYGO_MCP_ENABLED=true \
MYGO_MCP_PORT=48123 \
MYGO_MCP_DISCLOSE=true \
framework dev
```

The process prints the Streamable HTTP endpoint and its per-launch bearer token.
Configure a standard MCP client with both values. Omit `MYGO_MCP_DISCLOSE` in
normal launches so credentials do not enter routine output. The endpoint binds
only to `127.0.0.1`, rejects invalid Host, origin, and bearer credentials, and
shares the desktop service and database.

## Runtime defaults

- **Database.** GORM, Goose, and the settings store share one SQLite
  connection, so queries and transactions run one at a time; keep transactions
  short. The connection waits up to 5 seconds for a lock, enforces foreign
  keys, and uses WAL journaling. Startup fails if SQLite cannot enable WAL.
- **Migrations.** Before pending migrations run on a database that already has
  data, a copy is written to `backups/` beside it, named with the time and the
  schema version it holds; the newest five are kept. A database that a newer
  version of the application has migrated is not opened: startup fails with
  `database.ErrSchemaNewer` instead of running against tables it does not know.
  `appctl db backup FILE` exports the data and `appctl db restore FILE` replaces
  it, keeping a copy of what it replaced in `backups/`.
- **Logs.** Development builds log to stderr. Packaged builds also write
  `app.log` to `~/Library/Logs/<app>` on macOS and to the `logs` folder of the
  data directory elsewhere. The file rotates at 10 MB and keeps three compressed
  backups for up to 28 days; change this with `logging.Options`. Attributes
  named authorization, cookie, password, payload, request or response body,
  secret, settings, or token are redacted, as is everything in a group with one
  of those names.
- **Quit.** Quitting cancels the application context, then gives runtime
  shutdown up to 5 seconds before the process exits. Long-running work should
  return when that context is canceled.

## Develop, build, and test

Use the project-pinned MyGo CLI through the framework wrapper:

```sh
framework doctor
framework dev
framework build
```

Run the repository checks from its root:

```sh
go test ./...
go test -race ./...
CGO_ENABLED=0 go test ./...
go vet ./...
golangci-lint run ./...
bun run test:web
bun run typecheck
bun run build:web
framework generate --check
```

Race testing is a host-toolchain check and may use CGO. Shipping builds and the
separate CGO-free suite must continue to pass with `CGO_ENABLED=0`.
