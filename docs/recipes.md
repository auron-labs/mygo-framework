# Capability recipes

These task guides use the reference application in this repository. Run the Go
commands from the repository root. Import paths therefore start with
`mygo-framework`; replace that prefix with the `module` value from `go.mod` in
another application.

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

## Save a typed setting

Define keys once and register them with one `settings.Values` instance. The
SQLite store borrows the application's existing `database.SQL` pool; it does
not own or close that pool.

For example, add `app/bootstrap/settings.go`:

```go
package bootstrap

import (
	"fmt"

	"mygo-framework/internal/database"
	"mygo-framework/settings"
	settingssqlite "mygo-framework/settings/sqlite"

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

	"mygo-framework/app/profile"
	"mygo-framework/secrets"
	mygosecrets "mygo-framework/secrets/mygo"
)
```

Use `store.Set` to provision the value and `store.Delete` to remove it. The
adapter never falls back to plaintext. Keep the in-memory adapter for tests and
handle an unavailable or locked platform credential store as an operational
error. Do not assume MyGo's OS secret API works from the GUI-independent
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

	"mygo-framework/httpclient"
)
```

The default timeout is 30 seconds and retries are off. Enable bounded retries
only with `RetryOptions`. Reads may retry; writes remain single-attempt unless
`Writes` is explicitly true. Do not log authorization headers or response
bodies.

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

See [foundation.md](foundation.md) for lifecycle ownership, error mapping,
generator behavior, security boundaries, platform evidence, and measured build
and startup costs.
