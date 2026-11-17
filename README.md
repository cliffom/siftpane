![Siftpane — Your channels. Your people. Your terminal.](assets/branding/hero.png)

# Siftpane

A focused Slack client for your terminal. Choose the channels, DMs, and group DMs you want to keep close, then read and reply with the keyboard. Keep conversations close or bring them in temporarily, then dismiss handled attention. Optional mention and broadcast discovery can surface accessible conversations outside Kept; locally followed threads recover later replies.

## Try it

The 0.9 download target is **Apple silicon, macOS 26+**, with Developer ID signing/notarization support. Download the signed, notarized disk image from the [0.9.0 release](https://github.com/cliffom/siftpane/releases/tag/v0.9.0); see [installation and upgrades](docs/INSTALLATION.md).

For source builds, use Go **1.26.7 or newer**. From this checkout:

```sh
go build -o bin/siftpane ./cmd/siftpane
./bin/siftpane --demo
./bin/siftpane --version
```

The demo runs offline without credentials. It includes distinct conversations, unread activity, older messages, threads, and a simulated send failure you can try by sending `/fail`.

To install the command on your Go binary path:

```sh
go install ./cmd/siftpane
siftpane --demo
```

## Connect Slack

Connect one or more accounts through your own internal Slack apps. Repeat `--login` for each account and use `Ctrl+W` to switch.

1. Follow the [Slack setup guide](docs/SLACK.md) to create and install the app using the supplied [manifest](slack-app-manifest.yaml).
2. Run `./bin/siftpane --login` and paste your **User OAuth Token** into the hidden prompt. Siftpane saves it in your OS credential store.
3. Run `./bin/siftpane`, press `p`, and choose which conversations to keep. Nothing is kept automatically. Use `Ctrl+T` in the picker for a temporary conversation.

Read-only live mention discovery and exact thread opening have been exercised, and the operator confirmed live acceptance for 0.5.0 and 0.6.0. See the [verification record](docs/VERIFICATION.md) for the evidence and remaining coverage limitations.

## Find your way around

| Key | Action |
| --- | --- |
| `v` | Inspect refresh ages, errors and recovery details |
| Navigation `Ctrl+R` | Reload this account’s saved credential after `--login --account …` elsewhere |
| `Ctrl+W` | Switch workspace/account; inactive accounts pause refresh |
| `1` / `2` / `3` | Toggle your Acknowledged / Looking / Resolved reaction |
| `O` | Open the selected message in Slack |
| `p` | Choose conversations; `Tab` keeps or unkeeps an item |
| Picker `Ctrl+T` | Temporarily add and open |
| `K` / `d` / `n` | Keep/unkeep / dismiss attention / next attention |
| `M` twice | Enable optional direct mentions; one `M` disables |
| `B` / `H` twice | Enable independent channel/here broadcast search |
| Opened thread `f` | Follow/unfollow future replies locally |
| `Ctrl+B` twice / `Ctrl+X` | Opt into recent disk history / clear disk history |
| `/` or `Ctrl+K` | Search existing conversations by `@person` or `#channel`; `Tab` completes |
| `Tab` | Switch sidebar/message focus |
| `j/k` or arrows | Move through conversations or messages |
| `i` | Start composing |
| `Ctrl+S` | Send; `Enter` inserts a newline |
| `Esc` | Keep your draft and return to navigation |
| Messages `→`, `Enter` | Open the selected message’s existing thread |
| `t` | Open any selected message’s thread |
| Thread `←`, `Esc` | Return to the conversation |
| `o` | Load older messages or the next thread page |
| `?` / `F1` | Open scrollable keyboard help (`F1` also works while composing) |
| `q` | Quit; warns if you have unsent text |

Search opens over a dimmed main view. Paste a Slack channel link or conversation ID and press `Enter` to open it directly. `Enter` explicitly includes and opens an excluded result; typing or completing a name changes no inclusion. Supported terminals can also deliver `Cmd+K`; see [terminal compatibility](docs/TERMINALS.md#command-k).

Drafts stay with their conversation or thread while you switch, but remain in memory and do not survive exit. Failed sends keep their text. See the [full keyboard and retry guide](docs/USAGE.md).

## What to expect

- Updates arrive through bounded polling, so activity may appear after a delay. Unread dots and the `Siftpane *` window title mean **unread here**, independently of Slack's read position. The title depends on [terminal/multiplexer support](docs/TERMINALS.md#unread-window-title).
- Dismissing an unkept attention entry stops its local refresh without leaving it in Slack. Drafts and unresolved sends must be recovered first.
- Preferences are saved in your [platform's configuration directory](docs/CONFIGURATION.md). Channel, people and resolved author names are cached separately for faster startup. Messages appear before remaining names finish loading. Handled attention identities persist separately so replay does not resurrect dismissed activity. Tokens and message bodies never go into preferences or the metadata cache. Recent Kept history can be cached separately with explicit opt-in; see [privacy and bounds](docs/CONFIGURATION.md#optional-recent-message-cache).
- Ordinary terminals work without patches. Rounded Unicode frames, bracketed amber focus titles, and a compact layout keep messages readable. See [terminal compatibility](docs/TERMINALS.md).
- This version handles text messaging and threads. It does not include calls, huddles, file uploads, AI features, or a combined cross-workspace inbox.

The [Slack guide](docs/SLACK.md) explains permissions, rate limits, credential recovery, and polling limitations.

## Contribute

Start with the [development guide](docs/DEVELOPMENT.md). Run `make fmt` to format changes and `make check` before submitting them. Tool versions are pinned; no separate linter installation is needed.

[Architecture](docs/ARCHITECTURE.md) · [UI and UX guidelines](docs/DESIGN.md) · [Verification record](docs/VERIFICATION.md) · [Follow-up backlog](docs/BACKLOG.md) · [Changelog](CHANGELOG.md) · [Release guide](docs/RELEASING.md)
