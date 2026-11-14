# Siftpane

**Your channels. Your people. Your terminal.**

A focused, keyboard-driven Slack terminal client written in Go with Bubble Tea v2. Choose channels, DMs, and group DMs explicitly. Only included conversations are refreshed. No AI, ranking, summaries, workspace history export, or automatic inclusion.

This is an initial working vertical slice for one workspace and a user-created **internal Slack app**. Demo mode is entirely offline. The live adapter is implemented and covered with fake HTTP tests; authentication, app installation, and messaging against a real workspace have **not yet been verified with live credentials**.

## Run

Requires Go **1.26.7 or newer** (dependencies require 1.26.7; developed with Go 1.27.4).

```sh
go build -o bin/siftpane ./cmd/siftpane
./bin/siftpane --demo
```

Or install from this checkout:

```sh
go install ./cmd/siftpane
siftpane --demo
```

The offline Fieldwork workspace has distinct histories for public/private channels, DMs, and a group DM, with conversation-specific participants, topics, thread discussions, simulated updates, unread indicators, and older pages. Start with `Tab` to focus messages, `k` three times to select the question with three replies, then `t` to open it. Use `i` to compose, `Enter` for newlines, `Ctrl+S` to send, and `Esc` to navigate. Send text containing `/fail` to simulate a definite failure; `Esc`, then `r` retries successfully. Demo selection and read state are deliberately ephemeral and never touch your real preferences or keyring.

Review reproducible layouts without starting a TUI:

```sh
./bin/siftpane --snapshot messages --width 100 --height 30
./bin/siftpane --snapshot compose --width 60 --height 20
./bin/siftpane --snapshot picker --width 80 --height 24
./bin/siftpane --snapshot thread --width 100 --height 30
./bin/siftpane --snapshot help --width 100 --height 30
```

## Set up Slack

1. Open [Your Slack apps](https://api.slack.com/apps), choose **Create New App → From a manifest**, and select your workspace. Paste [slack-app-manifest.yaml](slack-app-manifest.yaml) into the YAML tab. Review and create the app. Keep this app internal to your workspace; do not enable public distribution.
2. In **OAuth & Permissions**, verify the **User Token Scopes** below. No bot scopes are needed. Install the app to the workspace, approve the requested access, and complete any administrator approval required by your workspace. If you change scopes later, reinstall the app.
3. Copy the **User OAuth Token**, beginning `xoxp-`, from OAuth & Permissions. This is the documented app-installation token, not a browser cookie, desktop session, bot token, or app-level token. There is no embedded client secret or hosted OAuth callback in this version. Installation in Slack's app dashboard provides the user grant for this personal internal app.
4. Run `./bin/siftpane --login`, paste the token into the hidden prompt, and press Enter. Siftpane validates it with `auth.test` and saves it in the OS credential store. Then run `./bin/siftpane`.
5. Press `p` to choose conversations. Type to filter the loaded directory page, use arrows to move, `Tab` to include/remove, and `Enter` to include and open. Use `Ctrl+L` to load another directory page; search covers only pages loaded in this session. A new installation includes **nothing** until you choose it. The sidebar’s `CHANNELS included/total` count covers public/private channel entries loaded so far; load more picker pages to discover additional channels.

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

## Keys

Navigation and composing are separate modes. The app name and right-aligned workspace plus authenticated `@username` share one header row. Focused pane headers are bold amber, and bracketed pane headers (for example, `[#studio]`) and `▸` reinforce color.

| Context | Key | Action |
| --- | --- | --- |
| Navigate | `Tab` | Switch channels/messages focus |
| Channels | `j/k`, `↓/↑` | Switch included conversations |
| Messages | `j/k`, `↓/↑` | Select a message; scroll it into view |
| Messages | `PgUp/PgDn` | Scroll text |
| Navigate | `G`, `End` | Newest messages and mark read locally |
| Navigate | `i`, `Enter` | Compose |
| Compose | `Ctrl+S` | Send (turn off terminal XON/XOFF with `stty -ixon` if intercepted) |
| Compose | `Enter` | Newline |
| Compose | `Esc` | Keep draft, return to navigation |
| Anywhere | `Ctrl+K` | Search included conversations; keep draft |
| Navigate | `/` | Search included conversations |
| Navigate | `p` | Searchable include/remove picker |
| Picker | `↑/↓`, `Ctrl+P/N` | Move |
| Picker | `Tab` | Include/remove locally |
| Picker | `Enter` | Include and open |
| Picker | `Ctrl+L` | Load next directory page |
| Picker | `Ctrl+R` | Reload first directory page |
| Picker | `Esc` | Close |
| Navigate | `t` | Open selected message's thread |
| Thread, navigating | `Esc` | Return to conversation, retain thread draft |
| Navigate | `o` | Load older history; in threads, load next reply page |
| Navigate | `R` | Refresh current conversation/thread |
| Navigate | `m` | Mark locally observed messages read |
| Navigate | `r` | Retry failed send; uncertain delivery requires a second explicit `r` |
| Navigate | `x` | Restore failed body for editing; retains any newer composed text |
| Navigate | `y` | Copy selected message body through OSC52, if supported |
| Navigate | `?` | Contextual key/border guide; any key closes |
| Navigate | `q` | Quit; warns if there is unsent text |
| Anywhere | `Ctrl+C` | Quit; a second press explicitly discards unsent text |

Unsent drafts survive conversation and thread switches and removal/re-inclusion during the session. Drafts and message bodies are **memory-only** and do not survive exit/crash. The composer stays locked while a send is pending, duplicate submissions are rejected, and its body is cleared only on a successful acknowledgement for that draft. Failed and pending states remain visible in the composer frame. A network timeout or server failure can mean Slack accepted a message without returning a response: inspect Slack before explicitly retrying. There are no automatic write retries, and exactly-once delivery is not promised. Retrying reuses the original body and client ID.

## Refresh, unread, and limits

This version intentionally uses the documented Web API instead of Socket Mode. Slack's [Events API permission model](https://docs.slack.dev/apis/events-api/) and [`message.channels`](https://docs.slack.dev/reference/events/message.channels/), [`message.groups`](https://docs.slack.dev/reference/events/message.groups/), [`message.im`](https://docs.slack.dev/reference/events/message.im/), and [`message.mpim`](https://docs.slack.dev/reference/events/message.mpim/) event families can expose visible conversation activity with corresponding history scopes and **user/workspace** subscriptions. Bot subscriptions describe the bot's visibility, not every conversation visible to its installer. Public history access also does not imply membership event coverage. Subscriptions are app/workspace-level, not dynamically narrowed to the local inclusion list.

[Socket Mode](https://docs.slack.dev/apis/events-api/using-socket-mode/) changes the delivery transport; it does not broaden authorization, recover historical gaps, or promise a complete user-client stream. It needs an additional `xapp-` app-level token with [`connections:write`](https://docs.slack.dev/reference/scopes/connections.write/), app configuration, acknowledgements, and reconnect handling. None are required or requested by this implementation. The manifest has Socket Mode off and no event subscriptions. Event integration is a follow-up; current updates are polling, **not real-time events**.

- Every three seconds, request at most one timeline, round-robin across included conversations. When a thread is open, every other tick refreshes it. With N selected conversations, conversation refresh is roughly 3N seconds, or 6N while a thread is open, plus network, author lookup, and backoff delays. At most one request per timeline is in flight. No automatic history fetching for excluded conversations.
- The active timeline's background refresh pauses while scrolling or selecting older messages. `G` resumes at newest; `R` explicitly refreshes. Inactive selected conversations continue polling. Notifications are quiet sidebar dots only; no desktop notifications or terminal bells.
- Reads fetch 50 messages per page. Older history and additional thread replies load only on demand. Recent history is reconciled for edits/deletions inside the returned window. Once a thread's loaded pages are complete, refresh asks for replies after the last loaded timestamp. Old edits/deletions and inactive-thread replies are not comprehensively tracked. High-volume bursts can exceed the newest 50-message window: this client is not an archive or guaranteed delivery monitor.
- The “Unread here” legend labels `●` as **locally observed** activity newer than the saved local read timestamp. On first observation, establish a baseline rather than label the entire channel unread. Opening a conversation, reaching newest, or `m` marks observed activity read locally. No `conversations.mark` call, Slack read synchronization, exact unread counts, or cross-client notification synchronization. Dots can lag, miss thread-only activity, or overreport after your own sends on another client.
- Up to 32 explicitly included conversations. Cache: at most 16 timelines, 300 messages each; author cache at most 512 names. Fetches are lazy and the directory loads in pages of 30. Directory metadata remains in memory for loaded pages. Loading older messages shifts the retained window; returning to newest may drop older cached pages. Drafts are separate from cache eviction.
- API method starts are paced at 1.3 seconds apart (directory listing 3.1 seconds, user lookup 650ms). HTTP `429` enforces `Retry-After` for the method across manual and background requests; it is surfaced immediately and later reads resume after the delay. Network reads reconnect through subsequent polling; writes require explicit retry. Cancellation on removal discards late fetch results; an already accepted send cannot be undone by removal.
- Slack's current [rate-limit clarification](https://docs.slack.dev/changelog/2025/05/29/rate-limit-changes-for-non-marketplace-apps/) keeps **internal customer-built apps** at Tier 3 (50+ requests/minute) for history/replies. Commercially distributed non-Marketplace apps have different limits, including 1/minute and 15 objects per request. This app targets the former; server `429` responses remain authoritative. See [general rate limits](https://docs.slack.dev/apis/web-api/rate-limits/). It is not configured for commercial distribution.

## Configuration and privacy

Preferences use `os.UserConfigDir()/siftpane/config.json`: normally `~/Library/Application Support/siftpane/config.json` on macOS, `$XDG_CONFIG_HOME/siftpane/config.json` or `~/.config/siftpane/config.json` on Linux, and `%AppData%\siftpane\config.json` on Windows. Override with `--config PATH`. See [example configuration](examples/config.json); do not copy placeholder IDs unchanged.

Inclusion and local-read maps are keyed by **workspace ID then conversation ID** so saved preferences cannot bleed between workspaces. The application still uses one workspace at a time. Writes use a private temporary file and rename; tokens and bodies never enter this file. There are no default application logs. A corrupt configuration is reported rather than overwritten. Use one Siftpane process per config file; concurrent writers are not coordinated.

## Terminal appearance

Design reference: Josh McKinney's [Terminal borders that don't cost a cell](https://www.joshka.net/cell-borders/), including the **Styles and joints** gallery and rounded solid/dashed/dotted examples. The implemented fallback reserves actual terminal rows/columns for its borders:

- One thin rounded outer frame, a shared sidebar edge, and an inset rounded composer frame with its own input prompt and shortcut footer; messages have no individual boxes.
- Solid primary outlines and composer in every state; dashed transient picker/help and Channels/People separators; dotted local-activity grouping. Rounded corners use `╭╮╰╯`. Dashed/dotted strokes are Unicode approximations whose continuity depends on the font.
- Conversation context uses one muted line for the channel topic/purpose, DM type, or group size when Slack supplies it. The footer keeps common actions visible; `?` opens the full guide.
- History starts at a complete message header where possible. Oversized messages remain scrollable and repeat the author with a “continued” label when the viewport starts inside them.
- Below 76 columns, the sidebar becomes a focused full-width view. `Tab` toggles it with messages, and `/` switches directly. Minimum practical size is 40×14; 80×24 or larger is more comfortable. Below minimum, a resize prompt appears. Long text wraps by display-cell width.
- No patched terminal, experimental cell-border escapes, capability probing, or native cell-border support. Future support needs protocol detection and integration with Bubble Tea's renderer first.
- Mouse reporting is off, so normal terminal selection remains available. Unicode borders may enter copied selections; `y` copies only the selected message body with OSC52. OSC52 depends on terminal/multiplexer policy. Use the modifier your terminal requires for native selection.

Slack mentions, labeled links, HTML entities, timestamps, basic mrkdwn text, and attachment fallbacks are readable. Bold/italic/code delimiters are preserved as text. Files, blocks, and unsupported message subtypes get a useful textual fallback and Slack deep link where available; this is not a Block Kit renderer. Text controls and directional overrides from Slack are stripped before display. Emoji aliases may remain aliases.

## Development and verification

```sh
gofmt -w cmd internal
go vet ./...
go test -race ./...
go build -o bin/siftpane ./cmd/siftpane
# or: make check
```

Tests use fixtures and a local HTTP server, never live credentials. They cover explicit filtering, workspace-specific persistence/removal, overlapping pagination, per-method rate-limit backoff, credential errors, send ambiguity and retry identity, draft preservation, pending paste protection, removal cancellation/late results, cache bounds, local unread, and dimensions/focus/border junctions at 120×36, 100×30, 80×24, 60×20, and 40×14. Snapshot views exercise messages, composing, picker, threads, and help.

See [architecture](docs/ARCHITECTURE.md), [verification record](docs/VERIFICATION.md), and [prioritized follow-ups](docs/BACKLOG.md). Live verification still needs an installed internal app and authorized user token. Workspace app approval and user-scope policy may prevent installation; that is a deployment constraint, not bypassed by this client.
