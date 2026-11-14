# Verification record

Verified locally on 2026-11-13 using macOS arm64 and Go 1.26.7 / Go 1.27.4. The module declares Go 1.26.7, the highest minimum required by its pinned direct dependencies. Other OS builds, OS keyring behavior, and actual terminal fonts have not been exercised.

`make check` passed on both toolchains: gofumpt/goimports formatting checks, pinned golangci-lint v2.15.0 (zero findings), `go vet ./...`, `go test -race ./...`, build, and tidy/checksum checks for the application and tooling modules. The formatting gate was tested with a deliberately malformed temporary Go file and correctly failed; the probe was removed. Tests use in-memory demo data and a local `httptest` server, with no Slack credentials. `--help` and all five offline snapshot modes were also exercised.

Unit/adapter tests check inclusion filtering, workspace-specific persistence/removal and file permissions, corrupt-file handling, overlapping/capped message merges, history/thread/directory parameters and cursor propagation, rate-limit cancellation and retained Retry-After deadline, expired authentication, ambiguous send errors, fallback formatting, send identity/duplicate guards, draft retention across conversation/thread switches, failure/retry, pending-paste protection, late fetch rejection after removal/re-inclusion, LRU bounds, older-page visibility, local-read indicators, direct lookup of saved selections, header-aligned viewports, oversized-message continuation context, Slack topic/member metadata, and distinct demo histories with correct DM participants, typed/wrapped delivery outcomes, invalid demo cursor handling, and an unsent-exit warning that survives background updates.

Layout tests render messages, composing, picker, thread, and help at 120×36, 100×30, 80×24, 60×20, and 40×14. They assert every row's display-cell width, total row count, non-color focus labels, absence of doubled vertical borders, and absence of experimental cell-border output. Help text is truncated to the viewport on short terminals; [the keyboard guide](USAGE.md) remains the complete shortcut reference.

A subprocess attached to a real PTY ran `bin/siftpane --demo` at **100×30** and **60×20**. A temporary Python/pyte harness sent ordinary key bytes and decoded the terminal output. Both runs passed navigation, thread opening, threaded sending, switching away/back with draft preservation, simulated failed send and retry, older-page loading, the conversation picker, unsent-exit warning, group-size context, and clean confirmed exit. No experimental cell-border sequence appeared in the captured output. These are interactive process checks, not just model snapshots.

The organization/standards refactor was compared against baseline commit `211704d`: all 25 snapshot combinations (five views × five sizes) matched byte-for-byte. GitHub Actions is configured to run the same checks with minimum/stable Go on Linux, but the hosted workflow has not been run in this local session.

Current rendered review captures (refreshed alongside UI changes):

- [Desktop messages](review/desktop-messages.txt): shared sidebar boundary, inset composer frame, readable message spacing, selected-message marker, local unread dots.
- [Small thread composer](review/small-thread-composer.txt): full-width thread history, wrapping, explicit composer focus, visible draft, compact help.
- [Small group DM](review/small-group-context.txt): accurate demo group size, distinct conversation history, author-aligned viewport, and compact help.
- [Conversation picker](review/conversation-picker.txt): rounded dashed transient frame, explicit checkboxes, dotted grouping, focused row.

The article and its `styles.png` and `panes-box.png` examples were fetched and visually inspected. Rounded solid, dashed, and dotted border guidance is reflected in the fallback. PTY decoding and plaintext review do **not** prove stroke continuity or font metrics on real terminal emulators; native cell borders were deliberately not implemented.

## Outstanding live verification

No live Slack credentials were used or assumed available. The implementation is complete for the documented core Web API flow, but importing the manifest into Slack and end-to-end verification remain outstanding. Do not interpret fake-server/PTY success as proof of successful app installation or live event delivery.

With an approved internal app, verify:

1. Import manifest; inspect user-scope grant; install; store `xoxp-` token through `--login`; confirm `auth.test` workspace/user identity.
2. Include one public channel, one accessible private channel, one DM, and one group DM. Load directory pages, remove/re-include an item, and restart to verify saved selections.
3. Read/send as the user; confirm author in Slack; open/read/reply to a thread; load older history and multiple thread pages.
4. Confirm local-only read behavior and delayed polling dots. Remove a conversation while a fetch is pending and confirm it stops receiving local activity.
5. Exercise permission denial, revoked/expired credentials, intermittent network failure, and server rate limits. Inspect Slack before retrying any ambiguous send.

Potential external blockers are workspace app approval, user-scope restrictions, conversation membership/posting policy, and OS keyring availability. The app makes no attempt to bypass them. Socket Mode is not configured, and there is no claim of receiving a complete Slack client event stream.

## Issue #4: owned values and typed states

Verified locally on 2026-11-14. `make fmt` and `make check` passed, including the pinned `goconst`/`mnd` checks, vet, race-enabled tests, build, and both modules' hygiene checks. Additional regression tests cover delivery transitions and original-attempt retention, exact serialized domain values and round trips, kind searching, navigation/picker draft retention, selection-limit feedback, and unchanged transport pacing/page sizes. No Slack credentials or live endpoints were used.

A temporary package with a string repeated three times and an unnamed numeric condition was rejected by the configured `make lint` gate with `goconst` and `mnd` diagnostics; the probe was removed. The checks' scope and upstream limitations are recorded in the [value policy](DEVELOPMENT.md#values-states-and-settings).

All 25 offline snapshots (messages, compose, picker, thread, help at 120×36, 100×30, 80×24, 60×20, 40×14) matched baseline `c9520e1` byte-for-byte. The 100×30 messages and 60×20 composing/thread layouts were inspected as text. Existing review captures remain unchanged because appearance is unchanged. This pass adds no terminal font/color, clipboard, keyring, or live Slack verification; prior limitations still apply. Hosted CI is reported separately in the PR.

No dedicated release or version change is needed: this is a behavior-preserving refactor and contributor-policy update. The repository has no published tags/releases or authoritative application version to advance at the time of this assessment.

## Issue #3: conversation search modal

Verified locally on 2026-11-14 with Go 1.27.4 on macOS arm64. `make fmt` and `make check` passed, including pinned lint, vet, race tests, build, and application/tooling module hygiene. Offline coverage now includes prefix/name/handle filtering, completion without inclusion, explicit include-and-open, inclusion-limit recovery, navigation/composing dismissal with retained thread drafts, search paste during a pending send, async result identity, explicit directory pagination/retry/error retention, viewport fitting, and modal border/color isolation. Existing fake Slack tests cover pacing and scopes; the added fake-server case verifies available display names/handles share one cached lookup.

All six snapshot modes were reviewed by dimension tests at 120×36, 100×30, 80×24, 60×20, and 40×14. Review text captures were refreshed, with new [desktop search](review/search-desktop.txt) and [small search](review/search-small.txt) captures. Search and inclusion modals have solid rounded outlines; help/separators retain dashed strokes. Modal color tests verify the backdrop is dim gray while the title remains amber and the border teal, including after wide-query resize.

An offline subprocess attached to a PTY passed at 100×30 and 60×20: opening search from a draft, explicit next-page loading, filtering/completion, cancellation back to editable text, a delivered Kitty Super+K sequence, explicit inclusion/opening, returning to the retained original draft, and confirmed unsent exit. [Desktop](review/search-desktop.png) and [small](review/search-small.png) PNGs render the decoded PTY cells using Menlo with true color enabled for the capture. These are reproducible terminal-grid renders, not screenshots proving every terminal/font or physical Command-key behavior. The original process environment used NO_COLOR; it was removed only for the colored capture subprocess. Text/focus cues remain testable without color.

The built binary and a source install into a temporary GOBIN both reported `siftpane 0.1.0`; the temporary install ran an offline small search snapshot. The initial v0.1.0 is prepared in the embedded version source, changelog, and release guide, with no tags/publication performed by this implementation. No permissions, API paging/pacing policy, or persisted preference schema changed. Live Slack, OS keyring, native clipboard, and physical Cmd+K mapping across terminals remain unverified; no real workspace messages or credentials were used in these checks. Hosted CI is reported separately in the PR.
