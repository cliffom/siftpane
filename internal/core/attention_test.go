package core

import (
	"fmt"
	"testing"
	"time"
)

const (
	attentionFixtureFirst   = "one"
	attentionFixtureChannel = "test-channel"
)

func attentionFixtureTrigger(id string, at time.Time) AttentionTrigger {
	return AttentionTrigger{ID: id, TS: "1794672000.000001", Reason: AttentionMention, At: at}
}

func TestAttentionReplayNewActivityAndPruning(t *testing.T) {
	now := time.Date(2026, time.November, 15, 12, 0, 0, 0, time.UTC)
	a := NewAttention(map[string]bool{"kept": true, "excluded": false})
	if !a.Kept["kept"] || len(a.Kept) != 1 {
		t.Fatal("legacy inclusion migration changed")
	}
	trigger := attentionFixtureTrigger(attentionFixtureFirst, now)
	for range 2 {
		if err := a.Add(attentionFixtureChannel, trigger); err != nil {
			t.Fatal(err)
		}
	}
	if len(a.Pending[attentionFixtureChannel]) != 1 {
		t.Fatal("duplicate trigger")
	}
	a.Dismiss(attentionFixtureChannel)
	if err := a.Add(attentionFixtureChannel, trigger); err != nil {
		t.Fatal(err)
	}
	if len(a.Pending) != 0 {
		t.Fatal("handled trigger resurrected")
	}
	if err := a.Add(attentionFixtureChannel, attentionFixtureTrigger("two", now.Add(time.Second))); err != nil {
		t.Fatal(err)
	}
	if len(a.Pending[attentionFixtureChannel]) != 1 {
		t.Fatal("new trigger was hidden")
	}
	b := a.Clone()
	b.Dismiss(attentionFixtureChannel)
	if len(a.Pending) == 0 {
		t.Fatal("candidate mutated committed state")
	}
	a.Prune(now.Add(-time.Second))
	if len(a.Handled) != 1 {
		t.Fatal("pruned inside replay window")
	}
	a.Prune(now.Add(time.Second))
	a.Prune(now.Add(-time.Hour))
	if len(a.Handled) != 0 || !a.Floor.Equal(now.Add(time.Second)) {
		t.Fatal("floor regressed")
	}
	if len(a.Pending) == 0 {
		t.Fatal("old active entry silently dismissed")
	}
}

func TestAttentionCapacityPreservesExistingIdentities(t *testing.T) {
	a := NewAttention(nil)
	now := time.Now()
	for i := range AttentionConversations {
		if err := a.Add(fmt.Sprint(i), attentionFixtureTrigger(attentionFixtureFirst, now)); err != nil {
			t.Fatal(err)
		}
	}
	if a.Add("overflow", attentionFixtureTrigger(attentionFixtureFirst, now)) == nil || len(a.Pending) != AttentionConversations {
		t.Fatal("conversation capacity bypassed")
	}
	a = NewAttention(nil)
	for i := range AttentionTriggers {
		if err := a.Add(attentionFixtureChannel, attentionFixtureTrigger(fmt.Sprint(i), now)); err != nil {
			t.Fatal(err)
		}
	}
	a.Dismiss(attentionFixtureChannel)
	if a.Add(attentionFixtureChannel, attentionFixtureTrigger("new", now)) == nil {
		t.Fatal("handled identity capacity bypassed")
	}
	if err := a.Add(attentionFixtureChannel, attentionFixtureTrigger("0", now)); err != nil {
		t.Fatal("replay should not consume capacity", err)
	}
}

func TestMessageTimePreservesPrecision(t *testing.T) {
	a, err := MessageTime("1794672000.000001")
	if err != nil {
		t.Fatal(err)
	}
	b, err := MessageTime("1794672000.000002")
	if err != nil || b.Sub(a) != time.Microsecond {
		t.Fatal("timestamp precision lost", err)
	}
	for _, s := range []string{"", "1", "1.00000", "-1.000000", "1.12345x", "999999999999999999999.000000"} {
		if _, err := MessageTime(s); err == nil {
			t.Fatal("invalid identity accepted", s)
		}
	}
}
