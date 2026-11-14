// Package core defines the small, transport-independent application boundary.
package core

import (
	"context"
	"errors"
	"sort"
	"strings"
)

type (
	// Identity is the workspace and user authenticated by the active credential.
	Identity struct{ Workspace, WorkspaceName, User, Username string }
	// Conversation describes an accessible Slack channel or direct conversation.
	Conversation struct {
		ID, Name, Kind string
		Topic, Purpose string
		Members        int
	}
)

// Label returns the terminal label, including its channel or person prefix.
func (c Conversation) Label() string {
	if c.Kind == "channel" {
		return "#" + c.Name
	}
	if c.Kind == "private" {
		return "◇ " + c.Name
	}
	return "@" + c.Name
}

// Message contains the text and fallback content needed to display one Slack message.
type Message struct {
	TS       string `json:"ts"`
	User     string `json:"user"`
	Author   string `json:"-"`
	Text     string `json:"text"`
	Thread   string `json:"thread_ts"`
	Replies  int    `json:"reply_count"`
	ClientID string `json:"client_msg_id"`
	Subtype  string `json:"subtype"`
	Link     string `json:"-"`
	Files    []struct {
		Title string `json:"title"`
		URL   string `json:"permalink"`
	} `json:"files"`
	Attachments []struct {
		Fallback string `json:"fallback"`
		Text     string `json:"text"`
	} `json:"attachments"`
	Blocks []map[string]any `json:"blocks"`
}

// Page is one history or thread page; Next and More describe pagination.
type Page struct {
	Messages []Message
	Next     string
	More     bool
}

// DirectoryPage is a page of accessible conversation metadata.
type DirectoryPage struct {
	Conversations []Conversation
	Next          string
}
type (
	// Query identifies a timeline and optional cursor or exclusive timestamp bounds.
	Query struct{ Conversation, Thread, Cursor, Before, After string }
	// Outgoing is an immutable send attempt with a stable client ID.
	Outgoing struct{ Conversation, Thread, Text, ID string }
	// Service is the context-aware boundary shared by Slack and the offline demo.
	Service interface {
		Identity(context.Context) (Identity, error)
		Conversation(context.Context, string) (Conversation, error)
		Directory(context.Context, string) (DirectoryPage, error)
		History(context.Context, Query) (Page, error)
		Send(context.Context, Outgoing) (Message, error)
	}
)

// Filter never selects on the user's behalf. Switching searches only included IDs.
func Filter(cs []Conversation, query string, included map[string]bool, onlyIncluded bool) []Conversation {
	out := []Conversation{}
	q := strings.ToLower(strings.TrimSpace(query))
	for _, c := range cs {
		if onlyIncluded && !included[c.ID] {
			continue
		}
		if strings.Contains(strings.ToLower(c.Label()+" "+c.Kind), q) {
			out = append(out, c)
		}
	}
	return out
}

// Merge deduplicates overlapping pages and caps in-memory message retention.
func Merge(old, fresh []Message, limit int) []Message {
	byID := map[string]Message{}
	for _, m := range old {
		byID[m.TS] = m
	}
	for _, m := range fresh {
		byID[m.TS] = m
	}
	out := make([]Message, 0, len(byID))
	for _, m := range byID {
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TS < out[j].TS })
	if len(out) > limit {
		out = out[len(out)-limit:]
	}
	return out
}

// Delivery retains the original body until acknowledgement, across retries.
// An in-flight attempt cannot be submitted again.
type Delivery struct {
	Outgoing
	State     string
	Err       error
	Uncertain bool
}

// Begin starts an attempt unless this delivery is already pending or acknowledged.
func (d *Delivery) Begin() bool {
	if d.State == "pending" || d.State == "sent" {
		return false
	}
	d.State = "pending"
	d.Err = nil
	return true
}

// Finish records acknowledgement or failure without discarding the original body.
func (d *Delivery) Finish(err error, uncertain bool) {
	d.Err = err
	d.Uncertain = uncertain
	if err != nil {
		d.State = "failed"
	} else {
		d.State = "sent"
	}
}

// DeliveryError classifies a failed send without depending on its display text.
// Implementations return true when the remote service may have accepted the write.
type DeliveryError interface {
	error
	DeliveryUncertain() bool
}

// UncertainDelivery conservatively treats unclassified errors as ambiguous.
func UncertainDelivery(err error) bool {
	var outcome DeliveryError
	if errors.As(err, &outcome) {
		return outcome.DeliveryUncertain()
	}
	return err != nil
}
