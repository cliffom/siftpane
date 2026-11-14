# UI and UX guidelines

Siftpane is a calm, keyboard-driven window into conversations the user explicitly includes. Favor readable messages, predictable navigation, and clear recovery over chrome. These guidelines preserve the reviewed baseline; change them deliberately alongside an authorized design change.

## Screen hierarchy

- A single header row: `siftpane` at left; workspace name and authenticated `@username` right-aligned. Retain one blank row below it. No tagline, duplicate read-state/focus description, or padding row below the footer.
- A shared-edge main frame with a left sidebar titled **Messages**. Do not restore the redundant `INCLUDED #/#` row.
- **CHANNELS included/total** and **PEOPLE** use dashed horizontal separators. Count only public/private channel metadata already loaded for the denominator; never imply a complete workspace count before discovery finishes.
- A quiet **UNREAD HERE** group explains local dots and drafts. Do not imply Slack-synchronized counts or read positions.
- The message region has one muted context row for topic/purpose, conversation type, or known group size. Keep the actual conversation title prominent.
- The composer is inset, rounded, and solid on all four sides in every state. It has a `›` prompt, a clear title, and a bottom shortcut hint. Focus accents its frame; it does not change its border type.
- A concise status row and one contextual shortcut row finish the screen. Keep common actions here; put the complete guide behind `?`.

## Borders and color

The primary reference is Josh McKinney's [Terminal borders that don't cost a cell](https://www.joshka.net/cell-borders/), especially the style/joint gallery and rounded examples. Use its visual discipline with ordinary Unicode box drawing.

| Element | Treatment |
| --- | --- |
| Main panes, idle composer | Thin solid gray `#8693A5` |
| Active composer and transient frame | Teal `#8AC7BA` |
| Focused pane title | Bold amber `#E5C07B`, wrapped as `[Title]` |
| Selected conversation/message | Bold pale teal `#BFE4DA` and `▸` |
| Channel/people separators, picker/help outline | Dashed approximation `╌` |
| Local-activity grouping | Subtle dotted delimiters |

Use matching colors on adjoining strokes and corners. Style focused title text and surrounding border segments independently: nesting the title inside a border style previously leaked amber into a horizontal stroke. Focus changes only the intended title/frame; preserve the accepted border palette. Brackets and selection arrows must still communicate focus without color. Do not put `[FOCUS]` before a title.

Keep shared boundaries single and junctions aligned. Do not box individual messages. Reserve every fallback border row/column in layout calculations. Never emit native cell-border sequences without confirmed capability support and renderer integration; native support remains future work.

## Navigation and text safety

- Navigation and composing are separate modes. Preserve the [documented keys](USAGE.md), discoverability, and quick searchable switching.
- Retain main-message and thread drafts across switches and removal/re-inclusion. Never clear text on failure. Warn before discarding unsent text on exit, and keep that warning visible during background updates.
- Lock pending sends against duplicate submission. Keep pending, failed, and uncertain states visible. Never retry writes automatically or promise exactly-once delivery after an ambiguous failure.
- Removing a conversation changes only local inclusion. Cancel its reads and ignore late results; do not leave or modify Slack membership.
- Align the first visible message to its author where practical. Oversized messages remain scrollable and repeat the author with a continuation label. Keep text selectable; mouse capture stays off. Unsupported Slack content needs useful text and a Slack link where available.
- Keep error and empty states concise, actionable, and free of implementation details that don't help the person recover. No AI ranking or summarization.

## Responsive review

Below 76 columns, use a full-width focused view rather than squeezing two panes. Minimum supported layout is 40×14; review 100×30 and 60×20 at a minimum. Long titles, workspace names, topics, and usernames must truncate by display-cell width without losing focus brackets or colliding with another region.

Review messages, channels, composing, threads, picker, and help. Include long/multiline drafts, long messages, unread/draft markers, failed/uncertain sends, and navigation after resize. Snapshot dimension tests cannot establish font stroke continuity, terminal color behavior, clipboard support, or live Slack correctness; use a terminal for those checks and record the evidence separately.

## Project artwork

The [project icon](BRANDING.md) combines a retro terminal and original conversation bubbles, inspired by Bambustand. Use the saved PNG for project documentation and the Slack app avatar; do not substitute Slack’s mark for Siftpane’s identity.
