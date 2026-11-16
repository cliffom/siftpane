# Changelog

## 0.8.0 — Unreleased

- Preserve reading anchors through older-page loads, edits before the viewport, resize and name enrichment; choose a visible retained successor when selected content disappears. Keep scroll action targets visible and newest-following explicit.
- Retain available thread roots within bounded reply windows, stop automatic partial-thread page loops, and recover an expired cursor with one paced timestamp continuation. Keep incomplete/failed pages actionable.
- Open scrollable refresh/recovery details with `v`, retaining per-timeline freshness and separate recent/page failures after unrelated activity.
- Replace a registered account’s token with `--login --account WORKSPACE_ID/USER_ID` in another terminal, then navigation `Ctrl+R` to reload in-session. Preserve drafts, failed/uncertain delivery and original in-flight sends; reject another account’s token and retain pacing/backoff. Environment sessions revalidate only the inherited override.
- Improve first-run and missing-grant diagnostics; retain the internal-app user-token model and existing scopes/formats. No automatic send/reaction retry or disk draft persistence is added.
- Prepare Apple silicon/macOS 26+ candidate packaging, checksums, macOS CI and a manual Developer ID signing/notarization/stapled-DMG workflow. A local signed/notarized/stapled candidate passed Gatekeeper and mounted CLI checks. Credentialed Actions, browser-downloaded installation and real Keychain/terminal/live acceptance remain pending; no 0.8 release is published.

## 0.7.0 — 2026-11-15

- Save multiple account credentials in the OS keyring; switch explicitly with `Ctrl+W`. Restore each workspace/user session, thread, selection, scroll, drafts and send recovery. `--account WORKSPACE_ID/USER_ID` chooses the initial account.
- Keep Needs Attention and Kept separate per account. Only the active account refreshes or discovers; activation resumes bounded catch-up with existing overlap, pacing and visible gaps. Late reads are rejected; sends finish for their original account. Exit checks unsent text across retained accounts.
- Toggle your own Acknowledged (👍, `1`), Looking (👀, `2`) and Resolved (✅, `3`) reactions on the selected message, independently of local dismissal. Reinstall/re-authorize with `reactions:read` and `reactions:write`, then update each account token with `--login`. No automatic reaction or send retries.
- Open selected messages in Slack with `O`. Improve fenced code, quote and list readability, preserving literal mentions and links inside code.
- Migrate the single legacy keyring credential on first normal launch or login; retire its verified owner before deleting the old slot so logout cannot restore it. Legacy workspace inclusion/read preferences are claimed once by the verified legacy owner when available; copy the configuration before upgrading. Account preferences are not understood by older versions. Durable attention/cache identities retain their existing workspace/user hashes; drafts/outbox remain memory-only.
- The operator confirmed functionality and release acceptance. Assistant-run implementation evidence remains offline; bounded discovery, physical-terminal and cross-platform limitations remain.

## 0.6.0 — 2026-11-15

- Add independent opt-in `@channel` (`B`) and `@here` (`H`) attention search, preserving direct mention controls and durable message deduplication across sources.
- Locally follow an opened thread with `f`. Recover future replies from others across restart using bounded replies scans, exact targets, durable completion, and visible incomplete/gap/error status. This does not mirror Slack subscriptions.
- Opt into private account-isolated recent Kept history caching with `Ctrl+B` twice; `Ctrl+X` clears disk content. Show cached/refreshing/error state while fetching live pages. Drafts and outbox remain memory-only.
- The operator confirmed live acceptance for this release. Search filters/indexing/collapsing and bounded scan budgets still limit coverage; see the discovery evidence record.

## 0.5.0 — 2026-11-15

- Add Needs Attention and Kept, with explicit temporary add/open (`Ctrl+T` in a picker), keep (`K`), dismiss (`d`) and next-attention (`n`). Preserve existing included conversations as Kept. Reading and replying never dismiss.
- Persist minimal account-isolated handled identities and checkpoints; keep drafts and unresolved sends protected during dismissal. Retain known catch-up gaps until confirmed acknowledgement with `Ctrl+G` twice.
- Add opt-in direct mentions (`M` twice), exact message/thread opening, bounded catch-up, and checking/failure/limited-coverage status. Add `search:read` to the manifest; existing installations must reinstall and update their stored user token.
- The operator confirmed live acceptance for this release. Search filters, indexing and nearby-result collapsing still limit discovery; see the discovery evidence record.

## 0.4.0 — 2026-11-15

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
