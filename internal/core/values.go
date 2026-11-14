package core

// ConversationKind identifies a conversation without changing its stable text value.
type ConversationKind string

// Conversation kinds retain the original directory/filter and serialization values.
const (
	ConversationChannel ConversationKind = "channel"
	ConversationPrivate ConversationKind = "private"
	ConversationDM      ConversationKind = "dm"
	ConversationGroup   ConversationKind = "group"
)

// DeliveryState is the lifecycle of an in-memory send attempt. The zero value is idle.
type DeliveryState string

// Delivery states preserve the original values; retries always require user input.
const (
	DeliveryPending DeliveryState = "pending"
	DeliveryFailed  DeliveryState = "failed"
	DeliverySent    DeliveryState = "sent"
)
