# Project guidance

- Keep transport-independent behavior in `app/services` with normal constructor
  arguments. Validate typed inputs at public service entry points.
- Register provider dependencies explicitly in `app/bootstrap`; one owner closes
  each resource. Goose SQL files in `database/migrations` are schema authority.
- Add a vertical feature with `framework make feature NAME`. Use focused
  `framework make service|request|model|migration|command NAME` only when the
  full vertical scaffold is not appropriate.
- Run `framework generate` after changing an exposed method signature or its
  `framework:` annotations, then run `bun run generate` for MyGo bindings.
- Never hand-edit `app/bootstrap/framework_generated.go`,
  `app/desktop/framework_generated.go`, `app/mcp/framework_generated.go`, or
  `framework_bindings_generated.go`.
- Keep MCP exposure opt-in. A desktop marker does not expose a method as an MCP
  tool.
