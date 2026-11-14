# Architecture

The program has one Bubble Tea event loop. State changes happen in `Update`; views format state and do not perform I/O. `tea.Cmd` closures perform Slack calls and return typed result messages. Configuration writes are small synchronous local-file operations; authentication runs before starting the TUI. No HTTP calls execute inside `View` or a key handler.

| Package | Responsibility |
| --- | --- |
| `cmd/siftpane` | Flags, credential-store/environment access, identity validation, lifecycle |
| `internal/core` | Service interface, conversation/message/page types, filtering, bounded merge, delivery state |
| `internal/slack` | Documented Web API transport, per-method pacing/backoff, coalesced profile lookup, immediate history pages, safe textual Slack content |
| `internal/demo` | In-memory implementation of the identical service interface, pagination, threads, deterministic send failure |
| `internal/store` | Workspace/conversation preferences and disposable display metadata, private atomic replacement, no credentials or bodies |
| `internal/ui` | Application state, commands, navigation, drafts/outbox, layout and rendering |

`core.Service` provides identity, direct conversation metadata, paginated discovery, paginated history/replies, and posting. Slack authentication uses a previously authorized user token from the internal app's install flow. `conversations.info` restores included IDs without scanning the directory. The directory is a user-paged metadata catalog; history is fetched only for included conversations. Demo mode creates a memory-only store and never opens a credential store or network transport.

Every included conversation gets a cancellation context and generation. Removal cancels outstanding reads, invalidates the generation, clears cached content and local notifications, and saves inclusion preferences. Late responses from a removed generation are ignored even after re-inclusion. A send already in progress is allowed to finish because cancellation cannot retract a message Slack may have accepted. Its draft/delivery state survives removal and informs the exit warning.

History cache eviction is LRU, limited to 16 timelines with 300 messages each. Old/new pages deduplicate by Slack timestamp. Recent history refresh replaces its observed timestamp range so changed/deleted messages in that window are reconciled. Loading older history retains the older portion of a full cache; newest refresh shifts back toward recent messages. Threads page forward and use an `oldest` watermark after all loaded pages are complete. Incomplete threads require explicit next-page requests. These bounds trade complete historical synchronization for predictable work and memory.

Local-read watermarks are independent of Slack. First observation establishes a baseline; subsequent observed timestamps can set a sidebar dot. Viewing/marking clears it locally. No Slack write occurs for selection or read-state changes. Drafts use conversation/thread keys and remain outside the timeline cache.

The outbox retains an immutable attempt body and stable random client ID. One delivery per target can be pending or failed; duplicate pending submissions are rejected. Acknowledgement clears only the matching draft. A transport-independent delivery-error interface classifies outcomes, including wrapped errors; display strings are never used for retry decisions. Definite failure permits manual retry; ambiguous failure requires an extra explicit retry after warning. Original text stays available for retry or restoration to the composer. Exactly-once delivery across a network failure is not guaranteed. Memory-only drafts are an intentional privacy tradeoff, with an explicit discard warning on exit.

The HTTP layer paces all calls per method with a mutex-protected clock. `429 Retry-After` extends that method's deadline, including manual calls. Waiting honors context cancellation. There are no automatic send retries. Automatic reads are bounded round-robin ticks; expired credentials stop refresh. The UI reports transport errors without logging request/response bodies.

The app name and right-aligned workspace plus authenticated `@username` from `auth.test` share a single header row, followed by one blank row separating it from the panes. Rendering uses one shared-edge Unicode outer frame and an inset rounded composer frame. The composer has a prompt, delivery-recovery text when needed, and a solid outline in every state, accented when focused and muted when idle. Rounded corners, solid modal frames, dashed separators, and dotted groups follow the Joshka reference without native cell-border sequences. Layout accounts for two outside columns, the shared column, two outside rows, two composer border rows, composer input rows, and one conversation-context row. History aligns the first visible row to a message header when it fits; oversized-message continuations repeat the author rather than strand body text without context. Below 76 columns the sidebar is a separate focused view. Mouse capture is disabled for native text selection; OSC52 copies only message text.

Current dependency APIs were checked against upstream Bubble Tea v2 documentation and installed module source/type documentation. Versions are pinned in `go.mod`/`go.sum`. No third-party Slack SDK is needed for these few HTTP methods.

The UI is organized by responsibility: `model.go` defines lifecycle/state, `commands.go` captures asynchronous requests, `state.go` manages local selection/cache/drafts, `input.go` and `picker.go` handle keys, and `history.go`, `delivery.go`, and `updates.go` apply typed results. Rendering is divided among layout, sidebar, composer, history, and dialog files. `snapshot.go` is an explicit offline review harness, separate from the pure `View` path. Shared rules live in [development standards](DEVELOPMENT.md) and [UI/UX guidelines](DESIGN.md).

## Value ownership

`core` owns the string-backed conversation kinds and delivery states shared by both services and the UI. Their textual values remain compatible; the preference schema is unchanged. UI focus and picker modes have separate private types, and offline snapshot mode identifiers are a distinct public boundary. The event loop continues to own all runtime state.

Immutable session limits, transport policy, CLI defaults, and layout/key presentation definitions stay in their owning packages and files, as specified by the [value policy](DEVELOPMENT.md#values-states-and-settings). They do not become mutable settings. Thread refreshes reserve every second tick while a thread is open; only conversation ticks advance the round-robin conversation index. Named durations and capacities explain the resource bound, while server backoff and runtime cancellation retain priority.

## Conversation search

Quick search and the include/remove picker share loaded conversation metadata and a single event-loop-owned text input. `core.Filter` understands `@`/`#` scopes; DM metadata retains the available handle alongside its preferred display name from the existing bounded `users.info` cache. No service method, scope, user-directory scan, new-DM creation, or per-keystroke network request is added. Explicit user-paged metadata discovery remains separate from included-conversation history polling.

The modal is composed with Lip Gloss layers over a color-stripped, dimmed copy of the main view. Rendering remains pure. Input and paste are routed to the modal even if an underlying send is pending. Dismissal restores composer focus; completion never selects/includes; explicit Enter can include-and-open. Asynchronous metadata updates retain the highlighted conversation's ID when list ordering changes. Directory busy/error/coverage state is independent of transient history/send status, and failed pagination preserves its retry cursor.

## Startup restoration

Before the event loop starts, `ui.New` reads the bounded disposable display cache through `store`, filtering it by current inclusion. `Init` schedules the active cached conversation's history alongside metadata restoration and directory discovery, so history does not await a directory scan or all saved metadata. Without a cache, the first available included conversation can start history immediately.

The startup sidebar retains its cached list (or a loading row on an uncached launch) while saved metadata requests settle, then publishes the grouped, name/ID-sorted result together. Search and message history can use arriving metadata immediately. Explicit include/open actions publish only their target into the startup list immediately, retaining grouped ordering and leaving unrelated pending metadata buffered. Sidebar rendering and keyboard navigation share the same published list; selection remains a conversation ID, independent of metadata sorting. Removal can end the startup wait, and late generations cannot change restoration state. The sidebar reserves a visible row for restoration state even when conversations fill the viewport, keeping the active row in view. Unavailable saved metadata has independent retry state; `R` retries those reads along with the current history. The existing per-method pacing and bounded polling remain unchanged.

Initial history errors stay on their timeline, so unrelated refreshes cannot turn them into an indefinite loading message. Explicit refresh clears that error while requesting a new page; successful empty history is distinct from loading. `store` owns cache schema, byte/entry/age bounds and whitelisted fields; cache writes reuse private atomic replacement independently of preferences. See [configuration and privacy](CONFIGURATION.md).

## Message names and first-page priority

History transport returns the raw bounded message page and Slack links as soon as the history request completes; it performs no author/mention requests. The optional UI-consumed profile service decorates names separately. The event loop schedules at most one name lookup at a time for the active timeline, newest messages first. Switching conversations/threads or removing the lookup's conversation cancels it and invalidates its generation. Late results cannot clear a newer lookup, change selection or drafts, or resurrect a removed timeline. Profile failures retain readable ID fallbacks and are attempted again only on a new focused view or explicit `R`; expired authentication still stops reads. Each focused view/manual refresh permits at most 512 distinct lookups, preventing eviction-driven loops.

The Slack client coalesces concurrent requests for the same user across metadata and message enrichment. Its synchronized name cache retains up to 512 minimal profiles; the UI copies snapshots onto the event loop and persists only changed snapshots. `store` isolates these disposable names by workspace and authenticated user in a separate, privately written cache, with per-entry expiry rather than renewing old identities on each save. Presentation decorates copies of message text, preserving raw cached text and avoiding interpreting display names as Slack markup. Rendering performs no I/O or mutations; clipboard output uses the same resolved names.

While the active conversation's first history request is pending, background ticks defer other timelines without consuming their round-robin turns. Normal rotation resumes after that page or an error, so an inaccessible active conversation cannot starve the rest. Thread rotation, explicit history requests, pacing, `Retry-After`, and send/retry semantics remain intact.

## Keyboard help

The keyboard guide shares the centered opaque/dimmed overlay renderer with search. Event-loop-owned help visibility and scroll offset leave underlying focus, picker, selection and drafts intact. Help handles keys before quit/send/search and blocks paste/input forwarding while still processing resize and service results. Single-column wrapped guide rows and viewport-derived bounds keep all shortcuts reachable; rendering clamps the offset without mutating state. The single status row remains readable below overlays and owns the minimal help entry point.
