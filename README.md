# MyGo application framework

This repository is a source-first framework and reference application for
building local-first [MyGo](https://mygo.egoist.dev/) desktop apps in Go. It
adds a predictable application layout, SQLite and GORM persistence, Goose
migrations, validation, explicit dependency injection, structured logging,
typed settings, OS secrets, outbound HTTP helpers, form state, generators, and
optional MCP tools without replacing MyGo's runtime or IPC.

The included Notes app is the working reference. It persists notes in one
SQLite database, uses the same service from the desktop UI and MCP, and shuts
down its listener and storage in dependency order.

## Requirements

- Go 1.27.3 or newer
- Bun 1.4.2 (web projects only)
- Git
- The native libraries required by MyGo on your operating system

The manifests pin MyGo 0.7.1 and the frontend toolchain. Run
`framework doctor` inside a generated project to check the local MyGo build
requirements.

## Create and run an app

Install the current CLI from a checkout. Ensure Go's binary directory is on
your `PATH`.

```sh
cd /path/to/mygo-framework
go install ./cmd/framework
framework --version
```

Create the app from the directory that should contain it:

```sh
cd /path/to/projects
framework new my-notes
cd my-notes
bun install --frozen-lockfile
framework doctor
framework dev
```

The last command opens the Notes desktop app with MyGo's normal live-reload
workflow. Add a note, quit the app, and start it again to see the persisted
note.

## Native Go UI

Create a native starter with Onevra components and persisted preferences:

```sh
framework new my-native-app --native
cd my-native-app
go mod download
framework dev
```

Native projects use the pinned Go MyGo tool for development, diagnostics, and
packaging. See the [native guide](docs/native.md) for persistence, testing, and
using framework packages in another application.

## Add the first feature

From the generated project root, create a complete vertical slice:

```sh
framework make feature Bookmark
bun run generate
```

The first command creates the model, request, service, provider, and Goose
migration. It also refreshes framework-owned adapters. The second command
refreshes MyGo's TypeScript client in `src/mygo.ts`.

Edit the generated service method and frontend for the feature, then verify the
owned files and builds:

```sh
framework generate --check
go test ./...
bun run test:web
framework build
```

Never edit files with a generated-code header. After changing an exposed method
signature or a `framework:` annotation, run `framework generate` and then
`bun run generate`. Ordinary method-body changes use MyGo's normal rebuild
flow.

## Common commands

| Command | Purpose |
| --- | --- |
| `framework dev` | Start MyGo development; generate adapters first for web projects |
| `framework build` | Build with MyGo; generate adapters first for web projects |
| `framework doctor` | Run the pinned MyGo doctor; check adapters first for web projects |
| `framework generate` | Refresh framework-owned Go adapters and registrations |
| `framework generate --check` | Fail if committed generated files are stale |
| `framework make feature NAME` | Create a complete feature and regenerate adapters |
| `framework make migration NAME` | Create the next numbered Goose migration |
| `go run ./cmd/appctl db status` | Inspect migrations while the desktop app is closed |
| `go run ./cmd/appctl db up` | Apply pending migrations while the desktop app is closed |

MyGo flags pass through `framework dev` and `framework build`. Use each
command's `--help` output for the pinned MyGo options.

## Learn the framework

- [Capability recipes](docs/recipes.md) show settings, secrets, HTTP,
  migrations, MCP exposure, and the exact verification commands.
- [Capability matrix](docs/capabilities.md) maps each requested feature to its
  implementation, direct evidence, and platform limits.
- [Foundation and architecture notes](docs/foundation.md) record ownership,
  lifecycle, generator, security, platform, and performance behavior.
- [Implementation plan](PLAN.md) is the authoritative scope and design record.
- [Execution checklist](TODO.md) records completed items and their verification.

The framework intentionally has no Goravel dependency, mandatory UI server,
global service facade, automatic database repair, plaintext secret fallback, or
second file watcher.
