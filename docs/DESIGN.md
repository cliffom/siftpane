# UI and UX guidelines

Siftpane is a calm, keyboard-driven window into conversations the user explicitly includes. Favor readable messages, predictable navigation, and clear recovery over chrome. These guidelines preserve the reviewed baseline; change them deliberately alongside an authorized design change.

## Screen hierarchy

- A single header row: `siftpane` at left; workspace name and authenticated `@username` right-aligned. Retain one blank row below it. No tagline, duplicate read-state/focus description, or padding row below the footer.
- A shared-edge main frame with a left sidebar titled **Messages**. Do not restore the redundant `INCLUDED #/#` row.
- **NEEDS ATTENTION** and **KEPT** use dashed horizontal separators. Attention rows reserve space for their primary reason (`mention`, `@channel`, `@here`, `reply` or `temporary`) and additional distinct-trigger count before unread/draft slots. A kept conversation needing attention appears once, in Needs Attention; dismissal returns it to Kept. Directory coverage remains in the picker, independent of attention.
- Reserve trailing sidebar columns for local unread dots and retained-draft markers, truncating names before those slots. Explain the symbols in help rather than a permanent sidebar legend. Do not imply Slack-synchronized counts or read positions.
- The message region has one muted context row for topic/purpose, conversation type, or known group size. Keep the actual conversation title prominent.
- The composer is inset, rounded, and solid on all four sides in every state. It has a `›` prompt and a clear title. Its placeholder uses the readable muted palette in both focus states; the editing row retains the terminal background instead of an ANSI black highlight. Keep its bottom edge quiet except for pending/failed delivery recovery. Focus accents its frame; it does not change its border type.
- One concise status row finishes the screen with a right-aligned help entry (`?` in navigation, `F1` in composing/pickers). Do not repeat routine hotkey lists in the sidebar, composer or footer. Show initial message loading and startup restoration counts at the bottom left. Routine polling and successful refreshes remain silent; credential and unresolved-delivery recovery is derived from persistent state and survives ordinary keypresses. Show a stale-data indicator only after the selected timeline’s last successful recent fetch is more than 30 seconds old; retain actionable errors, cached provenance and partial coverage. Keep loading text out of the message area. Derive progress from current selection so unrelated completions cannot hide it. Explicit-action feedback (including refused quit and copy/read confirmation) takes precedence over timeline progress/errors until the next explicit key or status result. Recovery/discard warnings take precedence over progress and the help entry. Give action feedback the full status row when the help entry would clip it, and keep quit recovery copy short enough to show the complete exit instruction at 40 columns.

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

Opening either conversation picker automatically loads remaining directory pages while showing progressive matches and coverage. Closing the picker cancels loading; failures and scan limits retain results and offer explicit continuation. DM names resolve independently of directory pages, with unresolved name coverage labeled separately from completed directory pagination. Cached directory suggestions are marked and verified before opening or inclusion; unkeeping an existing preference remains local. Pasting a Slack channel link or ID offers an explicit Enter verify/open action; show lookup progress and retain input/drafts on failure. Workspace mismatches offer switching rather than silently choosing another account. Search `Tab` completion is separate from inclusion. Always show **Enter include & open** for an excluded result and `[✓]`/`[+]` inclusion markers. Keep loading, empty, and directory errors inside the modal where background events cannot hide them. Restore the prior draft and composing focus on dismissal. See [search interaction](USAGE.md#search) and the [colored PTY review captures](review/search-desktop.png) / [small layout](review/search-small.png).

## Startup loading

Keep cached channel and people rows visible while saved metadata refreshes. On an uncached launch show restoration progress in the bottom status bar rather than inserting partial sidebar rows. Publish the restored list together when requests settle, with Needs Attention followed by Kept. Attention uses the oldest pending trigger time then conversation ID; Kept retains channels before people, ordered by name with ID as a tie-breaker. Explicit include/open actions must appear immediately in the sidebar and its keyboard navigation, without publishing unrelated pending rows. Reserve visible sidebar space for retry recovery even when the list fills; keep loading progress in the bottom status bar. Available history and search remain usable during restoration; metadata updates must preserve conversation selection and drafts. Missing metadata remains visibly retryable without claiming a complete directory. Use the bottom status bar for loading progress; keep empty history, initial-history errors and stopped authentication distinct in the message pane.

Message history should appear as soon as its page arrives. Use cached names or user IDs as interim author/mention labels, then update names in place without changing conversation selection, message selection, draft text or focus. Profile lookup failure keeps those readable fallbacks; it must not return a loaded page to a loading screen.

## Attention lifecycle

`Ctrl+T` in either picker is an explicit temporary add/open. It does not become Kept. Navigation `K` toggles Kept for the selected conversation, preserving pending attention independently. Ignore held-key repeats. Refuse a direct unkeep that would hide conversation/thread drafts or unresolved deliveries; after removal return focus to the sidebar; `d` dismisses its observed triggers only after durable persistence. Refuse dismissal while any conversation/thread draft or pending/failed/uncertain send needs recovery. Never silently hide composed text. `n` opens the next attention conversation in deterministic order and selects the exact triggering message/thread. Reading and replying do not dismiss; later distinct direct mentions can return a dismissed conversation.

`M` from navigation discloses unkept mention access and `search:read`; a second consecutive `M` enables discovery. An intervening key cancels confirmation. Reported key-repeat events cannot disclose, confirm or toggle discovery from navigation; repeated letters remain ordinary composing/picker input. Composing/pasting and help never trigger this action. One `M` disables an enabled source, preserving entries. Show checking, last successful check with limited-search coverage, failure/retry and catch-up gaps in the quiet footer. Automatic results must preserve selection, draft and focus, including during startup. History reads for an unkept attention entry begin only on explicit opening.

Known catch-up gap warnings persist until explicitly acknowledged with two consecutive `Ctrl+G` presses in navigation. Show the date bounds and acknowledgement shortcut where width permits; keep the gap indicator at small widths. Confirmation never advances discovery coverage or dismisses entries. Keep credential recovery readable through the minimum width when discovery authentication stops reads.

## Optional resume and attention sources

`B`/`H` control broadcasts independently of `M`; enablement requires two consecutive physical presses with unkept-search disclosure. `f` toggles explicit local following only in an opened thread; its context row says locally followed and how to unfollow. New activity groups without changing selection, focus or draft. Source scan completion, failure/incomplete and gaps stay visible independently of list contents.

`Ctrl+B` enables content caching only after a second consecutive physical press disclosing disk storage. One press disables an enabled cache; `Ctrl+X` clears account disk history. These are navigation actions, consumed by help and never activated by pasted text. Cached provenance/time and refresh failures appear in the bottom status row, preserving message space and the solid composer. Drafts/outbox stay in memory. Review cached, cached-error, mixed attention and local-follow states at desktop, small and minimum sizes; [captures](review/cached-desktop.png) are offline cell renders, with physical-terminal limits still applicable.


## Accounts and meaningful participation (0.7)

`Ctrl+W` opens the solid, centered Workspaces modal from navigation or composing. Arrow keys/j/k move, Enter explicitly switches and Esc restores the prior context. Show workspace names and @usernames in aligned columns without account IDs. Measure names in display cells across all accounts so scrolling preserves column alignment; truncate long workspace names to at most half the available name columns to retain username space. The selector explains that inactive accounts pause reads and omits inactive attention counts. Opening or canceling the selector clears pending source, caching, gap and retry confirmations, as do coordinator-owned quit actions. Switching restores the account's view and draft without carrying a confirmation into another account. Help consumes the shortcut. Pasting while the selector is open changes no draft.

The active header includes stable workspace/user IDs, prioritizing them at small widths; composer and recovery titles retain human identity. Needs Attention and Kept always show only this account. Catch-up retains the existing checking, limited, incomplete, gap and credential-recovery states. At newest, refresh advances both the visible window and selected action target to the newest message. Explicit browsing selections remain anchored; refresh and account activation preserve their focus and drafts, and activation does not refresh a scrolled view away from its anchor.

Navigation `1`, `2`, `3` toggle your own Acknowledged (+1), Looking (eyes), Resolved (white_check_mark) reactions. They neither dismiss local attention nor imply another person's acknowledgement. Ignore held mutation keys in navigation; in composing and both pickers, these characters and their key-repeat events remain ordinary text. Pending reactions reject duplicates; failures direct the user to inspect Slack before toggling again. `O` explicitly opens the selected message's canonical Slack permalink using the original account service. No browser is launched from rendering, discovery or text links.

Code fence content wraps without truncation, including text immediately after opening or before closing fences. Text following an opening fence is literal code, never an inferred language label. Code fences have muted opening/closing rails; code and quotes have a vertical rail; unordered lists use bullets and aligned continuation rows. Keep inline code literal and preserve mention/link syntax inside code. Clipboard text retains content rather than the display rails. Review Workspaces, drafts, uncertain recovery, help and formatted messages at 100×30, 60×20 and 40×14.


## Recovery details and credentials (0.8)

`v` from navigation opens a solid framed Refresh and recovery modal over dimmed context. It is scrollable at desktop, small and minimum sizes. Retained per-timeline freshness/errors and thread partial coverage are independent of the transient status row; sources retain their own coverage semantics. Arrows/PgUp/PgDn scroll, R retries the current view/sources and Esc dismisses without changing focus/drafts. Paste and unrelated action keys are consumed. Keep the bracketed amber title, independent border color, shared edges and solid composer unchanged.

Navigation Ctrl+R revalidates the current account’s saved credential after `--login --account WORKSPACE_ID/USER_ID` elsewhere. The picker retains its directory reload binding; composing/help/Workspaces/recovery details do not trigger account reload. Show checking/failure/success without implying a send retry. Identity mismatch retains the original session. No token is entered into a message input. Recovery details show the full account-specific instructions; short credential-expiry status keeps the reload key visible.

Background work preserves reading context where the retained window permits. Older-page loads preserve an existing browsing selection; deleted identities fall forward to a retained successor (or last retained message). Scrolling updates action selection when its old message moves out of view. G explicitly resumes newest-following. Partial thread state offers explicit o pagination and exposes failed-page details via v without automatic whole-thread loading.
