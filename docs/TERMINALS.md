# Terminal compatibility

Design reference: Josh McKinney's [Terminal borders that don't cost a cell](https://www.joshka.net/cell-borders/), including the **Styles and joints** gallery and rounded solid/dashed/dotted examples. The implemented fallback reserves actual terminal rows/columns for its borders:

- One thin rounded outer frame, a shared sidebar edge, and an inset rounded composer frame with its own input prompt and shortcut footer; messages have no individual boxes.
- Solid primary outlines and composer in every state; dashed transient picker/help and Channels/People separators; dotted local-activity grouping. Rounded corners use `╭╮╰╯`. Dashed/dotted strokes are Unicode approximations whose continuity depends on the font.
- Conversation context uses one muted line for the channel topic/purpose, DM type, or group size when Slack supplies it. The footer keeps common actions visible; `?` opens the full guide.
- History starts at a complete message header where possible. Oversized messages remain scrollable and repeat the author with a “continued” label when the viewport starts inside them.
- Below 76 columns, the sidebar becomes a focused full-width view. `Tab` toggles it with messages, and `/` switches directly. Minimum practical size is 40×14; 80×24 or larger is more comfortable. Below minimum, a resize prompt appears. Long text wraps by display-cell width.
- No patched terminal, experimental cell-border escapes, capability probing, or native cell-border support. Future support needs protocol detection and integration with Bubble Tea's renderer first.
- Mouse reporting is off, so normal terminal selection remains available. Unicode borders may enter copied selections; `y` copies only the selected message body with OSC52. OSC52 depends on terminal/multiplexer policy. Use the modifier your terminal requires for native selection.

Slack mentions, labeled links, HTML entities, timestamps, basic mrkdwn text, and attachment fallbacks are readable. Bold/italic/code delimiters are preserved as text. Files, blocks, and unsupported message subtypes get a useful textual fallback and Slack deep link where available; this is not a Block Kit renderer. Text controls and directional overrides from Slack are stripped before display. Emoji aliases may remain aliases.
