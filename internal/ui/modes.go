package ui

// UI modes are internal state, separate from snapshot CLI identifiers.
type (
	focusPane  string
	pickerMode string
)

const (
	focusChannels focusPane = "channels"
	focusMessages focusPane = "messages"
	focusCompose  focusPane = "compose"

	pickerClosed  pickerMode = ""
	pickerInclude pickerMode = "pick"
	pickerSwitch  pickerMode = "switch"
)
