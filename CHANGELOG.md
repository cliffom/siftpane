# Changelog

## 0.4.0 — Unreleased

- Right-align sidebar unread and draft indicators in reserved columns, keeping them visible beside long names. Remove the sidebar activity legend and repeated shortcut lists.
- Use a single quiet status row with a help entry point, giving messages another row. Show message loading/refresh progress and restoration counts at the bottom left instead of inside the message area. Retain pending/failed delivery recovery in the solid composer.
- Open a scrollable keyboard guide over dimmed conversation context with `?` from navigation or `F1` anywhere. Help consumes input and restores the prior context and draft on dismissal.

## 0.3.0 — 2026-11-14

- Display message pages before author/mention lookups finish, fill names in asynchronously for the active timeline, and reuse bounded account-isolated names across launches. Prioritize the active conversation’s initial page without changing API pacing, server backoff or send retries.
- Cache included channel and people display metadata across launches, isolated by workspace and account. Cached names are available immediately while live metadata refreshes; message bodies and drafts stay in memory.
- Keep the startup sidebar steady until saved metadata settles, then publish one alphabetized CHANNELS group and one PEOPLE group. Available message history and search remain usable while restoration continues.
- Show actionable initial-history failures instead of indefinite loading, and let `R` retry unavailable saved conversations as well as message history.

## 0.2.1 — 2026-11-14

- Keep every included conversation in the background refresh rotation while a thread is open. Thread refreshes no longer skip conversation turns, preventing stale messages and local unread indicators for even-sized selections. The three-second tick, alternating thread refresh, and one background timeline request per tick remain unchanged.

## 0.2.0 — 2026-11-14

- Show `Siftpane *` in the terminal window/tab title while any included conversation has locally observed unread activity. Restore `Siftpane` when all included conversations are read or removed, including below the minimum terminal size.
- The marker shares the sidebar's local state, is independent of Slack's read position, and depends on terminal/multiplexer title support. Polling delays still apply; no extra fetching or scopes are added.

## 0.1.0 — 2026-11-14

Initial versioned release of Siftpane, a terminal client for explicitly included Slack conversations, with an offline demo, retained session drafts, threads, and manual send recovery.

- Search existing accessible DMs, group DMs, and public/private channels using `Ctrl+K` or `/`; supported terminals can also deliver `Cmd+K`.
- Use `@` and `#` filters, case-insensitive names/handles, arrow navigation, and `Tab` completion in a centered modal over a dimmed main view.
- Include an excluded search result only with an explicit `Enter` selection. The `p` picker retains `Tab` include/remove behavior.
- Keep directory discovery paginated and rate-aware, with visible coverage, loading, and errors. Search does not create DMs, join channels, or add scopes.
- Inspect the embedded application version with `--version`.

Live Slack installation and workspace messaging remain unverified. Drafts remain in memory only; see [verification](docs/VERIFICATION.md) and [Slack limits](docs/SLACK.md).
