package ui

import (
	"strings"

	"github.com/cliffom/siftpane/internal/core"
)

// Key identifiers use Bubble Tea names; presentation labels derive from them.
const (
	keyReloadCredential = "ctrl+r" // Navigation; pickers retain directory reload.
	keyRefreshDetails   = "v"
	keyAccounts         = "ctrl+w"
	keyAcknowledged     = "1"
	keyLooking          = "2"
	keyResolved         = "3"
	keyOpenSlack        = "O"
	keyChannel          = "B"
	keyHere             = "H"
	keyFollow           = "f"
	keyMessageCache     = "ctrl+b"
	keyClearCache       = "ctrl+x"
	keyAcknowledgeGap   = "ctrl+g"
	keyTemporary        = "ctrl+t"
	keyKeep             = "K"
	keyDismiss          = "d"
	keyAttention        = "n"
	keyMentions         = "M"
	keyQuit             = "ctrl+c"
	keySend             = "ctrl+s"
	keySwitch           = "ctrl+k"
	keyCommandSwitch    = "super+k"
	keyPickerDown       = "ctrl+n"
	keyPickerUp         = "ctrl+p"
	keyDirectoryNext    = "ctrl+l"
	keyDirectoryReload  = "ctrl+r"
	keyEscape           = "esc"
	keyFocus            = "tab"
	keyCompose          = "enter"
	keyDown             = "down"
	keyUp               = "up"
	keyPageUp           = "pgup"
	keyPageDown         = "pgdown"
	keyNewest           = "end"
	keyQuitQuiet        = "q"
	keyHelp             = "?"
	keyHelpGlobal       = "f1"
	keyInclude          = "p"
	keySearch           = "/"
	keyWrite            = "i"
	keyNext             = "j"
	keyPrevious         = "k"
	keyLast             = "G"
	keyOlder            = "o"
	keyRefresh          = "R"
	keyRead             = "m"
	keyThread           = "t"
	keyCopy             = "y"
	keyRetry            = "r"
	keyEditFailed       = "x"
)

func keyLabel(k string) string {
	if strings.HasPrefix(k, "ctrl+") {
		return "Ctrl+" + strings.ToUpper(strings.TrimPrefix(k, "ctrl+"))
	}
	switch k {
	case keyHelpGlobal:
		return "F1"
	case keyEscape:
		return "Esc"
	case keyFocus:
		return "Tab"
	case keyCompose:
		return "Enter"
	case keyPageUp:
		return "PgUp"
	case keyPageDown:
		return "PgDn"
	case keyNewest:
		return "End"
	case keyUp:
		return "↑"
	case keyDown:
		return "↓"
	}
	return k
}

func isSearchKey(k string) bool { return k == keySwitch || k == keyCommandSwitch }

func broadcastReason(k string) core.AttentionReason {
	switch k {
	case keyChannel:
		return core.AttentionChannel
	case keyHere:
		return core.AttentionHere
	default:
		return ""
	}
}
