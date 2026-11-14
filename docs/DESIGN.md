# UI and UX guidelines

Siftpane is a calm, keyboard-driven window into conversations the user explicitly includes. Favor readable messages, predictable navigation, and clear recovery over chrome. These guidelines preserve the reviewed baseline; change them deliberately alongside an authorized design change.

## Screen hierarchy

- A single header row: `siftpane` at left; workspace name and authenticated `@username` right-aligned. Retain one blank row below it. No tagline, duplicate read-state/focus description, or padding row below the footer.
- A shared-edge main frame with a left sidebar titled **Messages**. Do not restore the redundant `INCLUDED #/#` row.
- **CHANNELS included/total** and **PEOPLE** use dashed horizontal separators. Count only public/private channel metadata already loaded for the denominator; never imply a complete workspace count before discovery finishes.
- Reserve trailing sidebar columns for local unread dots and retained-draft markers, truncating names before those slots. Explain the symbols in help rather than a permanent sidebar legend. Do not imply Slack-synchronized counts or read positions.
- The message region has one muted context row for topic/purpose, conversation type, or known group size. Keep the actual conversation title prominent.
- The composer is inset, rounded, and solid on all four sides in every state. It has a `›` prompt and a clear title. Keep its bottom edge quiet except for pending/failed delivery recovery. Focus accents its frame; it does not change its border type.
- One concise status row finishes the screen with a right-aligned help entry (`?` in navigation, `F1` in composing/pickers). Do not repeat routine hotkey lists in the sidebar, composer or footer. Show message loading/refresh progress and startup restoration counts at the bottom left, replacing the idle status. Keep loading text out of the message area. Derive progress from current selection so unrelated completions cannot hide it. Explicit-action feedback (including refused quit and copy/read confirmation) takes precedence over timeline progress/errors until the next explicit key or status result. Recovery/discard warnings take precedence over progress and the help entry.

## Borders and color

The primary reference is Josh McKinney's [Terminal borders that don't cost a cell](https://www.joshka.net/cell-borders/), especially the style/joint gallery and rounded examples. Use its visual discipline with ordinary Unicode box drawing.

| Element | Treatment |
| --- | --- |
| Main panes, idle composer | Thin solid gray `#8693A5` |
| Active composer and transient frame | Teal `#8AC7BA` |
| Focused pane title | Bold amber `#E5C07B`, wrapped as `[Title]` |
| Selected conversation/message | Bold pale teal `#BFE4DA` and `▸` |
| Channel/people separators | Dashed approximation `╌` |
| Local-activity grouping | Subtle dotted delimiters |

Use matching colors on adjoining strokes and corners. Style focused title text and surrounding border segments independently: nesting the title inside a border style previously leaked amber into a horizontal stroke. Focus changes only the intended title/frame; preserve the accepted border palette. Brackets and selection arrows must still communicate focus without color. Do not put `[FOCUS]` before a title.

Keep shared boundaries single and junctions aligned. Do not box individual messages. Reserve every fallback border row/column in layout calculations. Never emit native cell-border sequences without confirmed capability support and renderer integration; native support remains future work.

## Navigation and text safety

- Navigation and composing are separate modes. Preserve the [documented keys](USAGE.md), discoverability, and quick searchable switching.
- Retain main-message and thread drafts across switches and removal/re-inclusion. Never clear text on failure. Warn before discarding unsent text on exit, and keep that warning visible during background updates.
- Lock pending sends against duplicate submission. Keep pending, failed, and uncertain states visible. Never retry writes automatically or promise exactly-once delivery after an ambiguous failure.
- Removing a conversation changes only local inclusion. Cancel its reads and ignore late results; do not leave or modify Slack membership.
- Align the first visible message to its author where practical. Oversized messages remain scrollable and repeat the author with a continuation label. Keep text selectable; mouse capture stays off. Unsupported Slack content needs useful text and a Slack link where available.
- Keep error and empty states concise, actionable, and free of implementation details that don't help the person recover. No AI ranking or summarization.

## Responsive review

Below 76 columns, use a full-width focused view rather than squeezing two panes. Minimum supported layout is 40×14; review 100×30 and 60×20 at a minimum. Long titles, workspace names, topics, and usernames must truncate by display-cell width without losing focus brackets or colliding with another region.

Review messages, channels, composing, threads, picker, and help. Include long/multiline drafts, long messages, unread/draft markers, failed/uncertain sends, and navigation after resize. Snapshot dimension tests cannot establish font stroke continuity, terminal color behavior, clipboard support, or live Slack correctness; use a terminal for those checks and record the evidence separately.

## Project artwork

The [project icon](BRANDING.md) combines a retro terminal and original conversation bubbles, inspired by Bambustand. Use the saved PNG for project documentation and the Slack app avatar; do not substitute Slack’s mark for Siftpane’s identity.

## Help modal

`?` in navigation or `F1` anywhere opens an opaque centered keyboard guide over dimmed context, using a solid rounded outline and bracketed amber title. Keep the status row readable. Wrap a single-column guide to the available width; reserve a fixed scroll-position/control row. Arrow keys or `j/k` scroll, `PgUp/PgDn` page, and `End`/`G` reaches the end. Any other key dismisses and is consumed, including send/quit/search keys. Pasted text is ignored. Dismissal preserves the prior conversation, thread, selection, focus and draft, including an underlying picker. `?` remains ordinary composer text. Review full guide reachability at the minimum viewport and after resize.

Layout measurements and defaults are owned by `internal/ui/layout.go`; palette values remain attached to named styles in `view.go`. Shortcut identifiers and display labels are owned by `keys.go` so handlers and hints share one source. Keep focus/picker state distinct from display copy and offline snapshot identifiers. Follow the [value policy](DEVELOPMENT.md#values-states-and-settings) when extending these definitions; a matching number alone is not a reason to share two unrelated layout measurements.

## Search modal

Search and include/remove use an opaque centered modal over a dimmed main view, preserving surrounding conversation context. Strip underlying color highlights and apply dim gray plus faint styling; keep the modal's teal solid rounded border and bracketed amber title independent. The modal fits the viewport with visible outside margins at 100×30, 60×20, and the 40×14 minimum. Input, suggestions, explicit selected action, directory coverage/state, and page controls reserve their own rows; long text truncates by display-cell width.

Search `Tab` completion is separate from inclusion. Always show **Enter include & open** for an excluded result and `[✓]`/`[+]` inclusion markers. Keep loading, empty, and directory errors inside the modal where background events cannot hide them. Restore the prior draft and composing focus on dismissal. See [search interaction](USAGE.md#search) and the [colored PTY review captures](review/search-desktop.png) / [small layout](review/search-small.png).

## Startup loading

Keep cached channel and people rows visible while saved metadata refreshes. On an uncached launch show restoration progress in the bottom status bar rather than inserting partial sidebar rows. Publish the restored list together when requests settle, with one CHANNELS group followed by one PEOPLE group, each ordered by name with ID as a tie-breaker. Explicit include/open actions must appear immediately in the sidebar and its keyboard navigation, without publishing unrelated pending rows. Reserve visible sidebar space for retry recovery even when the list fills; keep loading progress in the bottom status bar. Available history and search remain usable during restoration; metadata updates must preserve conversation selection and drafts. Missing metadata remains visibly retryable without claiming a complete directory. Use the bottom status bar for loading progress; keep empty history, initial-history errors and stopped authentication distinct in the message pane.

Message history should appear as soon as its page arrives. Use cached names or user IDs as interim author/mention labels, then update names in place without changing conversation selection, message selection, draft text or focus. Profile lookup failure keeps those readable fallbacks; it must not return a loaded page to a loading screen.
