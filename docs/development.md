# Development and releases

## Set up a checkout

Install [mise](https://mise.jdx.dev/getting-started.html) and Git, then run from
the repository root:

```sh
mise trust
mise install
mise run install
mise exec -- hk install --mise
```

`mise.toml` pins Go, Bun, Node, hk, and the tools used by the existing hooks.
TypeScript, Vite, and MyGo CLI stay in the Bun dependency graph. Node runs the
package CLI launchers; Bun owns dependency installation and frontend scripts.
No shell configuration changes are needed to run mise tasks.

The pre-commit hook applies the configured Go formatting, lint fixes, module
tidying, and repository hygiene checks to matching staged files. It stashes
unstaged changes to protect partial commits. The commit-msg hook requires a
conventional commit header, such as `feat: add bookmarks` or `fix: close storage`.
Tests run separately from pre-commit.

## Check and build changes

| Command | Purpose |
| --- | --- |
| `mise run test` | Run Go tests and Bun frontend tests |
| `mise run typecheck` | Check frontend types |
| `mise run check` | Run the configured hk checks across the repository |
| `mise run fix` | Apply configured fixes across the repository |
| `mise run check:generated` | Detect stale framework-owned adapters |
| `mise run build:web` | Typecheck and build the frontend |
| `mise run build` | Package the reference desktop app through MyGo |

Desktop development and packaging also need the native libraries required by
MyGo on your operating system. Run `mise exec -- go run ./cmd/framework doctor`
to inspect those requirements. CI runs checks, generated-file verification,
tests, and the frontend build on Linux; it does not launch or package a desktop
window.

For the opt-in fresh-project acceptance flow, run:

```sh
mise exec -- env MYGO_GENERATOR_ACCEPTANCE=1 go test -run '^TestGeneratedProjectAcceptance$' -v ./internal/generator
```

This creates a disposable project, installs its dependencies, generates a
feature and both adapter layers, then tests and builds it. It requires access
to the pinned Go and Bun dependencies.

## Prepare a release

[Release Please](https://github.com/googleapis/release-please-action) owns the
framework changelog, `vX.Y.Z` tags, and GitHub Releases. The manifest starts at
the existing `v0.0.1` tag; the bootstrap commit limits the first automated
changelog to changes after that tag. The private Notes frontend and generated
starter metadata do not represent separately published packages.

After these files are committed and pushed to `main`, the release workflow uses
the built-in `GITHUB_TOKEN`. In the repository's Actions settings, allow GitHub
Actions to create and approve pull requests. Organization policy must also
permit the workflow's write permissions.

Use conventional commits for releasable changes: `feat:`, `fix:`, or a breaking
change such as `feat!: change the generator API`. Squash merging keeps the
commit history and changelog clear. Release Please opens or updates a release
pull request when releasable commits reach `main`. Review its version and
changelog, then merge it to create the tag and GitHub Release. The workflow can
also be started manually from the Actions tab.

The default token does not trigger other workflows for bot-created release
pull requests. If CI is required before merging one, run CI manually with
`workflow_dispatch` on the release branch. See the action's
[token behavior](https://github.com/googleapis/release-please-action#github-credentials).

Releases contain source archives. Install the framework CLI from a checkout
using the README instructions. No registry publishing, binary upload, desktop
signing, or installer publication is configured.
