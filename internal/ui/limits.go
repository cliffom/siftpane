package ui

import "time"

// Session resource bounds; drafts are deliberately outside the timeline cache.
const (
	profileLookupLimit = 512             // Name lookups attempted per focused view/manual refresh.
	cacheLimit         = 300             // Messages per timeline.
	replyCursorHistory = 16              // Recent reply cursors for cycle detection.
	timelineLimit      = 16              // LRU timelines per session.
	selectionLimit     = 32              // Explicitly included conversations per workspace.
	refreshInterval    = 3 * time.Second // One round-robin read per tick.
	threadPollEvery    = 2               // Existing scheduler reserves every second tick for a thread.
)

// Input bounds are fixed editing limits, not Slack API limits or runtime settings.
const (
	composerRuneLimit      = 12000
	composerLineLimit      = 1000
	pickerRuneLimit        = 160
	pickerPromptColumns    = 3
	composerPromptColumns  = 2
	initialComposerColumns = 60
)
