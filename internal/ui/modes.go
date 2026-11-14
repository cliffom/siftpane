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

// statusKind separates user-action feedback from routine transport progress.
type statusKind uint8

const (
	statusRoutine statusKind = iota
	statusAction
)
