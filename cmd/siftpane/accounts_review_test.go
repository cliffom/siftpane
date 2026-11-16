package main

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/zalando/go-keyring"

	"github.com/cliffom/siftpane/internal/core"
	"github.com/cliffom/siftpane/internal/store"
)

const legacyReviewToken = "xoxp-legacy-review-offline"

func reviewAccountStore(t *testing.T) *store.Store {
	t.Helper()
	keyring.MockInit()
	t.Setenv(tokenEnvironment, "")
	st, err := store.Open(filepath.Join(t.TempDir(), accountTestConfig))
	if err != nil {
		t.Fatal(err)
	}
	if err = keyring.Set(credentialService, credentialAccount, legacyReviewToken); err != nil {
		t.Fatal(err)
	}
	return st
}

func TestLoginBeforeStartupRetiresLegacyAndPreservesOwner(t *testing.T) {
	for _, user := range []string{"U1", "U2"} {
		t.Run(user, func(t *testing.T) { testLoginBeforeStartup(t, user) })
	}
}

func testLoginBeforeStartup(t *testing.T, user string) {
	t.Helper()
	st := reviewAccountStore(t)
	original := core.Identity{Workspace: "T1", User: "U1"}
	replacement := core.Identity{Workspace: "T1", User: user}
	st.Workspace(original.Workspace).Included["C1"] = true
	authenticate := func(_ context.Context, token string) (core.Identity, error) {
		if token != legacyReviewToken {
			t.Fatal("unexpected authentication target")
		}
		return original, nil
	}
	if err := saveLoginAccount(context.Background(), st, replacement, healthyAccountToken, authenticate); err != nil {
		t.Fatal(err)
	}
	if _, err := keyring.Get(credentialService, credentialAccount); !errors.Is(err, keyring.ErrNotFound) {
		t.Fatal("legacy token retained", err)
	}
	if token, err := keyring.Get(credentialService, store.AccountKey(replacement)); err != nil || token != healthyAccountToken {
		t.Fatal("replacement credential overwritten", err)
	}
	if !st.AccountWorkspace(original).Included["C1"] {
		t.Fatal("legacy owner lost preferences")
	}
	if user != original.User && st.AccountWorkspace(replacement).Included["C1"] {
		t.Fatal("legacy preferences assigned to another user")
	}
	if err := logoutAccount(st, store.AccountKey(replacement)); err != nil {
		t.Fatal(err)
	}
	if user != original.User {
		if token, err := keyring.Get(credentialService, store.AccountKey(original)); err != nil || token != legacyReviewToken {
			t.Fatal("logout removed original account", err)
		}
		if err := logoutAccount(st, store.AccountKey(original)); err != nil {
			t.Fatal(err)
		}
	}
	restored, err := store.Open(st.Path)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Config.LegacyAccount != store.AccountKey(original) {
		t.Fatal("legacy retirement not durable")
	}
	if accounts, err := loadAccountsWith(context.Background(), restored, "", authenticate); err == nil || len(accounts) != 0 {
		t.Fatal("logout restored legacy account", err)
	}
}

func TestLogoutRetriesOnlyItsLegacySlot(t *testing.T) {
	st := reviewAccountStore(t)
	original := core.Identity{Workspace: "T1", User: "U1"}
	other := core.Identity{Workspace: "T2", User: "U2"}
	authenticate := func(context.Context, string) (core.Identity, error) { return original, nil }
	if err := saveLoginAccount(context.Background(), st, original, healthyAccountToken, authenticate); err != nil {
		t.Fatal(err)
	}
	// Recreate the leftover slot from a failed best-effort migration cleanup.
	if err := keyring.Set(credentialService, credentialAccount, legacyReviewToken); err != nil {
		t.Fatal(err)
	}
	if err := saveAccount(st, other, fakeAccountToken); err != nil {
		t.Fatal(err)
	}
	if err := logoutAccount(st, store.AccountKey(other)); err != nil {
		t.Fatal(err)
	}
	if _, err := keyring.Get(credentialService, credentialAccount); err != nil {
		t.Fatal("unrelated logout removed legacy owner's slot", err)
	}
	if err := logoutAccount(st, store.AccountKey(original)); err != nil {
		t.Fatal(err)
	}
	if _, err := keyring.Get(credentialService, credentialAccount); !errors.Is(err, keyring.ErrNotFound) {
		t.Fatal("owner logout retained legacy slot", err)
	}
	// Even a stale/restored OS slot cannot silently reimport a retired account.
	if err := keyring.Set(credentialService, credentialAccount, legacyReviewToken); err != nil {
		t.Fatal(err)
	}
	calls := 0
	accounts, err := loadAccountsWith(context.Background(), st, "", func(context.Context, string) (core.Identity, error) { calls++; return original, nil })
	if err == nil || len(accounts) != 0 || calls != 0 {
		t.Fatal("retired legacy import was attempted", err, calls)
	}
	// An explicit login for another account must not resurrect the retired owner.
	if err := saveLoginAccount(context.Background(), st, other, fakeAccountToken, authenticate); err != nil {
		t.Fatal(err)
	}
	if _, ok := st.Config.Accounts[store.AccountKey(original)]; ok {
		t.Fatal("unrelated login restored retired legacy owner")
	}
	if len(st.Config.Accounts) != 1 {
		t.Fatal("unexpected accounts after explicit login")
	}
}

func TestLogoutCleanupFailureDoesNotReportSuccess(t *testing.T) {
	st := reviewAccountStore(t)
	id := core.Identity{Workspace: "T1", User: "U1"}
	if err := saveLoginAccount(context.Background(), st, id, legacyReviewToken, func(context.Context, string) (core.Identity, error) {
		t.Fatal("same verified token was authenticated twice")
		return core.Identity{}, nil
	}); err != nil {
		t.Fatal(err)
	}
	keyring.MockInitWithError(errors.New("locked credential store"))
	t.Cleanup(keyring.MockInit)
	if err := logoutAccount(st, store.AccountKey(id)); err == nil {
		t.Fatal("logout hid cleanup failure")
	}
	if _, ok := st.Config.Accounts[store.AccountKey(id)]; !ok {
		t.Fatal("failed logout removed recovery registry")
	}
}

func TestLoginCannotDiscardUnidentifiedLegacyCredential(t *testing.T) {
	st := reviewAccountStore(t)
	id := core.Identity{Workspace: "T1", User: "U1"}
	authenticate := func(context.Context, string) (core.Identity, error) {
		return core.Identity{}, errors.New("identity unavailable")
	}
	if err := saveLoginAccount(context.Background(), st, id, healthyAccountToken, authenticate); err == nil {
		t.Fatal("unidentified legacy credential replaced")
	}
	if len(st.Config.Accounts) != 0 {
		t.Fatal("failed login registered replacement")
	}
	if token, err := keyring.Get(credentialService, credentialAccount); err != nil || token != legacyReviewToken {
		t.Fatal("failed login discarded legacy credential", err)
	}
	if err := logoutAccount(st, credentialAccount); err != nil {
		t.Fatal(err)
	}
	if err := saveLoginAccount(context.Background(), st, id, healthyAccountToken, authenticate); err != nil {
		t.Fatal("explicit legacy removal did not unblock login", err)
	}
}
