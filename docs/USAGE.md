# Keyboard guide

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
| Outside help | `Ctrl+K`, delivered `Cmd+K` | Open conversation search; keep draft |
| Navigate | `/` | Open conversation search |
| Navigate | `p` | Searchable Kept chooser |
| Picker | `Ctrl+T` | Temporarily add and open without keeping |
| Navigate | `K` | Keep active conversation; retain attention |
| Navigate | `d` | Dismiss observed attention after draft/send recovery |
| Navigate | `n` | Open next attention trigger |
| Navigate | `M` | Enable mentions with a second consecutive `M`; disable an enabled source |
| Navigate | `B` / `H` twice | Enable independent channel/here broadcast search; one press disables an enabled source |
| Opened thread, navigating | `f` | Follow/unfollow future replies locally |
| Navigate | `Ctrl+B` twice | Enable disk content cache after disclosure; one press disables |
| Navigate | `Ctrl+X` | Clear account disk content cache, retain drafts |
| Navigate | `Ctrl+G` twice | Acknowledge known catch-up gaps without recovering missed history |
| Picker | `↑/↓`, `Ctrl+P/N` | Move |
| Search | `Tab` | Complete highlighted name; do not include or open |
| Kept chooser (`p`) | `Tab` | Keep/unkeep locally |
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
| Navigate | `?` | Open scrollable keyboard help modal |
| Anywhere | `F1` | Open help, retaining composing/picker context |
| Help | `↑/↓`, `j/k`, `PgUp/PgDn`, `End`/`G` | Scroll, page, or reach the end |
| Help | `Esc` or any other non-scroll key | Close; the key is consumed |
| Navigate | `q` | Quit; warns if there is unsent text |
| Outside help | `Ctrl+C` | Quit; a second press explicitly discards unsent text |

Unsent drafts survive conversation and thread switches and removal/re-inclusion during the session. Drafts and outbox attempts are **memory-only** and do not survive exit/crash. Message bodies also stay in memory unless you explicitly enable the separate [recent message cache](CONFIGURATION.md#optional-recent-message-cache). The composer stays locked while a send is pending, duplicate submissions are rejected, and its body is cleared only on a successful acknowledgement for that draft. Failed and pending states remain visible in the composer frame. A network timeout or server failure can mean Slack accepted a message without returning a response: inspect Slack before explicitly retrying. There are no automatic write retries, and exactly-once delivery is not promised. Retrying reuses the original body and client ID.

## Search

`Ctrl+K` opens a centered modal over the dimmed main screen from navigation or composing. `/` opens the same search from navigation. Use `@` for existing DMs/group DMs, `#` for accessible public/private channels, or no prefix for both. Matching ignores case and uses available channel names, display names, and DM handles. Search covers loaded conversation metadata, not the entire workspace user directory; it does not create DMs or join channels.

Suggestions update locally as you type. `↑/↓` or `Ctrl+P/N` highlights a result, `Tab` completes it, and `Enter` opens it. `[✓]` marks a Kept result; `[!]` marks an attention-only result; `[+]` marks an excluded result whose action reads **Enter include & open**. Searching, highlighting, and completion never start refresh for an excluded conversation. Only explicit `Enter` includes and opens it, subject to the existing inclusion limit. The separate `p` picker uses `Tab` to keep/unkeep; `Ctrl+T` in either picker temporarily adds and opens the highlighted result. An existing attention result opens without being automatically kept.

`Esc` dismisses search without modifying inclusion or sending text. It restores the prior navigation/composing focus, conversation/thread, and draft. Selecting a result retains the prior draft and opens the new conversation in navigation mode.

The modal reports directory coverage, loading, and errors. The initial metadata page loads at startup; `Ctrl+L` explicitly loads the next page, and `Ctrl+R` reloads the first page or retries an initial failure. Typing/completion makes no network requests. Existing suggestions remain available after a page failure; a failed next-page request retains its cursor for an explicit retry. Background refresh cannot overwrite the modal's directory error. Use the [terminal fallback](TERMINALS.md#command-k) if `Cmd+K` is intercepted.

## Help and local markers

Help opens over dimmed conversation context. Its single-column guide wraps to the terminal width; scroll to see all shortcuts and marker explanations. Paste is ignored while help is open. Closing restores the prior draft, focus and picker. Use `F1` while composing; `?` still inserts a question mark. The footer keeps a minimal help entry beside status rather than repeating hotkeys. Its bottom-left status bar shows message loading/refresh progress or saved-conversation restoration counts, then returns to the completed status. The message area stays clear while loading; failures retain actionable recovery text.

Sidebar trailing slots show `●` for locally observed unread activity and `✎` for a retained conversation draft. Long names truncate before these slots. These markers do not imply Slack-synchronized read state or persisted drafts.

## Needs Attention and Kept

Existing included conversations migrate to Kept. Explicit temporary additions and optional direct mentions appear in Needs Attention, with a reason beside each name. Repeated triggers group into one conversation row; a Kept conversation needing attention appears once. `n` opens the next attention entry, including the triggering reply's thread. Switching, reading, local marking read and replying never dismiss attention.

Use `K` to keep an active conversation and `d` to dismiss its observed triggers. A Kept conversation remains available after dismissal. An unkept entry is removed locally, without changing Slack membership/read state or sending anything. Dismissal refuses any retained draft or unresolved send across all that conversation's threads. Return to the affected target, inspect Slack for uncertain delivery, use existing retry/edit recovery, and explicitly clear or send the draft before dismissal. A temporary result can be reopened with a new `Ctrl+T` action after dismissal.

`M` twice opts into direct personal mentions from accessible unkept conversations using `search:read`; an intervening key cancels confirmation. The source starts disabled and its preference persists per account. One `M` disables it without removing entries. `R` retries discovery alongside current history. The footer shows checking, failure/retry, last successful check with limited coverage, or a catch-up gap. An empty list does not establish successful discovery. See [search limitations](SLACK.md#direct-mentions-and-coverage) before relying on it.

Known catch-up gaps persist across successful scans, source toggles and restarts. The warning summarizes the earliest-to-latest uncovered bounds; some intervening periods may have been checked. `Ctrl+G` twice acknowledges that warning without fetching the missing interval, advancing the checkpoint, dismissing attention or changing drafts. An intervening key cancels confirmation; key repeat cannot confirm. If gap bounds change before confirmation, review and confirm the updated warning again.

## Broadcasts, local follows and resume

`B` twice enables exact `@channel` broadcast search; `H` twice independently enables `@here`. One press disables that enabled source. Direct mentions (`M`) remain independent. Confirmation discloses accessible unkept search using the existing `search:read` grant; an intervening key cancels, and held-key repeats cannot enable/disable. These sources are off by default. They detect tokens, not Slack notification eligibility: `@here` may surface while you were absent and broadcasts can come from accessible unkept channels. `@everyone`, user groups, saved Slack notification settings and subscriptions are not mirrored.

Open a thread with `t`, then press `f` to follow future replies from others locally. Its context row shows local following and `f` to unfollow. A follow persists across restart and continues even after dismissing attention; explicitly unkeeping the conversation removes its local follows. Opening or replying alone never follows automatically. There are at most 32 local roots. Checks run at startup and at least three minutes apart per root through the existing paced replies API. Reconnect scans cover up to seven days after the last completed window; partial pages retain discovered identities without claiming completion. Longer absences show a durable gap. `R` retries due checks; errors, incomplete pagination and checking remain distinct from an empty attention list.

Automatic sources share conversation/message identities. Duplicate discovery never adds another row or revives a handled message. A duplicate pending message uses the strongest label (mention, channel, here, reply); `+N` counts other distinct pending triggers in that conversation. Reading and replying still never dismiss. Local follow scans create attention, without switching selection, focus or drafts or loading an unkept conversation timeline.

`Ctrl+B` twice enables local Slack content storage; `Ctrl+B` once disables it. `Ctrl+X` clears disk history while leaving the option enabled for future successful normal fetches. Defaults, bounds, account isolation and cleanup are in [configuration](CONFIGURATION.md#optional-recent-message-cache). Cached pages render immediately with cached/refreshing status, retain readable content after network failure, then reconcile with live results. They are display hints rather than proof of freshness/current access. Drafts, failed/pending sends and thread reply bodies are never restored from disk.


## Multiple workspaces and quick participation

Run `siftpane --login` for each authorized account. Use `--account WORKSPACE_ID/USER_ID` to choose startup identity, or `Ctrl+W` then arrows and Enter to switch. Esc closes without switching. Needs Attention and Kept show only the current account; each account retains its selected conversation/thread, selection, scroll and memory-only drafts/recovery. The header shows stable IDs as well as available display names. `--demo` offers two isolated offline accounts and in-memory reactions.

Inactive accounts pause all automatic reads and discovery. Returning resumes enabled-source bounded catch-up with the same pacing, overlap and page/time budgets; long absences or dense activity remain visibly incomplete or show gaps. Disabled sources stay disabled. No inactive counts are presented as live. Already accepted activity/dismissals survive canceled scans, and only complete intervals advance checkpoints. A scrolled view remains in place; return to newest with `G` or explicitly refresh with `R` for its live page.

A pending send can finish for its original account while another is active. Return to that account for failed/uncertain recovery; switching never resends. Exit protection checks unsent text in every visited account. Drafts and outbox remain memory-only; copy needed text before restarting or confirming exit.

From navigation, select a message and press `1` for Acknowledged (👍), `2` for Looking (👀), or `3` for Resolved (✅). Each toggles only your own matching reaction, after checking current Slack state. Reactions require the new grants and do not dismiss attention; `d` remains a separate local action. Pending toggles reject duplicates and never retry automatically. On a failure, inspect Slack before trying again because the mutation may have succeeded. Keys remain ordinary text while composing. Press `O` to open the exact selected message in your system's Slack/browser handler; this explicit action fetches its canonical permalink. It requires an available platform browser opener. Demo opening is disabled and tests never launch browsers.
