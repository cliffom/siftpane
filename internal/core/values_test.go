package core

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestDomainValueCompatibility(t *testing.T) {
	// These independent literals freeze the existing textual boundary values.
	for _, tc := range []struct {
		value any
		wire  string
	}{
		{ConversationChannel, `"channel"`},
		{ConversationPrivate, `"private"`},
		{ConversationDM, `"dm"`},
		{ConversationGroup, `"group"`},
		{DeliveryState(""), `""`},
		{DeliveryPending, `"pending"`},
		{DeliveryFailed, `"failed"`},
		{DeliverySent, `"sent"`},
	} {
		b, err := json.Marshal(tc.value)
		if err != nil || string(b) != tc.wire {
			t.Fatalf("%T marshals to %s, want %s: %v", tc.value, b, tc.wire, err)
		}
	}
	for _, kind := range []ConversationKind{ConversationChannel, ConversationPrivate, ConversationDM, ConversationGroup} {
		b, err := json.Marshal(kind)
		if err != nil {
			t.Fatal(err)
		}
		var decoded ConversationKind
		if err := json.Unmarshal(b, &decoded); err != nil || decoded != kind {
			t.Fatalf("kind round trip: %q, %v", decoded, err)
		}
		c := Conversation{ID: "C", Name: "example", Kind: kind}
		if got := Filter([]Conversation{c}, string(kind), map[string]bool{"C": true}, true); len(got) != 1 {
			t.Fatalf("kind %q no longer searchable", kind)
		}
	}
}

func TestDeliveryStateTransitions(t *testing.T) {
	d := Delivery{Outgoing: Outgoing{Text: "retained", ID: "attempt"}}
	if d.State != "" || !d.Begin() || d.State != DeliveryPending || d.Begin() {
		t.Fatal("idle to pending or duplicate guard changed", d)
	}
	sendErr := errors.New("offline")
	d.Finish(sendErr, true)
	if d.State != DeliveryFailed || !d.Uncertain || !errors.Is(d.Err, sendErr) {
		t.Fatal("failed delivery lost classification", d)
	}
	if !d.Begin() || d.State != DeliveryPending || d.Err != nil || d.Text != "retained" || d.ID != "attempt" {
		t.Fatal("explicit retry changed original attempt", d)
	}
	d.Finish(nil, false)
	if d.State != DeliverySent || d.Uncertain || d.Begin() {
		t.Fatal("acknowledgement or sent guard changed", d)
	}
}
