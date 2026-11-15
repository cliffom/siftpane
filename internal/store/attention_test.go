package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cliffom/siftpane/internal/core"
)

const attentionTestUnkept = "unkept"

func TestAttentionDurabilityMigrationAndAccountIsolation(t *testing.T) {
	path := filepath.Join(t.TempDir(), testConfigFilename)
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	id := core.Identity{Workspace: "test-workspace", User: "test-user"}
	s.Workspace(id.Workspace).Included["channel"] = true
	a, err := s.Attention(id)
	if err != nil || !a.Kept["channel"] {
		t.Fatal("migration", err)
	}
	at, err := core.MessageTime("1794672000.000001")
	if err != nil {
		t.Fatal(err)
	}
	trigger := core.AttentionTrigger{ID: "mention:1794672000.000001", TS: "1794672000.000001", Reason: core.AttentionMention, At: at}
	if err := a.Add(attentionTestUnkept, trigger); err != nil {
		t.Fatal(err)
	}
	a.Dismiss(attentionTestUnkept)
	a.Enabled = true
	a.Checkpoint = time.Now().UTC()
	if err := s.SaveAttention(id, a); err != nil {
		t.Fatal(err)
	}
	fresh, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := fresh.Attention(id)
	if err != nil {
		t.Fatal(err)
	}
	if err := restored.Add(attentionTestUnkept, trigger); err != nil {
		t.Fatal(err)
	}
	if len(restored.Pending) != 0 || !restored.Enabled || !restored.Checkpoint.Equal(a.Checkpoint) {
		t.Fatal("restart lost durable identities/checkpoint")
	}
	for _, other := range []core.Identity{{Workspace: id.Workspace, User: "different"}, {Workspace: "different", User: id.User}} {
		state, err := fresh.Attention(other)
		if err != nil || state.Enabled || len(state.Handled) > 0 {
			t.Fatal("account state leaked", err)
		}
	}
	info, err := os.Stat(s.cachePath(id, attentionFileKind))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatal("state not private")
	}
	b, err := os.ReadFile(s.cachePath(id, attentionFileKind))
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"text", "token", "permalink", "display_name"} {
		if strings.Contains(string(b), forbidden) {
			t.Fatal("unexpected persisted payload", forbidden)
		}
	}
}

func TestCorruptAttentionIsNotSilentlyReset(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), testConfigFilename))
	if err != nil {
		t.Fatal(err)
	}
	id := core.Identity{Workspace: "workspace", User: "user"}
	for _, contents := range []string{"{", `{"version":99}`, strings.Repeat("x", attentionBytes+1)} {
		if err := os.WriteFile(s.cachePath(id, attentionFileKind), []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Attention(id); err == nil {
			t.Fatal("corrupt state silently reset")
		}
	}
}

func TestAttentionGapDurabilityIsolationAndValidation(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), testConfigFilename))
	if err != nil {
		t.Fatal(err)
	}
	id := core.Identity{Workspace: "workspace", User: "user"}
	a, err := st.Attention(id)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	a.RecordGap(now.Add(-core.AttentionLookback-time.Hour), now.Add(-core.AttentionLookback))
	candidate := a.Clone()
	candidate.RecordGap(now.Add(-time.Hour), now)
	if a.Gap.Until.Equal(candidate.Gap.Until) {
		t.Fatal("candidate mutated committed gap")
	}
	if err := st.SaveAttention(id, candidate); err != nil {
		t.Fatal(err)
	}
	restored, err := st.Attention(id)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Gap == nil || !restored.Gap.From.Equal(a.Gap.From) || !restored.Gap.Until.Equal(now) {
		t.Fatal("gap bounds changed during reload")
	}
	other, err := st.Attention(core.Identity{Workspace: id.Workspace, User: "other"})
	if err != nil || other.Gap != nil {
		t.Fatal("gap leaked to another account", err)
	}
	bad := candidate.Clone()
	bad.Gap.Until = bad.Gap.From
	if err := st.SaveAttention(id, bad); err == nil {
		t.Fatal("invalid gap bounds accepted")
	}
}
