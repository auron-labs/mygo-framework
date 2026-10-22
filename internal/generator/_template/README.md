# __PROJECT_TITLE__

A MyGo web desktop application created by the MyGo framework CLI.

## Start developing

Requirements: Go 1.27.3, Bun 1.4.2, and Git. The checked-in manifests pin MyGo
and the frontend toolchain. `framework new` initializes the repository so MyGo
can attach supported VCS metadata to its generated Go build.

```sh
bun install
framework dev
```

The initial Notes feature demonstrates an embedded Goose migration, an ordinary
validated Go service, explicit bootstrap registration, a generated desktop
adapter, and the small TypeScript form/request helpers in `src/framework/`.

## Add a feature

```sh
framework make feature Bookmark
bun run generate
```

`make feature` creates the hand-written model, request, service, provider, and
migration, then regenerates framework-owned Go files. Run `bun run generate`
afterward to refresh MyGo's TypeScript client. Manual changes to exposed service
signatures or `framework:` annotations require `framework generate` before the
MyGo generation step. Method-body-only edits use MyGo's normal rebuild flow.

Use `framework generate --check` in CI. Do not edit files carrying the standard
generated-code header.
