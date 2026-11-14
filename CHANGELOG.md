# Changelog

## 0.2.0 — Unreleased

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
