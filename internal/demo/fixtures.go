package demo

import "strings"

type fixture struct {
	topic    string
	authors  []string
	tasks    []string
	recent   []string
	thread   [4]string
	activity string
}

func fixtures() map[string]fixture {
	team := []string{"Maya Chen", "Leo Park", "You", "Nora Ellis"}
	return map[string]fixture{
		"Cstudio": {
			topic: "Daily collaboration, fewer interruptions", authors: team,
			tasks: []string{"Keyboard navigation", "Draft recovery", "Thread view", "Empty states", "Message wrapping", "Shortcut guide", "Small terminal layout"},
			recent: lines(`Morning, team. A little room to focus today.
The new navigation is ready to try. Small details, fewer interruptions.
I like where this is going. Can we keep the shortcuts discoverable?
Yes — a visible focus label makes a big difference.
A note for @You: the empty states are in the design file.
Let's keep the first version deliberately small. Channels, people, and a good composer.
*Ready for review*: keyboard navigation, thread replies, and drafts that stay with you.
The spacing feels good at 80 columns, too.
Here's the checklist: <https://example.com/review|review notes>
No rush. Pick this up when you have a quiet moment.
How should the border hierarchy work on smaller terminals?
I'll check the composer at 60 columns after lunch.
The focused header is much easier to spot now.
Thanks, everyone. Let's leave some breathing room around the input.`),
			thread:   [4]string{"How should the border hierarchy work on smaller terminals?", "Solid outlines for the main panes. Dashed for the transient picker.", "And dotted grouping inside the sidebar. No boxes around individual messages.", "Agreed. Keep the messages at the center."},
			activity: "The small-terminal review is ready. No new blockers.",
		},
		"Cshipping": {
			topic: "Release checks, rollout notes, and small wins", authors: team,
			tasks: []string{"Release checklist", "macOS build", "Linux build", "Version notes", "Install instructions", "Smoke test", "Rollback notes"},
			recent: lines(`Release check-in: the candidate is built.
The macOS smoke test passed this morning.
Linux package names now match the installation guide.
I found one missing flag example in the README.
Fixed that example. It now shows the offline demo first.
Let's keep the release notes short and concrete.
The artifact checksums are ready for review.
One last check: do drafts survive switching to a thread?
Confirmed. Main-message and thread drafts stay separate.
The release checklist is at <https://example.com/releases|release notes>.
Should we ship the keyboard guide with this candidate?
Yes, it answers the most common setup questions.
I'll run the install flow from a clean directory.
Clean install passed. Ready for the next review.`),
			thread:   [4]string{"Should we ship the keyboard guide with this candidate?", "Yes. Include the compose and navigation modes.", "I'll check the examples against the actual keybindings.", "Reviewed. The guide matches this build."},
			activity: "Clean-install verification finished. The candidate is still green.",
		},
		"Cdesign": {
			topic: "Typography, spacing, and interaction details", authors: team,
			tasks: []string{"Color palette", "Header hierarchy", "Composer frame", "Unread legend", "Message spacing", "Dialog edges", "Compact layout"},
			recent: lines(`I put three composer sketches in the review file.
The inset version gives the input a clearer edge.
Can we keep the message list itself free of boxes?
Definitely. A little spacing is enough between messages.
The rounded corners should share the same stroke color.
The previous mockup mixed gray and teal at one corner.
I prefer brackets around the focused header.
Amber looks clear against the quieter pane outlines.
Let's check the same palette without relying on color.
Brackets and the selection arrow still carry the meaning.
How much room should the conversation context use?
One line. A topic or group size should be enough.
Updated the narrow layout with the shorter footer.
This feels calmer. The message text gets most of the room.`),
			thread:   [4]string{"How much room should the conversation context use?", "A single muted line above the first message.", "Truncate long topics instead of adding more chrome.", "Agreed. Keep the group size there when we know it."},
			activity: "Updated the spacing notes with the latest composer treatment.",
		},
		"Dmaya": {
			authors: []string{"Maya Chen", "You"},
			tasks:   []string{"Tuesday review", "Navigation sketch", "Notes for Leo", "Composer wording", "Feedback list", "Review time", "Next iteration"},
			recent: lines(`Hey, do you have ten minutes to look at the keyboard flow?
Yes — send me the bits you want a second pair of eyes on.
Mostly switching conversations without losing the draft.
I'll try a message and a thread reply, then switch back.
Both drafts stayed put. That's a good start.
Great. I also changed the empty-state wording.
It sounds more inviting now.
Would you keep the shortcut hints near the input?
Just the send and exit keys. The full list can live in help.
That matches what I was thinking.
Can we review the compact layout tomorrow morning?
Sure. I have a quiet half hour before stand-up.
I'll bring the 60-column example.
Perfect. Thanks for the thoughtful review.`),
			thread:   [4]string{"Can we review the compact layout tomorrow morning?", "I'll check wrapping and cursor visibility first.", "I'll handle the shortcut wording.", "Let's compare notes after the review."},
			activity: "Tomorrow's compact-layout review is on my list. See you then.",
		},
		"Dleo": {
			authors: []string{"Leo Park", "You"},
			tasks:   []string{"Build setup", "Test fixtures", "Retry case", "Cursor paging", "Request pacing", "Error wording", "Local read check"},
			recent: lines(`I reproduced the retry issue with the fake service.
Was the body getting cleared before the acknowledgement?
Yes. The pending state needs to retain the original text.
I'll keep the composer locked until the send finishes.
That also prevents repeated Ctrl+S from queueing duplicates.
Good catch. What about a timeout after Slack accepted it?
We should show delivery as unknown and require an explicit retry.
I'll add the warning beside the retained draft.
The API test now checks that a retry keeps the same client ID.
Nice. Can you cover a late fetch after conversation removal too?
Should the read watermark ever move backward?
No. Older cached pages shouldn't make newer messages unread again.
The regression case passes now.
Thanks. That closes the last item on my list.`),
			thread:   [4]string{"Should the read watermark ever move backward?", "I saw it happen while loading an older cached page.", "Keep the maximum observed timestamp when marking read.", "Fixed and checked with the older-page regression case."},
			activity: "The pagination regression checks are passing on my machine.",
		},
		"Gcrew": {
			topic: "Small launch review", authors: []string{"Maya Chen", "Leo Park", "You"},
			tasks: []string{"Review agenda", "Demo script", "Small-screen check", "Permission notes", "User-token setup", "Launch wording", "Open questions"},
			recent: lines(`Starting a small group for the first walkthrough.
I'll drive the demo so you can both focus on the rough edges.
Sounds good. Let's keep a list of concrete fixes.
First up: choose just a few conversations.
Then write a draft and switch to a DM.
I'll check the thread flow while you do that.
The input needs a stronger boundary than the message list.
An inset frame works without boxing every message.
Can we show who this conversation is with?
The header has our names. A group-size hint would help too.
What should we prioritize after this walkthrough?
The clipped first message and the noisy footer.
I'll take the viewport adjustment. Maya, could you review the labels?
Deal. Let's regroup once those changes are ready.`),
			thread:   [4]string{"What should we prioritize after this walkthrough?", "Keep the first visible author attached to their message.", "And cut the footer down to the shortcuts people use most.", "I'll work on those together, then rerun the small layout."},
			activity: "The walkthrough fixes are ready for one more look.",
		},
		"Cquiet": {
			topic: "Private space for deliberate work", authors: []string{"Nora Ellis", "You"},
			tasks: []string{"Focus block", "Reading notes", "Research links", "Open questions", "Weekly reflection", "Writing time", "Loose ends"},
			recent: lines(`I'm setting aside this channel for quieter work.
Good idea. I'll keep the longer notes here.
Today's focus block is for reading through the API docs.
I wrote down the scope questions before starting.
The distinction between user and bot tokens is clearer now.
Let's link the official references next to each decision.
The rate-limit notes are in the research file.
I'll check them after the current focus block.
No need for a meeting unless we find a blocker.
Agreed. A short written update should do.
Would a weekly reflection be useful here?
Yes — a few lines on what helped us focus.
This week, the smaller conversation selection helped most.
Same here. Fewer places to check, more time to finish things.`),
			thread:   [4]string{"Would a weekly reflection be useful here?", "Keep it optional and short.", "A couple of observations rather than another checklist.", "That sounds right. I'll try it on Friday."},
			activity: "Added a short reflection to the focus notes. Read it whenever suits you.",
		},
		"Crandom": {
			topic: "Coffee, walks, and things outside the terminal", authors: team,
			tasks: []string{"Coffee recommendations", "Walking route", "Weekend reading", "Lunch plan", "Photo walk", "Playlist swap", "Book exchange"},
			recent: lines(`Does anyone have a good afternoon coffee recommendation?
The little place near the park has a great decaf.
I tried it yesterday. Their pastries are good too.
Adding that to my next walking route.
Speaking of walks, the trees by the river are turning.
Perfect excuse to leave the desk before sunset.
I brought a camera last time and forgot to take any photos.
That sounds like a successful walk to me.
Any weekend reading plans?
A short novel and absolutely no API references.
Anyone up for a coffee walk on Friday?
Yes, as long as we go before it gets dark.
I'll meet you by the front door at three.
See you there. Bringing a reusable cup this time.`),
			thread:   [4]string{"Anyone up for a coffee walk on Friday?", "Three works for me.", "I'll take the route past the park.", "Count me in. See you by the front door."},
			activity: "Coffee walk still on for Friday. The forecast looks promising.",
		},
	}
}
func lines(s string) []string { return strings.Split(s, "\n") }
