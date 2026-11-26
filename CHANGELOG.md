# Changelog

## [2.1.0](https://github.com/auron-labs/mygo-framework/compare/v2.0.0...v2.1.0) (2026-11-26)


### Features

* **examples:** mirror native lifecycle integration ([88c60ee](https://github.com/auron-labs/mygo-framework/commit/88c60ee519c02ecab78aaf596115459499ef699e))
* **generator:** add lifecycle support to native starters ([2a151e0](https://github.com/auron-labs/mygo-framework/commit/2a151e0f67d481a340070f751e22eff6429c4669))
* **lifecycle:** add desktop background lifecycle helper ([c6eae52](https://github.com/auron-labs/mygo-framework/commit/c6eae529f67f4ace2a36a136421406124a704bc5))
* **onevra:** add an assistant sidebar to the chat components ([d3e65c7](https://github.com/auron-labs/mygo-framework/commit/d3e65c77696d290f7f73099455d420f140e80e44))
* **onevra:** add ShellOptions.LeftToggleTrailing to the sidebar shell ([61716b1](https://github.com/auron-labs/mygo-framework/commit/61716b1261e117d81fb0ab3da5d8ebec17d98662))
* use platform log directories and add a success log level ([87e3441](https://github.com/auron-labs/mygo-framework/commit/87e344123aa958a34bb2c4e49e35dd2a9f181c5e))


### Bug Fixes

* **datadir:** read XDG_STATE_HOME with Unix path rules ([adfe328](https://github.com/auron-labs/mygo-framework/commit/adfe32818fcb68177a7a7eb1558db39d99792792))

## [2.0.0](https://github.com/auron-labs/mygo-framework/compare/v1.0.1...v2.0.0) (2026-11-19)


### ⚠ BREAKING CHANGES

* Update framework imports, Go tool directives, and the Sentry linker symbol to use github.com/auron-labs/mygo-framework/v2.

### Features

* **datadir:** keep app data in the app sandbox on phones ([7bc9b8b](https://github.com/auron-labs/mygo-framework/commit/7bc9b8bc8859842d3172d46e0290d01537935d94))
* **generator:** pin framework CLI as a Go tool in generated projects ([df831b4](https://github.com/auron-labs/mygo-framework/commit/df831b4a6bcc88321b5223a308dbe1457bcbc961))
* **httpclient:** add conditional requests, response hook, and rate-limit details on errors ([56f9bce](https://github.com/auron-labs/mygo-framework/commit/56f9bced90d39dce52c4f4e8260c0e2a8d0b1041)), closes [#15](https://github.com/auron-labs/mygo-framework/issues/15)
* **httpclient:** add resumable file download with progress and SHA-256 verification ([ef89188](https://github.com/auron-labs/mygo-framework/commit/ef89188646034e174f7ff923cfd75686df70afe4))
* **jobs:** add named lanes with concurrency limits, Queued status and List ([7553ce2](https://github.com/auron-labs/mygo-framework/commit/7553ce2eaa75395a5d4dfa93680b44cc52dffb1c))
* **jobs:** add periodic task scheduler with persisted last run and catch-up ([88fb5c7](https://github.com/auron-labs/mygo-framework/commit/88fb5c7ed8f8d64c9292557bcc44f6ec8e78e13f))
* migrate the Go module to v2 ([#34](https://github.com/auron-labs/mygo-framework/issues/34)) ([eab544a](https://github.com/auron-labs/mygo-framework/commit/eab544a5cfb43e7d072946d3a39c6f1998b2d4b9))
* **mobile:** add phone settings to the starters and framework devices and publish ([1a21b1e](https://github.com/auron-labs/mygo-framework/commit/1a21b1ef9e82faa2fb2f3cc3b292a72ec132bbbd))
* **mobile:** fit the web starter's page to phone screens and touch ([4b13f99](https://github.com/auron-labs/mygo-framework/commit/4b13f99a849e61a73bf762fb8ec680be4d5debe5))
* **mobile:** reach starter actions on phones without menus or desktop dialogs ([31049e5](https://github.com/auron-labs/mygo-framework/commit/31049e565fe6be4b1e4633041f2ff46200170439))
* **mobile:** save state on suspend and restore it after the system ends the app ([37fd259](https://github.com/auron-labs/mygo-framework/commit/37fd2591021dee8bdb710e25937f589a7735923e))
* **mygomcp:** drive touch input and phone viewports from the headless harness ([268ac0a](https://github.com/auron-labs/mygo-framework/commit/268ac0a103c0b5bd1fcd7685aaea063cc29191c7))
* **mygomcp:** import mygo-mcp as a framework package ([b051235](https://github.com/auron-labs/mygo-framework/commit/b051235c39dddb5eb8d4c6b5d850f1521fa062b4))
* **mygomcp:** import mygo-mcp as a framework package ([e788e33](https://github.com/auron-labs/mygo-framework/commit/e788e337bd83d6b347dbb074ba30bd4a1b86dcba))
* **native:** add CI workflow, mise.toml and golangci config to the starter ([d1e8e00](https://github.com/auron-labs/mygo-framework/commit/d1e8e00d237dafc60445b5f7ab43ae7250cab0c4))
* **native:** add sidebar-shell starter variant with page routing and icon rail ([#22](https://github.com/auron-labs/mygo-framework/issues/22)) ([aacd35a](https://github.com/auron-labs/mygo-framework/commit/aacd35a2f14983671d9ec68969e479767b2abe86))
* **native:** return to the same sidebar page after framework dev restarts ([f22ad66](https://github.com/auron-labs/mygo-framework/commit/f22ad66cbc44fcdce007c0abee92c92d55a9edf3))
* **native:** run the native starters on phones ([97b1b0e](https://github.com/auron-labs/mygo-framework/commit/97b1b0eedd7df7948dd327165a17c60cf8cd80a3))
* **onevra:** maintain native UI packages in the framework ([10233e0](https://github.com/auron-labs/mygo-framework/commit/10233e05bc445d39e07d5c52e7576953dfc72b69))
* **onevra:** make components and shells work by touch on phones ([8884cb8](https://github.com/auron-labs/mygo-framework/commit/8884cb8e81639cdfa9327923a517a95a6d5f1270))
* **onevra:** scroll menus longer than the window ([1bc71d6](https://github.com/auron-labs/mygo-framework/commit/1bc71d6f1f249317ac18ee6f56cd22453692f25c))


### Bug Fixes

* **ci:** remove unused scheduler lint suppression ([5299fcc](https://github.com/auron-labs/mygo-framework/commit/5299fcc49e49d24bcb33873bb9f53ec4d70abfcd))
* **ci:** remove unused scheduler lint suppression ([f5a5c6a](https://github.com/auron-labs/mygo-framework/commit/f5a5c6a4842c833cfdc106559c76b48d87fa4a69))
* **generator:** make scaffolded native feature test pass golangci-lint ([3e547ba](https://github.com/auron-labs/mygo-framework/commit/3e547bab8d180ab71d7d37d450169b6f0dacc60f))
* **jobs:** deliver events before Shutdown returns, cancel queued jobs on parent cancel ([c0df61c](https://github.com/auron-labs/mygo-framework/commit/c0df61c43067c6fa8843d1b00452c0c6af66a9fb))
* **jobs:** publish job events in order, add ErrEmptyLane ([14eb363](https://github.com/auron-labs/mygo-framework/commit/14eb363b2d66b8d2bfb431c32f1f965eaf7b20dd))
* stabilize Windows UI and path tests ([3feddd8](https://github.com/auron-labs/mygo-framework/commit/3feddd84c76fcd6a96fc047a77c9f9297f16cada))
* stabilize Windows UI and path tests ([285fe80](https://github.com/auron-labs/mygo-framework/commit/285fe80c87d15e92a2a13adfb58628bf0afb07e1))

## [1.0.1](https://github.com/auron-labs/mygo-framework/compare/v1.0.0...v1.0.1) (2026-11-16)


### Bug Fixes

* **deps:** regenerate MyGo 0.9.2 client and keep CLI go.sum entries ([1d58a77](https://github.com/auron-labs/mygo-framework/commit/1d58a777c47f44ac9fe7be614f2e82d62f4d043d))

## [1.0.0](https://github.com/auron-labs/mygo-framework/compare/v0.3.0...v1.0.0) (2026-11-11)


### ⚠ BREAKING CHANGES

* the module path changed from mygo-framework, and generated projects no longer contain the framework packages.

### Features

* add single instance, menu bar, notifications and deep links to the starters ([54e1c72](https://github.com/auron-labs/mygo-framework/commit/54e1c72b20191f7002dd31343217222e7a611325))
* add support bundles, an error-report switch and release workflows ([a5564a4](https://github.com/auron-labs/mygo-framework/commit/a5564a461a5fa4678f3a76f112223db103aa7de8))
* **database:** snapshot before migrating and guard against newer schemas ([35a6c95](https://github.com/auron-labs/mygo-framework/commit/35a6c9515b4cc50cc178c5fdf8c3e557ba98c504))
* **native:** keep preferences in a cell with autosave ([7aa3399](https://github.com/auron-labs/mygo-framework/commit/7aa339973115c06c66bb868af4f88ed42ae846c8))
* publish the framework as an importable Go module ([a8bcdf5](https://github.com/auron-labs/mygo-framework/commit/a8bcdf5ed473c1d22beaa0c1562d3c695c7b3353))
* **state:** add reactive cells, derived values and autosave ([e5137cb](https://github.com/auron-labs/mygo-framework/commit/e5137cb2b8c699d6b1e7a0870c4417fc7c854506))

## [0.3.0](https://github.com/auron-labs/mygo-framework/compare/v0.2.0...v0.3.0) (2026-11-10)


### Features

* add cancellable background job service ([bea349f](https://github.com/auron-labs/mygo-framework/commit/bea349f752f303b5a90e5f532f846c637b5a4542))
* add linked time-series chart for native views ([a4b806e](https://github.com/auron-labs/mygo-framework/commit/a4b806e286504206aef1cf2523f4236a800f9eb9))
* add PDF report package for native apps ([3321417](https://github.com/auron-labs/mygo-framework/commit/3321417ceb27ecb33cc4b84b10e0a8618089fd83))
* add per-profile database helper ([6a10b74](https://github.com/auron-labs/mygo-framework/commit/6a10b741443e64195478df8363311c3ebfcbd0e3))
* **cli:** scaffold features in native projects ([f332613](https://github.com/auron-labs/mygo-framework/commit/f332613a8c398b066726f4d6d7bd90343af906db))
* **native:** add opt-in signed updates ([2c7f67b](https://github.com/auron-labs/mygo-framework/commit/2c7f67b362f47214132c71f78b18f18859db0a95))
* **native:** add SQLite database and migrations to the starter ([13e62b9](https://github.com/auron-labs/mygo-framework/commit/13e62b96ef4ab6b39dd76bb75637829fc2f4fb1f))
* **native:** add typed application settings ([a8e8426](https://github.com/auron-labs/mygo-framework/commit/a8e8426797b74930ced748e63ea6256f6ca5a34d))
* **native:** import a folder by drop or folder picker ([042d8d8](https://github.com/auron-labs/mygo-framework/commit/042d8d88efdb1049b655c99d613c0c4cb47dd645))
* **native:** make starter strings translatable ([7744be6](https://github.com/auron-labs/mygo-framework/commit/7744be615d17c5e9aa2725dfbf7b4ae26d09aca0))
* report errors to Sentry ([ec9e34f](https://github.com/auron-labs/mygo-framework/commit/ec9e34fb8671c6c0af22efa199011243e8b90a5a))


### Bug Fixes

* **database:** open SQLite databases on Windows ([aad7913](https://github.com/auron-labs/mygo-framework/commit/aad79132232eebfbfd4b5d8a3172dc8563c5a657))
* **database:** resolve relative paths before building the file URI ([5d704ec](https://github.com/auron-labs/mygo-framework/commit/5d704ecefdc19958ba67cfe6a5c68f5e532b80b9))

## [0.2.0](https://github.com/auron-labs/mygo-framework/compare/v0.1.0...v0.2.0) (2026-10-28)


### Features

* add mygo-mcp ([503e55d](https://github.com/auron-labs/mygo-framework/commit/503e55d34e89fd49b3df2d074a53fd612423fda9))

## [0.1.0](https://github.com/auron-labs/mygo-framework/compare/v0.0.1...v0.1.0) (2026-10-28)


### Features

* add native project starter and upgrade MyGo to v0.7.1 ([34c72f8](https://github.com/auron-labs/mygo-framework/commit/34c72f86b866f856f011f236810a1154d7420c2e))
