# Working in Siftpane

- Read [development standards](docs/DEVELOPMENT.md), [architecture](docs/ARCHITECTURE.md), and [UI/UX guidelines](docs/DESIGN.md) before changing the relevant code.
- Keep README.md for human installation and everyday use. Put engineering, setup details, and design decisions in docs/ and link to them here; do not duplicate manuals in this file.
- Use idiomatic Go with small, cohesive packages. Keep UI state on the Bubble Tea event loop, external calls in commands, and rendering free of I/O and mutations.
- Run `make fmt` after Go edits and `make check` before finishing. The pinned tooling checks production code and tests; fix findings rather than weakening lint/format rules. Explain any narrow suppression.
- Preserve explicit conversation selection, workspace isolation, cancellation, bounded polling/cache, draft retention, and pending/failed/retry behavior. Never silently discard composed text or automatically retry a send.
- Use documented Slack APIs and the minimum implemented scopes. Verify current official docs before changing authentication, API parameters, subscriptions, or limits. No cookies, desktop sessions, or undocumented endpoints.
- Never commit credentials or log tokens/message bodies. Keep secrets in the OS credential store or an explicit environment override. Demo and automated tests must remain offline and credential-free.
- For UI changes, inspect 100×30 and 60×20 layouts, plus relevant states. Preserve bracketed amber focus titles, independent border colors, the solid composer, shared edges, and space for messages. Refresh affected review captures.
- Record validation and material limitations honestly. Live Slack verification is still outstanding; fake-service or snapshot success is not live verification. See [verification](docs/VERIFICATION.md) and [backlog](docs/BACKLOG.md).
- Keep changes focused. Do not commit, push, publish, or release unless requested.
