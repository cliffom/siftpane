package core

import "testing"

func TestPrefixSearchAndCompletion(t *testing.T) {
	cs := []Conversation{
		{ID: "C1", Name: searchChannelName, Kind: ConversationChannel},
		{ID: "C2", Name: "design-review", Kind: ConversationPrivate},
		{ID: "D1", Name: "Maya Chen", Handle: "mchen", Kind: ConversationDM},
		{ID: "G1", Name: "Design crew", Kind: ConversationGroup},
	}
	for _, tc := range []struct {
		query string
		ids   []string
	}{
		{"#", []string{"C1", "C2"}},
		{" #DESIGN ", []string{"C1", "C2"}},
		{"@", []string{"D1", "G1"}},
		{"@MCH", []string{"D1"}},
		{"@maya", []string{"D1"}},
		{"@design", []string{"G1"}},
		{searchChannelName, []string{"C1", "C2", "G1"}},
		{"#maya", nil},
		{"@missing", nil},
	} {
		t.Run(tc.query, func(t *testing.T) {
			got := Filter(cs, tc.query, nil, false)
			if len(got) != len(tc.ids) {
				t.Fatalf("Filter(%q) = %v, want %v", tc.query, got, tc.ids)
			}
			for i, id := range tc.ids {
				if got[i].ID != id {
					t.Fatalf("Filter(%q)[%d] = %s, want %s", tc.query, i, got[i].ID, id)
				}
			}
		})
	}
	for _, c := range cs {
		matches := Filter(cs, c.Completion(), nil, false)
		found := false
		for _, match := range matches {
			found = found || match.ID == c.ID
		}
		if !found {
			t.Fatalf("completion %q lost %s", c.Completion(), c.ID)
		}
	}
	if cs[1].Completion() != "#design-review" || cs[2].Completion() != "@mchen" {
		t.Fatal("completion lost private channel prefix or user handle")
	}
}

const searchChannelName = "design"
