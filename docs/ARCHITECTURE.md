# Architecture

The program has one Bubble Tea event loop. State changes happen in `Update`; views format state and do not perform I/O. `tea.Cmd` closures perform Slack calls and return typed result messages. Configuration writes are small synchronous local-file operations; authentication runs before starting the TUI. No HTTP calls execute inside `View` or a key handler.

| Package | Responsibility |
| --- | --- |
| `cmd/siftpane` | Flags, credential-store/environment access, identity validation, lifecycle |
| `internal/core` | Service interface, conversation/message/page types, filtering, bounded merge, delivery state |
| `internal/slack` | Documented Web API transport, per-method pacing/backoff, lazy author lookup, safe textual Slack content |
| `internal/demo` | In-memory implementation of the identical service interface, pagination, threads, deterministic send failure |
| `internal/store` | Workspace/conversation preferences, private atomic replacement, no credentials or bodies |
| `internal/ui` | Application state, commands, navigation, drafts/outbox, layout and rendering |

`core.Service` provides identity, direct conversation metadata, paginated discovery, paginated history/replies, and posting. Slack authentication uses a previously authorized user token from the internal app's install flow. `conversations.info` restores included IDs without scanning the directory. The directory is a user-paged metadata catalog; history is fetched only for included conversations. Demo mode creates a memory-only store and never opens a credential store or network transport.

Every included conversation gets a cancellation context and generation. Removal cancels outstanding reads, invalidates the generation, clears cached content and local notifications, and saves inclusion preferences. Late responses from a removed generation are ignored even after re-inclusion. A send already in progress is allowed to finish because cancellation cannot retract a message Slack may have accepted. Its draft/delivery state survives removal and informs the exit warning.

History cache eviction is LRU, limited to 16 timelines with 300 messages each. Old/new pages deduplicate by Slack timestamp. Recent history refresh replaces its observed timestamp range so changed/deleted messages in that window are reconciled. Loading older history retains the older portion of a full cache; newest refresh shifts back toward recent messages. Threads page forward and use an `oldest` watermark after all loaded pages are complete. Incomplete threads require explicit next-page requests. These bounds trade complete historical synchronization for predictable work and memory.

Local-read watermarks are independent of Slack. First observation establishes a baseline; subsequent observed timestamps can set a sidebar dot. Viewing/marking clears it locally. No Slack write occurs for selection or read-state changes. Drafts use conversation/thread keys and remain outside the timeline cache.

The outbox retains an immutable attempt body and stable random client ID. One delivery per target can be pending or failed; duplicate pending submissions are rejected. Acknowledgement clears only the matching draft. A transport-independent delivery-error interface classifies outcomes, including wrapped errors; display strings are never used for retry decisions. Definite failure permits manual retry; ambiguous failure requires an extra explicit retry after warning. Original text stays available for retry or restoration to the composer. Exactly-once delivery across a network failure is not guaranteed. Memory-only drafts are an intentional privacy tradeoff, with an explicit discard warning on exit.

The HTTP layer paces all calls per method with a mutex-protected clock. `429 Retry-After` extends that method's deadline, including manual calls. Waiting honors context cancellation. There are no automatic send retries. Automatic reads are bounded round-robin ticks; expired credentials stop refresh. The UI reports transport errors without logging request/response bodies.

The app name and right-aligned workspace plus authenticated `@username` from `auth.test` share a single header row, followed by one blank row separating it from the panes. Rendering uses one shared-edge Unicode outer frame and an inset rounded composer frame. The composer has a prompt, a shortcut footer, and a solid outline in every state, accented when focused and muted when idle. Rounded corners, dashed transient frames, and dotted groups follow the Joshka reference without native cell-border sequences. Layout accounts for two outside columns, the shared column, two outside rows, two composer border rows, composer input rows, and one conversation-context row. History aligns the first visible row to a message header when it fits; oversized-message continuations repeat the author rather than strand body text without context. Below 76 columns the sidebar is a separate focused view. Mouse capture is disabled for native text selection; OSC52 copies only message text.

Current dependency APIs were checked against upstream Bubble Tea v2 documentation and installed module source/type documentation. Versions are pinned in `go.mod`/`go.sum`. No third-party Slack SDK is needed for these few HTTP methods.

The UI is organized by responsibility: `model.go` defines lifecycle/state, `commands.go` captures asynchronous requests, `state.go` manages local selection/cache/drafts, `input.go` and `picker.go` handle keys, and `history.go`, `delivery.go`, and `updates.go` apply typed results. Rendering is divided among layout, sidebar, composer, history, and dialog files. `snapshot.go` is an explicit offline review harness, separate from the pure `View` path. Shared rules live in [development standards](DEVELOPMENT.md) and [UI/UX guidelines](DESIGN.md).
