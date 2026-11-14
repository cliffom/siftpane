# Terminal compatibility

Design reference: Josh McKinney's [Terminal borders that don't cost a cell](https://www.joshka.net/cell-borders/), including the **Styles and joints** gallery and rounded solid/dashed/dotted examples. The implemented fallback reserves actual terminal rows/columns for its borders:

- One thin rounded outer frame, a shared sidebar edge, and an inset rounded composer frame with its own input prompt and shortcut footer; messages have no individual boxes.
- Solid primary outlines and composer in every state; solid search/include modals; dashed help and Channels/People separators; dotted local-activity grouping. Rounded corners use `╭╮╰╯`. Dashed/dotted strokes are Unicode approximations whose continuity depends on the font.
- Conversation context uses one muted line for the channel topic/purpose, DM type, or group size when Slack supplies it. The footer keeps common actions visible; `?` opens the full guide.
- History starts at a complete message header where possible. Oversized messages remain scrollable and repeat the author with a “continued” label when the viewport starts inside them.
- Below 76 columns, the sidebar becomes a focused full-width view. `Tab` toggles it with messages, and `/` switches directly. Minimum practical size is 40×14; 80×24 or larger is more comfortable. Below minimum, a resize prompt appears. Long text wraps by display-cell width.
- No patched terminal, experimental cell-border escapes, cell-border capability probing, or native cell-border support. Future support needs protocol detection and integration with Bubble Tea's renderer first.
- Mouse reporting is off, so normal terminal selection remains available. Unicode borders may enter copied selections; `y` copies only the selected message body with OSC52. OSC52 depends on terminal/multiplexer policy. Use the modifier your terminal requires for native selection.

Slack mentions, labeled links, HTML entities, timestamps, basic mrkdwn text, and attachment fallbacks are readable. Bold/italic/code delimiters are preserved as text. Readable text, rich-text blocks, and attachment previews do not get an extra Slack deep-link row. Unrendered content gets a useful textual fallback and Slack deep link where available; this is not a Block Kit renderer. Text controls and directional overrides from Slack are stripped before display. Emoji aliases may remain aliases.

## Command-K

`Ctrl+K` is the portable search binding. The pinned Bubble Tea v2 stack requests basic keyboard disambiguation by default and exposes Command/Windows as `ModSuper` (`super+k`). Siftpane handles that event from navigation and composing. See the upstream [modifier definitions](https://github.com/charmbracelet/bubbletea/blob/v2.1.1/mod.go) and [keyboard enhancement API](https://pkg.go.dev/charm.land/bubbletea/v2#KeyboardEnhancements).

`Cmd+K` works only when the terminal sends it to the application. Terminals may intercept it for actions such as clearing the screen, and legacy terminal input does not encode Command. Remap `Cmd+K` in your terminal's keyboard settings to send **Ctrl+K**, hexadecimal byte **0B** (`\x0b`), or use `Ctrl+K` directly. If your terminal can send Kitty keyboard events, the decoded Super modifier is supported; no blanket claim is made for every terminal/multiplexer. A delivered Kitty sequence was exercised through a PTY, not a physical Command-key press in each terminal.

The search/include picker is an opaque centered modal with a teal solid rounded frame and bracketed amber title. The main screen remains visible with its prior highlights removed, a dim gray foreground, and the faint attribute. In no-color terminals the inset frame, brackets, result arrow, inclusion markers, and explicit action still distinguish it; faint rendering depends on terminal support.
