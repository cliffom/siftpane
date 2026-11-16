# Attention discovery decision and evidence (#14)

Status: **bounded search source; operator-confirmed release acceptance**, 2026-11-15. [Milestone 1](https://github.com/cliffom/siftpane/milestone/1) orders #14 (discovery decision), #15 (manual lifecycle), then #16 (integration/live acceptance). The operator confirmed the outstanding live acceptance as “verified” in the release conversation. This resolves PR #18's draft release gate; no case-by-case operator fixture log or timings were supplied, so the assistant's observations below remain separately attributed. GitHub issue/milestone state is managed separately from publishing this release.

## Documented basis

| Mechanism | Official evidence checked on 2026-11-15 | Decision |
| --- | --- | --- |
| [Message search](https://docs.slack.dev/reference/methods/search.messages/) | User `search:read`, Tier 2, timestamp sorting, page/count and cursor options. Saved Slack UI filters affect results; nearby matches can collapse. | Use exact-token search as an opt-in, explicitly limited source. Do not equate a successful empty page with complete mention delivery. |
| [Search scope](https://docs.slack.dev/reference/scopes/search.read/) | Legacy user scope, distinct from history scopes. | Added to the internal app grant and checked-in manifest; reinstall existing installations. |
| [Real-time Search](https://docs.slack.dev/apis/web-api/real-time-search-api/) | Described for apps using AI features and in response to user interactions. | No silent substitution for this non-AI background client. |
| [Message events](https://docs.slack.dev/reference/events/message/) / [Events API](https://docs.slack.dev/apis/events-api/) | History-scoped families, best-effort delivery and bounded retries. | No subscriptions yet. Test user visibility and historical recovery before selecting events. |
| [App mentions](https://docs.slack.dev/reference/events/app_mention/) | Mentions of the app's bot user. | Not a personal-mention source. |

Search detection is independent of Slack's notifications, mute/presence and read settings. In 0.5, broadcasts and followed-thread replies without direct mentions were not enabled sources; 0.6 adds independent opt-in controls and explicit local follows, with operator-confirmed 0.6.0 release acceptance recorded below. If controlled tests show that search collapsing/filters prevent the milestone's required new-mention behavior, bring user events and a demonstrated historical gap-recovery mechanism forward. Socket Mode alone is not a closed-client inbox; an always-on receiver would require its own concrete design. Current subscriptions and Socket Mode remain off.

## Actual read-only observations

The user approved all conversations for read-only testing and refreshed the installation with `search:read`. Authentication succeeded using the existing OS credential store. Secrets and message bodies were never logged or committed. No messages, membership changes, read-position writes, app configuration changes through APIs, or credential-store writes were performed by the probes.

- Exact `<@USER_ID>` search over the preceding seven days returned 35 exact matches across two pages (`count=20`), with 35 distinct conversation/message identities. Replaying page one preserved its identities. Comparing the local Kept IDs found 32 matches outside Kept. Three matches had private-conversation metadata; this alone does **not** establish private-channel/thread coverage, since group DMs can be private too.
- The `to:me` candidate reported 96 results; neither of its first two results contained the exact mention token. The implementation therefore validates the raw exact token and does not use `to:me` as a synonym for mentions.
- Search matches carried thread roots in permalink query parameters rather than top-level `thread_ts`. A bounded public-thread reply request returned the exact searched message. No private-thread sample was available for that pass.
- A later run through the actual Go `Client.Mentions` adapter returned 37 exact matches, including 34 thread targets. `Client.History` with its new target anchor returned the exact selected trigger. A discovered identity was dismissed, atomically saved in a private temporary account-state file, reloaded and replayed without resurrection. That temporary directory was removed afterward. This is read-only adapter/store evidence, not a complete interactive TUI acceptance pass.

Counts describe those observations, not complete workspace coverage or stable fixture expectations. Differences across runs are not proof of controlled new activity. No claim is made that all matching messages were returned, that indexing completed within a guaranteed latency, or that any controlled mention was received while the client was closed.

The following matrix records **assistant-run observations before the operator confirmation**. The operator confirmation did not include case-by-case results.

| Case | Assistant-run evidence / earlier validation gaps |
| --- | --- |
| Public direct mention and exact thread opening | Existing-message search and production adapter context observed; controlled fixture still needed |
| Private-channel/root/thread coverage | Private-conversation search observed; private-channel and private-thread fixtures not established |
| Unkept conversation | Existing matches outside Kept observed |
| Pagination and duplicate replay | Two live pages and stable first-page replay; deterministic offline interruption/budget/changed-total tests |
| Adjacent new mentions after dismissal | Not exercised live; documented collapsing remains a feasibility risk |
| Closed-client catch-up / indexing latency | Historical mentions returned; controlled close/receive/reopen sequence not run |
| Saved filters and access/grant revocation | Not exercised live; offline failure/cancellation/late-result handling covered |
| Server rate limit | No live `429` observed; offline transport tests cover delay/cancellation |
| Broadcasts / followed-thread reply coverage | No controlled fixtures; subscription/notification equivalence unverified |
| Respond/dismiss/restart/new mention | Durable discovered-identity reload/replay observed; interactive response and controlled second mention outstanding |

## Implemented bounded contract

The optional `Mentions` capability is separate from directory search and `Service.History`. Discovery starts disabled; `M` twice explicitly enables it after disclosing accessible unkept mention access and `search:read`. An intervening key cancels confirmation. The preference is account-isolated. Disabling cancels the scan, invalidates late generations and retains entries. Discovery never enumerates all channels or reads an unopened unkept history.

- Initial and maximum catch-up: seven days. Incremental scans revisit two days before the last completed checkpoint, bounded by a monotonic persisted replay floor. A longer absence produces a durable visible catch-up gap. Success/restart cannot clear it; confirmed `Ctrl+G` acknowledgement records that the user accepts the warning, without checking the missing interval. Search date bounds are padded by a day, followed by exact local timestamp filtering; midnight/timezone edge coverage remains a live test item.
- One request at a time, 100 results per page, ten pages per check, three minutes between checks, at least 3.1 seconds between search starts. Server `Retry-After` is authoritative. Requests remain cancelable and use existing HTTP timeouts.
- Accepted identities persist page by page. Only a validated terminal page and successful save advance the completed interval/last-success time. Malformed/changed pagination, page exhaustion, failure or capacity limits keep the preceding completion and partial entries visible. `R` retries. A full page can still be terminal when consistent paging metadata says so.
- Active attention is capped at 128 conversations; active plus handled identities at 1,000 per account. Handled records expire only behind the replay floor. Pending entries remain until explicit dismissal. Never expand backward over pruned identities after restart/re-enable. Capacity exhaustion is an incomplete check, not permission to drop entries.
- The footer labels successful checks **limited search**, and separately shows checking, failure/retry and known gaps. Local success cannot overcome saved filters, retention, indexing beyond the overlap, old edits or result collapsing. Dense intervals exceeding the budget remain incomplete; automatic day splitting is not implemented.

## Identity and manual lifecycle contract

`core` owns the typed reason and lifecycle state; `store` owns a separate versioned workspace/user-isolated durable file; `ui` owns selection, commands and cancellation. See [architecture](ARCHITECTURE.md#attention-ownership) and [privacy](CONFIGURATION.md#durable-attention-state).

Existing included IDs migrate to Kept. A direct trigger uses its source reason/message timestamp inside a conversation and account; the root timestamp is a separate opening target. Timestamp parsing never uses floating point. A manual temporary add has a fresh random stable ID and explicit creation time, so a deliberate re-add is new activity. Repeated triggers group by conversation; a Kept conversation with attention appears once. Ordering uses the oldest pending trigger time then conversation ID, independent of asynchronous completion order.

`K` keeps explicitly; `d` saves handled identities before hiding observed triggers; `n` opens the next exact trigger. Read/reply does not dismiss. A distinct later mention can return the conversation. Any retained conversation/thread draft or pending/failed/uncertain send blocks dismissal until recovery. Failed persistence leaves the entry visible. Context access failure remains actionable; removal cancels reads and ignores late generations. Cache eviction cannot discard durable dismissal state. No automatic send retries or Slack membership/read-state changes are introduced.

## Reproducible live protocol

Use synthetic fixture labels and keep the mapping to real IDs/timestamps private. Keep credentials in the OS store or explicit override, never shell arguments, logs or chat. An operator creates the fixtures; probes remain read-only unless the user explicitly authorizes a send.

1. In accessible public and private test channels, create separated and adjacent direct mentions, including replies; include other-user mentions/chatter as negative controls. Include an accessible nonmember public channel and an inaccessible private-channel control.
2. Verify exact-token search, forced multi-page results and exact targets, with saved UI filters both restrictive and cleared. Check midnight/date-bound behavior. If nearby newer mentions are hidden after dismissal, record a feasibility failure; do not silently accept a nearby substitute.
3. Close Siftpane, receive a mention, reopen and check indexing at 1, 5 and 15 minutes. Open/respond, dismiss, restart and verify suppression; then receive a new mention and confirm return. Record response/send identity separately from read-only proof.
4. Interrupt a page, restart/replay, and insert activity during paging. Confirm partial entries survive but completion cannot skip the failed interval. Test access and grant revocation without trying to repair membership.
5. Observe normal rate headers; do not flood Slack to provoke `429`. Record unobserved live backoff separately from deterministic offline tests. Assess `@channel`, `@here` and followed-thread-only replies separately from personal mentions.

The operator confirmation allows the reviewed 0.5.0 implementation to proceed through the release workflow. The documented search limits and the distinction between observed, offline-tested and operator-confirmed behavior still apply. This release operation does not separately close or edit #14/#15/#16 or milestone #1.

## 0.6 extension evidence

Milestone 2 implementation tracks #11 (opt-in content cache), #19 (separate broadcasts), #20 (local thread following), and #21 (shared identity/catch-up). Official method and special-token documentation was rechecked on 2026-11-15; links and the bounded contract are in [Slack coverage](SLACK.md#broadcast-and-local-follow-coverage-06). The baseline `mention:TS` identity remains stable across all automatic sources and durable dismissals survive cache clearing. Search completion waits for every enabled source; followed roots have separate completed windows, sharing gap warnings and identity bounds. Dense search/reply windows beyond ten pages remain explicitly incomplete and require retry; no complete-workspace inbox guarantee is made.

Assistant-run new-source evidence is offline; [#22](https://github.com/cliffom/siftpane/issues/22) tracks the live acceptance protocol. On 2026-11-15 the operator reported “verified” when requesting release of PR #23. This is recorded as operator confirmation of outstanding 0.6.0 live release acceptance; no case-by-case fixture log or timings were supplied to the assistant. Future controlled checks should create public/private exact channel/here broadcasts, follow a root locally, close the client, add replies from another author, reopen, verify exact opening, dismiss/restart, and add mixed-source new activity. Include restrictive saved search filters, nearby collapsed matches, access/grant revocation and gaps over seven days. Keep actual IDs/bodies private and record per-case outcomes without credentials. No new sends or live Slack probes were performed for 0.6 implementation.

The 0.6.0 operator confirmation satisfies the release gate without changing the documented bounded/search limitations or attributing live fixture results to assistant-run tests. The assistant performed no new live Slack calls, sends, membership/read-position writes or credential operations during release preparation. Issue/milestone bookkeeping remains separate from merge/tag/publication.
