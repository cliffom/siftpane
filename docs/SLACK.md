# Slack setup

1. Open [Your Slack apps](https://api.slack.com/apps), choose **Create New App → From a manifest**, and select your workspace. Paste [slack-app-manifest.yaml](../slack-app-manifest.yaml) into the YAML tab. Review and create the app. Keep this app internal to your workspace; do not enable public distribution.
2. In **OAuth & Permissions**, verify the **User Token Scopes** below. No bot scopes are needed. Install the app to the workspace, approve the requested access, and complete any administrator approval required by your workspace. If you change scopes later, reinstall the app.
3. Copy the **User OAuth Token**, beginning `xoxp-`, from OAuth & Permissions. This is the documented app-installation token, not a browser cookie, desktop session, bot token, or app-level token. There is no embedded client secret or hosted OAuth callback in this version. Installation in Slack's app dashboard provides the user grant for this personal internal app.
4. Run `./bin/siftpane --login`, paste the token into the hidden prompt, and press Enter. Siftpane validates it with `auth.test` and saves it in the OS credential store. Then run `./bin/siftpane`.
5. Press `p` to choose conversations. Type to filter the loaded directory page, use arrows to move, `Tab` to include/remove, and `Enter` to include and open. Use `Ctrl+L` to load another directory page; search covers only pages loaded in this session. A new installation includes **nothing** until you choose it. The sidebar’s `CHANNELS included/total` count covers public/private channel entries loaded so far; load more picker pages to discover additional channels.

Upload the [project icon](../assets/branding/icon.png) under **Basic Information → Display Information** to use the same artwork for your Slack app. See [branding assets](BRANDING.md) for dimensions and the generation record.

An environment variable is an explicit fallback when the OS credential store is unavailable:

```sh
# Supply SIFTPANE_SLACK_TOKEN through your shell's secure secret mechanism.
./bin/siftpane
```

`SIFTPANE_SLACK_TOKEN` takes precedence over the keyring. Avoid pasting tokens into shell history. `--logout` deletes the saved keyring entry; it does not revoke the token at Slack or remove an environment override. Revoke the app/token in Slack if necessary. Tokens with rotation enabled are not supported: the supplied manifest leaves rotation disabled. Expired/revoked credentials stop automatic refresh and leave drafts in memory; reinstall/replace the token and restart. Copy any drafts before restarting.

OS storage uses macOS Keychain, Windows Credential Manager, or Linux Secret Service. A locked/missing Linux secret service causes `--login` to fail clearly, without silently writing a plaintext token file. Siftpane stores one active credential; switching workspaces is not a supported UI feature.

### Permissions

All ten scopes are **user** scopes:

| Scope | Implemented use |
| --- | --- |
| `channels:read`, `groups:read`, `im:read`, `mpim:read` | `conversations.list` for the picker and `conversations.info` to restore saved selections |
| `channels:history`, `groups:history`, `im:history`, `mpim:history` | `conversations.history` and `conversations.replies` |
| `users:read` | Lazy `users.info` for DM labels, authors, and mentions; no workspace user dump |
| `chat:write` | `chat.postMessage` as the authenticated user, including `thread_ts` replies |

`auth.test` identifies the workspace and user. User-token writes act as that user. History access covers public channels and private conversations the user can access. Posting still depends on membership, channel restrictions, and workspace policy; Siftpane never joins a channel for you. The picker lists existing accessible conversations; starting new DMs or joining/leaving channels is outside this version. Group DMs use their Slack-provided name.

The implementation follows Slack's official [user-token model](https://docs.slack.dev/authentication/tokens/), [OAuth installation flow](https://docs.slack.dev/authentication/installing-with-oauth/), [listing](https://docs.slack.dev/reference/methods/conversations.list/), [history](https://docs.slack.dev/reference/methods/conversations.history/), [thread replies](https://docs.slack.dev/reference/methods/conversations.replies/), and [posting](https://docs.slack.dev/reference/methods/chat.postMessage/) documentation. No undocumented session APIs are used. The stable `client_msg_id` field is supplied as a best-effort duplicate identifier; correctness does not assume Slack guarantees exactly-once delivery.


## Refresh, unread, and limits

This version intentionally uses the documented Web API instead of Socket Mode. Slack's [Events API permission model](https://docs.slack.dev/apis/events-api/) and [`message.channels`](https://docs.slack.dev/reference/events/message.channels/), [`message.groups`](https://docs.slack.dev/reference/events/message.groups/), [`message.im`](https://docs.slack.dev/reference/events/message.im/), and [`message.mpim`](https://docs.slack.dev/reference/events/message.mpim/) event families can expose visible conversation activity with corresponding history scopes and **user/workspace** subscriptions. Bot subscriptions describe the bot's visibility, not every conversation visible to its installer. Public history access also does not imply membership event coverage. Subscriptions are app/workspace-level, not dynamically narrowed to the local inclusion list.

[Socket Mode](https://docs.slack.dev/apis/events-api/using-socket-mode/) changes the delivery transport; it does not broaden authorization, recover historical gaps, or promise a complete user-client stream. It needs an additional `xapp-` app-level token with [`connections:write`](https://docs.slack.dev/reference/scopes/connections.write/), app configuration, acknowledgements, and reconnect handling. None are required or requested by this implementation. The manifest has Socket Mode off and no event subscriptions. Event integration is a follow-up; current updates are polling, **not real-time events**.

- Every three seconds, request at most one timeline, round-robin across included conversations. When a thread is open, every other tick refreshes it without advancing the next conversation's turn. With N selected conversations, conversation refresh is roughly 3N seconds, or 6N while a thread is open, plus network, author lookup, and backoff delays. At most one request per timeline is in flight. No automatic history fetching for excluded conversations.
- The active timeline's background refresh pauses while scrolling or selecting older messages. `G` resumes at newest; `R` explicitly refreshes. Inactive selected conversations continue polling. Notifications are quiet sidebar dots only; no desktop notifications or terminal bells.
- Reads fetch 50 messages per page. Older history and additional thread replies load only on demand. Recent history is reconciled for edits/deletions inside the returned window. Once a thread's loaded pages are complete, refresh asks for replies after the last loaded timestamp. Old edits/deletions and inactive-thread replies are not comprehensively tracked. High-volume bursts can exceed the newest 50-message window: this client is not an archive or guaranteed delivery monitor.
- The “Unread here” legend labels `●` as **locally observed** activity newer than the saved local read timestamp. On first observation, establish a baseline rather than label the entire channel unread. Opening a conversation, reaching newest, or `m` marks observed activity read locally. No `conversations.mark` call, Slack read synchronization, exact unread counts, or cross-client notification synchronization. Dots can lag, miss thread-only activity, or overreport after your own sends on another client.
- Up to 32 explicitly included conversations. Cache: at most 16 timelines, 300 messages each; author cache at most 512 names. Fetches are lazy and the directory loads in pages of 30. Directory metadata remains in memory for loaded pages. Loading older messages shifts the retained window; returning to newest may drop older cached pages. Drafts are separate from cache eviction.
- API method starts are paced at 1.3 seconds apart (directory listing 3.1 seconds, user lookup 650ms). HTTP `429` enforces `Retry-After` for the method across manual and background requests; it is surfaced immediately and later reads resume after the delay. Network reads reconnect through subsequent polling; writes require explicit retry. Cancellation on removal discards late fetch results; an already accepted send cannot be undone by removal.
- Slack's current [rate-limit clarification](https://docs.slack.dev/changelog/2025/05/29/rate-limit-changes-for-non-marketplace-apps/) keeps **internal customer-built apps** at Tier 3 (50+ requests/minute) for history/replies. Commercially distributed non-Marketplace apps have different limits, including 1/minute and 15 objects per request. This app targets the former; server `429` responses remain authoritative. See [general rate limits](https://docs.slack.dev/apis/web-api/rate-limits/). It is not configured for commercial distribution.

Search reads only loaded conversation metadata from the existing [`conversations.list`](https://docs.slack.dev/reference/methods/conversations.list/) pages. DM display names and handles come from the already-used [`users.info`](https://docs.slack.dev/reference/methods/users.info/) response (`profile.display_name`, `real_name`, and `name`) and share its bounded, paced cache. `@` includes existing accessible DMs and group DMs; it does not scan `users.list` or call `conversations.open`. No scopes, request page sizes, or pacing intervals change. Official method documentation was checked on 2026-11-14 for this metadata use.
