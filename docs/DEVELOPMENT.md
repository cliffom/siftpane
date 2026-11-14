# Development

Siftpane follows [Effective Go](https://go.dev/doc/effective_go) and the [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments). Repository-specific decisions below make those conventions reproducible. Formatting and linting are required checks, including in tests.

## Daily workflow

Use Go 1.26.7 or newer. Run commands from the repository root:

```sh
make fmt       # Apply gofumpt and goimports
make lint      # Verify the configuration and run the pinned linters
make test      # Race-enabled tests; no Slack credentials
make check     # Formatting, lint, vet, race tests, build, module hygiene
make demo      # Offline interactive review
```

`make check` checks formatting without rewriting files. It fails on formatting drift, lint findings, test/build errors, or untidy modules. `make mod-check` also verifies downloaded module checksums. The first invocation downloads and builds the pinned tooling; subsequent runs use Go's cache.

[Golangci-lint v2.15.0](https://golangci-lint.run/docs/) is pinned as a Go tool in `tools/go.mod` and `tools/go.sum`. The application module stays separate from the linter's dependencies. The Makefile invokes it with:

```sh
go tool -modfile=tools/go.mod golangci-lint run ./...
```

[.golangci.yml](../.golangci.yml) is the source of truth. The enabled checks cover errors and HTTP body cleanup, vet/static analysis, unused code, cognitive complexity, idiomatic exported APIs and names, import hygiene, and formatting. Production code and tests run through the same gate; there are no repository test or legacy-code exclusions. Individual analyzer limitations are documented below. Cognitive complexity is limited to 30 per function. Split a function around meaningful operations rather than weakening the threshold.

Use `//nolint:<specific-linter> // reason` only for a justified false positive that cannot be expressed clearly in code. Fix findings first; do not add general suppressions or exclusions to make a failing check pass. Deliberately ignored cleanup errors use an explicit assignment with a nearby reason. Preserve the primary write/sync error when cleanup also fails.

## Go conventions

- Use gofumpt and goimports, with standard library, third-party, and `github.com/cliffom/siftpane` imports in separate groups. Do not hand-align code or maintain a second formatting configuration in an editor.
- Use clear package names, Go initialisms (`ID`, `URL`, `HTTP`), `err` for error variables, short conventional receivers, and descriptive names for state with a longer lifetime. Use multiline literals for substantial model/configuration values.
- Keep exported identifiers documented, including internal package APIs. Comments explain contracts, ownership, failure behavior, and non-obvious decisions. Do not repeat the implementation line by line.
- Return errors to the boundary that can present them. Add useful context with `%w`; use `errors.Is`/`errors.As` for classification. Avoid matching an error's display string. Never wrap an HTTP error in a way that exposes credentials or message bodies.
- Pass `context.Context` first to service methods. Honor cancellation and bound network waits. The Bubble Tea model owns its application-lifetime context and selected-conversation cancellation contexts; HTTP clients do not store request contexts.
- Only the event loop mutates UI state. Commands capture immutable request data and return typed messages. Rendering performs no I/O and makes no state changes. Shared service caches and rate-limit clocks are synchronized.
- Use small consumer-facing interfaces and concrete types internally. Keep changes within existing packages unless a new responsibility justifies a boundary. Avoid generic `utils`, `helpers`, `common`, or one-type-per-package structures.
- Keep dependency versions in the modules and checksums in their sum files. Update application and tooling dependencies separately and deliberately; do not use `@latest` in committed automation. After updates, run `make check` and inspect module diffs.

## Values, states, and settings

Name behavioral values before using them in handlers or adapters. Keep definitions with the smallest owning responsibility; there is no application-wide constants/settings package. Preserve existing wire values, preference keys, timings, capacities, keybindings, and appearance when doing cleanup. A constant is not automatically a new configurable option.

| Value category | Owner and convention |
| --- | --- |
| Domain states | `core.ConversationKind` and `core.DeliveryState` with typed constants in `internal/core/values.go`. Use these constants across adapters, fixtures, and UI; convert to strings only for text/serialization boundaries. Delivery's zero value means idle. |
| UI states | Private `focusPane` and `pickerMode` types in `internal/ui/modes.go`; `pickerClosed` is the zero value. Snapshot CLI identifiers have a separate `SnapshotMode` type. Never use snapshot identifiers as navigation state. |
| Operational bounds | `internal/ui/limits.go` owns session/cache/input limits and polling. Slack transport policy lives in `internal/slack/client.go`; CLI credential identifiers and authentication timeout live in `cmd/siftpane`. Use `time.Duration` expressions with explicit units and names/comments indicating messages, entries, bytes, runes, or ticks. |
| Presentation | `internal/ui/layout.go` owns viewport defaults, breakpoints, and row/column reservations. Compose related dimensions from those reservations. Named styles in `view.go` own the palette. `keys.go` owns Bubble Tea key identifiers and derives display labels used by help/status/composer text. |
| Runtime settings and defaults | Flags in the CLI and workspace preferences in `store` are runtime inputs. Defaults remain in the owning package; share viewport defaults with the CLI. Do not persist immutable policy or copy credentials into preferences. |
| Protocol identifiers | Keep Web API methods and shared parameter identifiers in the Slack adapter; use standard-library HTTP method/status constants. JSON struct tags must remain literal tags. One-use parameter keys, regular expressions, formatting verbs, UUID bit masks/slices, and timestamp layouts can remain local where the protocol meaning is clear and documented. |
| Fixture and ordinary text | Demo names, seed spacing, and thread placement belong to `internal/demo/fixtures.go`; offline UI setup/snapshot text remains explicit fixture data in the UI review harness. Keep ordinary messages/errors/help copy at the rendering or failure site. Share repeated fixture identities within their package/test file, not through production domain constants. |

Use names for policy, not aliases such as `two` or `defaultValue`. Small structural arithmetic (zero/one, indices, a shared edge) can stay inline when its meaning is immediate. Name larger layout reservations rather than reusing an unrelated constant that happens to have the same value. Derive limit messages, help labels, and calculations from their source definitions. Tests for the current policy should use those definitions; compatibility tests deliberately assert independent old wire values or fixed expectations so a mistaken change cannot update both sides of an assertion.

The pinned [golangci-lint checks](https://golangci-lint.run/docs/linters/configuration/) enforce part of this policy through `make lint` and therefore `make check`/CI:

- `goconst` reports strings of at least three characters repeated three times within a package, including calls and test files. Resolve findings with meaningful ownership; do not move fixture text into a global constants package.
- `mnd` checks numeric arguments, conditions, and returns. Its upstream implementation ignores 0/1, tests, `time.Date`, and numeric parsing/formatting calls. No repository file exclusions or blanket suppressions are added. Assignment/operation-wide checks are deliberately not enabled: protocol bit operations, slice offsets, and fixture arithmetic need semantic review rather than arbitrary number aliases.
- Existing `durationcheck` checks duration arithmetic; type checking keeps runtime strings from being passed as domain states without conversion. Go still accepts untyped string literals for a defined string type, so reviewers must reject ad hoc state literals, unjustified conversions, unnamed one-off policy, and duplicated sources of truth. Linters cannot determine whether a value belongs to protocol, fixture, presentation, defaults, or runtime configuration.

To verify enforcement after a tooling change, introduce a temporary package with a string repeated three times and a condition comparing against an unnamed number, run the configured lint gate, confirm both `goconst` and `mnd` diagnostics, and remove the probe before running `make check`. Do not weaken thresholds or exclude files to conceal violations.

## Organization

See [architecture](ARCHITECTURE.md) for package responsibilities. Files within `internal/ui` separate model lifecycle, state, asynchronous commands, input/picker handling, history/delivery updates, and rendering. Extend the relevant file or extract a named operation; do not grow another monolithic event handler. The model remains one event-loop-owned state object rather than many independently mutable controllers.

Keep tests next to their package, named `*_test.go`. Prefer table-driven cases where the scenarios share a contract. Use `t.Helper` for assertion/setup helpers and `t.Cleanup` for test resources. Check fixture and setup errors. Network handler helpers report errors with `t.Error` and return; they do not call `t.Fatal` from server goroutines.

## Validation

Tests use in-memory services and `httptest`, never real tokens or Slack calls. Cover behavior that can lose text, misattribute activity, cross workspace boundaries, defeat selection scoping, bypass backoff, or break pagination. Keep timing tests bounded and allow for scheduler variability. Run `go test -race ./...` after concurrency changes.

For visible UI changes, review both a desktop and a small terminal:

```sh
go run ./cmd/siftpane --snapshot messages --width 100 --height 30
go run ./cmd/siftpane --snapshot compose --width 60 --height 20
go run ./cmd/siftpane --snapshot picker --width 80 --height 24
go run ./cmd/siftpane --snapshot search --width 100 --height 30
go run ./cmd/siftpane --snapshot search --width 60 --height 20
go run ./cmd/siftpane --snapshot thread --width 100 --height 30
go run ./cmd/siftpane --demo
```

Check borders/corners, display-cell width, wrapping, the first visible author, amber title isolation, drafts, pending/failed/retry states, and the short help row. Follow [design guidelines](DESIGN.md). Refresh affected captures in `docs/review/` when the appearance changes; don't update a capture to conceal a regression.

CI runs `make check` on Linux with the minimum Go version and the current stable Go version. Its action revisions are pinned. A local success does not establish that hosted CI or real Slack verification passed; report those separately in [the verification record](VERIFICATION.md).

## Documentation ownership

- `README.md`: human introduction, install/run, essential shortcuts, expectations, and links.
- `AGENTS.md`: short working instructions and links, not a duplicate manual.
- `docs/DEVELOPMENT.md`: contributor standards, tooling, and validation.
- `docs/ARCHITECTURE.md`: boundaries, ownership, and implementation decisions.
- `docs/DESIGN.md`: accepted interaction and visual rules.
- `docs/SLACK.md`, `USAGE.md`, `CONFIGURATION.md`, `TERMINALS.md`: detailed user/setup references.
- `docs/VERIFICATION.md`: evidence and outstanding checks; `docs/BACKLOG.md`: future work.

When changing behavior, update the authoritative document and any affected README summary. Keep facts consistent without copying the entire explanation between files.
