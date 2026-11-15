# Attention discovery feasibility (#14)

Status: **proposed; live gate open**, 2026-11-15. This record advances [#14](https://github.com/cliffom/siftpane/issues/14), the first gate of [milestone 1](https://github.com/cliffom/siftpane/milestone/1). It does not establish a verified discovery mechanism or complete #14. No approved test installation has been established in this session, no Slack requests were made, and no scopes or subscriptions changed. Documentation review is evidence about the API contract, not evidence about this installation's coverage.

The milestone orders #14 → #15 → #16. Finish the live decision before treating the manual lifecycle contract (#15) or direct-mention integration (#16) as accepted. The proposed contracts below make that decision reviewable without changing today's inclusion-only read boundary.

## Candidate and documented constraints

Evaluate documented user-token `search.messages` first. It can query messages outside Siftpane's included list without enumerating every channel. This is a candidate, not a claim of complete personal-mention delivery.

| Mechanism | Official evidence checked on 2026-11-15 | Consequence |
| --- | --- | --- |
| [Message search](https://docs.slack.dev/reference/methods/search.messages/) | User `search:read`; Tier 2; timestamp sorting; page/count pagination (maximum 100 each), and a documented cursor option. Saved Slack UI filters affect results; nearby matches can collapse. | Test pagination and exact trigger recovery, including adjacent mentions. A successful empty response alone cannot prove complete coverage. |
| [Search scope](https://docs.slack.dev/reference/scopes/search.read/) | `search:read` is a legacy user scope. | Confirm it can actually be granted to this internal app; existing history scopes do not grant search. |
| [Real-time Search](https://docs.slack.dev/apis/web-api/real-time-search-api/) | The guide describes use exclusively in apps using AI features and in response to user interactions; granular scopes differ from legacy search. | Siftpane has no AI feature. Do not silently substitute this API for background discovery; establish suitability first. |
| [Message events](https://docs.slack.dev/reference/events/message/) and [Events API](https://docs.slack.dev/apis/events-api/) | History-scoped message families exist; delivery is best effort with bounded retries. | Test user-subscription visibility separately from bot visibility. Events alone do not demonstrate recovery while a desktop client is closed. |
| [App mentions](https://docs.slack.dev/reference/events/app_mention/) | This event describes mentions of the app's bot user. | Reject as a source for mentions of the authenticated person. |

No default-manifest change is justified yet. If legacy search passes, its incremental permission is user `search:read`; current metadata/history scopes remain necessary for opening context. No new posting permission is needed for discovery. If events prove necessary, verify user `message.channels`/`message.groups` coverage, and require a demonstrated historical gap-recovery mechanism before choosing them. Socket Mode would also require app-level `connections:write`, envelope acknowledgements and reconnection handling; it is not an offline inbox. An always-on receiver would add infrastructure and needs a separate concrete design.

Search may identify relevant activity; it does not reproduce Slack's notification, presence, mute, or read settings. Broadcasts and followed-thread replies are assessment cases for the next milestone, not implicitly enabled sources.

## Reproducible live experiment

Use an approved internal test app and synthetic conversations controlled by the operator. Keep tokens in the existing OS credential store or explicit environment override; never paste them into this record, shell arguments, captures or logs. Do not use a connected Slack connector as proof of Siftpane's own grant. The operator creates fixtures in Slack; the discovery experiment is read-only. No automated messages or membership changes are part of this plan.

1. Confirm workspace/user identity with `auth.test` and inspect the installation's granted scopes. Obtain administrator approval and reinstall the test app with `search:read` if required. Record grant success or refusal without recording credentials. Leave the shipping manifest unchanged until a mechanism is selected.
2. Prepare unkept public and private channels the person can access, a thread in each, and a public channel the person has not joined. Keep a private inaccessible channel as a negative control. Identify fixtures with synthetic labels and keep an operator-only mapping to Slack IDs/timestamps. Include ordinary chatter and a mention of a different person as negative controls.
3. Using the documented search call generator or a local client that reads the credential securely, compare queries containing the exact `<@USER_ID>` token and `to:me`, bounded by `after:YYYY-MM-DD before:YYYY-MM-DD`. These are **query candidates requiring verification**, not an asserted API mention syntax. [Slack's search guide](https://slack.com/help/articles/202528808-Search-in-Slack) documents date and thread modifiers. Use `sort=timestamp`, `sort_dir=asc`, `highlight=false`, `count=2`, `page=1`, then successive pages. Confirm which returned messages actually contain the exact mention token; name matches or contextual neighbors are not direct mentions.
4. Have the operator create separated and adjacent direct mentions in each accessible test channel/thread, using at least five matches to force pages. Include two mentions within seconds of one another. Verify exact conversation/message identities and whether a reply's root can be recovered from search or a bounded documented context request. If roots or collapsed matches cannot be recovered, record failure of exact-target coverage rather than substituting a nearby message.
5. Close Siftpane, have the operator create new mentions, and repeat the bounded search after reopening. Record elapsed indexing time and repeat after 1, 5 and 15 minutes. Repeat with restrictive saved Slack search filters and with them cleared. A filter-dependent miss must remain a declared coverage limitation; decide whether it fails milestone acceptance.
6. Replay all pages, restart between pages, insert activity during pagination, and retry a deliberately interrupted page. Compare identities, not result counts alone. Test the documented cursor path separately if page movement loses matches. Record the actual response pagination shape; do not assume the older example establishes cursor behavior.
7. Have the operator revoke access to a test private channel, then retry discovery and context opening. Separately revoke the test grant. Confirm denied/stale context is actionable, old results are not treated as current access, automatic checking stops on expired authentication, and no membership repair is attempted.
8. Inspect rate headers during normal bounded requests. Do not flood Slack to force `429`. If no live rate limit occurs, record it as unobserved and use offline transport tests for `Retry-After`, cancellation and no checkpoint advancement. [Slack's rate-limit guide](https://docs.slack.dev/apis/web-api/rate-limits/) specifies honoring the response delay.
9. Separately assess `@channel`, `@here`, and replies to a followed thread without a direct mention. Record whether search or approved user events observe them; do not equate observed broadcast text with Slack notification eligibility. Do not infer followed-thread state from ordinary message events.

Record each run below using fixture aliases, times, page counts, result identities as aliases, filter state, permission outcome and observed delays. Keep message bodies, tokens, real channel names and private permalinks out of committed evidence.

| Case | Required observation | Current result |
| --- | --- | --- |
| Public/private direct mention; root and thread | Exact trigger and usable root/context in an unkept conversation | Not run |
| Accessible nonmember public channel | Search visibility versus history/open permission | Not run |
| Adjacent mentions and multiple pages | No silently lost newer trigger after dismiss/replay | Not run |
| Closed-client catch-up and indexing delay | Recover synthetic mentions inside the proposed window | Not run |
| Saved filters and negative controls | Exclusions visible; other-user/chatter results rejected | Not run |
| Interrupted pages, overlap and restart | No skipped checkpoint interval or duplicate attention | Not run |
| Access/grant revocation | Explicit recovery; no unauthorized context requests | Not run |
| Server rate limit | Delay honored, or explicitly unobserved live | Not run |
| Broadcasts/followed-thread replies | Document coverage separately from direct mentions | Not run |

## Proposed bounded catch-up contract

These are implementation proposals to validate, not current defaults or demonstrated guarantees. Keep eventual named values in the Slack transport, UI limits and store owners described in [development standards](DEVELOPMENT.md#values-states-and-settings).

- Start with a seven-day initial lookback and a seven-day maximum catch-up window. A longer absence displays an explicit uncovered interval; it does not silently advance past it as if checked. Search retention, saved filters and indexing can reduce coverage further.
- Capture a fixed scan interval at check start, ending at the preceding UTC midnight; scan the current UTC day separately as provisional. Use documented calendar-date bounds padded by a day on each side, then filter by exact message timestamps locally. Validate Slack's date/timezone interpretation at midnight boundaries before acceptance. Recheck at least the previous two days on every cycle for delayed indexing. Older late-indexed messages remain an explicit limitation; do not claim exhaustive delivery.
- Try at most ten pages of 100 results per check, one request at a time, with a three-minute check interval, at least 3.1 seconds between search starts, existing HTTP timeouts, and authoritative server backoff. A full final page, inconsistent paging, malformed response, cancellation or budget exhaustion means incomplete coverage. Preserve the previous committed checkpoint and expose manual retry. Dense windows can be split by day on subsequent checks; if one day still exceeds the budget, report the unresolved gap rather than skip it.
- Commit a completed historical interval only after every page is validated and its accepted identities are durably stored. Keep successful partial entries visible and deduplicate their replay; never advance a checkpoint merely to the newest result seen. A persistence failure is a failed check. A provisional current-day check can record its successful check time, but cannot close that day or imply complete coverage.
- Cap active attention at 128 conversations and 1,000 trigger identities per account. Capacity exhaustion must retain the old checkpoint and report incomplete coverage. Durable handled identities must outlive the replay window; prune them only when that interval cannot be replayed. A seven-day lookback floor must remain enforced after pruning. Restarting, re-enabling discovery or losing a checkpoint must not quietly expand the replay window and resurrect pruned dismissals.

Calendar search and mutable indexes cannot establish archive completeness. The live decision must explicitly say whether this bounded, delayed source satisfies the milestone. If it does not, select and validate another documented mechanism before integrating it.

## Proposed lifecycle handoff to #15

Keep transport-independent identity and lifecycle types in `core`, persistence in `store`, and event-loop-owned selection/draft/delivery in `ui`. Discovery should use a separate optional service capability with context cancellation, independent of directory search and `Service.History`. Demo and automated tests remain offline.

| Identity/state | Proposed contract |
| --- | --- |
| Account | Workspace ID + authenticated user ID from `auth.test`; no display names or token hashes as logical identity. Existing inclusion preferences are only workspace-keyed, so migration must explicitly seed the current account as Kept without sharing new dismissals/checkpoints with another account. |
| Trigger | Account + conversation ID + message timestamp + typed source reason. Preserve timestamp precision; never use floating point or display text as identity. Root timestamp is a separate opening target, not a substitute for the reply identity. |
| Manual temporary add | Locally generated stable identity; an explicit re-add makes a new trigger. Searching, completing and opening an existing result never creates a subscription implicitly. |
| Attention | One row per conversation with aggregated reasons and an exact selected trigger. Kept membership is independent; a kept conversation with attention must not acquire a duplicate sidebar row. |
| Dismissal | Persist handled trigger identities atomically before hiding attention. A subsequent distinct direct mention can return it; chatter cannot. Dismiss only the observed set, so a concurrently arriving new trigger survives. Read, reply and reaction do not dismiss. |
| Text safety | Refuse dismissal while any conversation/thread draft or pending/failed/uncertain delivery needs recovery. Provide explicit keep/open/recovery, never silently hide text, cancel a send as if retracted, or automatically retry it. |
| Selection/context | Opening a trigger fetches only bounded context for that explicitly opened target. Cancel removed generations and ignore late results. Keep current selection/drafts stable during unrelated discovery completions. Access loss leaves an actionable unavailable target. |
| Persistence | Separate versioned account-isolated durable state, independent of disposable message/name caches. Store IDs, reasons, timestamps, enabled source and checkpoints, never bodies or credentials. Private atomic writes; corrupt state must not be silently reset. |
| Navigation | Next-attention order is deterministic (trigger timestamp then conversation ID), independent of completion order. Exact keys and row presentation belong to #15's design and viewport review. |
| Freshness | Typed disabled/checking/current/incomplete/failed/auth-stopped states. Last successful check, last completed interval and known gaps are distinct; an empty list is not a success signal. Keep manual use and existing results available on failure. |

## Decision and next step

**No mechanism selected as verified.** The next required input is an approved internal installation and an operator able to create synthetic fixtures and grant/revoke test access. Complete the matrix, settle query/pagination/context recovery and the supported window, then amend this record with observed evidence and an explicit search/events decision. If events are needed, bring subscriptions and gap recovery into #16 rather than describing polling as equivalent.

After that decision, #15 implements and validates the manual lifecycle; #16 integrates the selected source and performs the milestone's live close/mention/reopen/respond/dismiss/restart/new-mention acceptance pass. Keep #14, #15 and #16 open until their actual acceptance criteria pass. This documentation-only preparation needs no dedicated version bump under [release policy](RELEASING.md); the current published version remains 0.4.0.
