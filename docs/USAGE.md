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

