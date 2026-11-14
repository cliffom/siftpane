package core

import (
	"errors"
	"fmt"
	"testing"
)

func TestExplicitFilter(t *testing.T) {
	cs := []Conversation{{ID: "C", Name: "design", Kind: "channel"}, {ID: "D", Name: "Dora", Kind: "dm"}}
	included := map[string]bool{"D": true}
	if got := Filter(cs, "D", included, true); len(got) != 1 || got[0].ID != "D" {
		t.Fatal(got)
	}
	if len(Filter(cs, "DESIGN", included, false)) != 1 {
		t.Fatal("case-insensitive picker search")
	}
	if len(included) != 1 {
		t.Fatal("search mutated selection")
	}
	delete(included, "D")
	if len(Filter(cs, "", included, true)) != 0 {
		t.Fatal("removed conversation still included")
	}
}

func TestMergeOverlapAndBound(t *testing.T) {
	got := Merge([]Message{{TS: "1", Text: "old"}, {TS: "2"}}, []Message{{TS: "1", Text: "edited"}, {TS: "3"}}, 3)
	if len(got) != 3 || got[0].Text != "edited" {
		t.Fatal(got)
	}
	got = Merge(got, []Message{{TS: "4"}}, 2)
	if len(got) != 2 || got[0].TS != "3" {
		t.Fatal(got)
	}
}

func TestDeliveryRetainsTextAndGuardsDuplicates(t *testing.T) {
	d := Delivery{Outgoing: Outgoing{Text: "keep me", ID: "same-id"}}
	if !d.Begin() || d.Begin() {
		t.Fatal("duplicate accepted")
	}
	d.Finish(errors.New("failed"), false)
	if d.Text != "keep me" || !d.Begin() || d.ID != "same-id" {
		t.Fatal("retry lost text or identity")
	}
	d.Finish(nil, false)
	if d.Begin() {
		t.Fatal("sent delivery repeated")
	}
}

func TestUncertainDeliveryHandlesWrappedOutcomes(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want bool
	}{
		{name: "acknowledged"},
		{name: "unknown", err: errors.New("connection lost"), want: true},
		{name: "definite", err: testDeliveryError{}},
		{name: "wrapped ambiguous", err: fmt.Errorf("post: %w", testDeliveryError{uncertain: true}), want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := UncertainDelivery(tc.err); got != tc.want {
				t.Fatalf("UncertainDelivery()=%v, want %v", got, tc.want)
			}
		})
	}
}

type testDeliveryError struct{ uncertain bool }

func (testDeliveryError) Error() string             { return "test delivery outcome" }
func (e testDeliveryError) DeliveryUncertain() bool { return e.uncertain }
