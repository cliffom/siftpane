package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"

	"github.com/cliffom/siftpane/internal/core"
	"github.com/cliffom/siftpane/internal/store"
)

const (
	fakeAccountToken    = "xoxp-offline-test-secret"
	accountTestConfig   = "config.json"
	healthyAccountToken = "xoxp-healthy-offline"
)

func TestSavedCredentialsAreIsolatedAndNeverWrittenToConfig(t *testing.T) {
	keyring.MockInit()
	st, err := store.Open(filepath.Join(t.TempDir(), accountTestConfig))
	if err != nil {
		t.Fatal(err)
	}
	ids := []core.Identity{{Workspace: "T1", User: "U1"}, {Workspace: "T1", User: "U2"}, {Workspace: "T2", User: "U1"}}
	for _, id := range ids {
		if err = saveAccount(st, id, fakeAccountToken); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(st.Path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), fakeAccountToken) || strings.Contains(string(data), "xoxp-") {
		t.Fatal("secret in config")
	}
	for _, id := range ids {
		token, err := keyring.Get(credentialService, store.AccountKey(id))
		if err != nil || token != fakeAccountToken {
			t.Fatal("account credential missing")
		}
	}
	if err = logoutAccount(st, store.AccountKey(ids[0])); err != nil {
		t.Fatal(err)
	}
	if _, err = keyring.Get(credentialService, store.AccountKey(ids[0])); !errors.Is(err, keyring.ErrNotFound) {
		t.Fatal("credential retained")
	}
	if _, err = keyring.Get(credentialService, store.AccountKey(ids[1])); err != nil {
		t.Fatal("logout removed another user")
	}
	if len(st.Config.Accounts) != 2 {
		t.Fatal("registry mismatch")
	}
}

func TestUnavailableKeyringDoesNotCreateRegistry(t *testing.T) {
	keyring.MockInitWithError(errors.New("locked"))
	t.Cleanup(keyring.MockInit)
	st, err := store.Open(filepath.Join(t.TempDir(), accountTestConfig))
	if err != nil {
		t.Fatal(err)
	}
	if saveAccount(st, core.Identity{Workspace: "T1", User: "U1"}, fakeAccountToken) == nil {
		t.Fatal("keyring error ignored")
	}
	if len(st.Config.Accounts) != 0 {
		t.Fatal("failed save registered account")
	}
	if _, err = os.Stat(st.Path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("failed credential save wrote config")
	}
}

func TestLegacyCredentialMigrationCommitsBeforeDeletingOriginal(t *testing.T) {
	for _, failSave := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "save-failure"}[failSave], func(t *testing.T) {
			testLegacyCredentialMigration(t, failSave)
		})
	}
}

func TestUnavailableDefaultDoesNotBlockHealthyAccount(t *testing.T) {
	keyring.MockInit()
	t.Setenv(tokenEnvironment, "")
	st := &store.Store{Config: store.Config{Workspaces: map[string]*store.Workspace{}}}
	healthy := core.Identity{Workspace: "T1", User: "U1"}
	failed := core.Identity{Workspace: "T2", User: "U2"}
	if err := saveAccount(st, healthy, healthyAccountToken); err != nil {
		t.Fatal(err)
	}
	if err := saveAccount(st, failed, "xoxp-failed-offline"); err != nil {
		t.Fatal(err)
	}
	authenticate := func(_ context.Context, token string) (core.Identity, error) {
		if token == healthyAccountToken {
			return healthy, nil
		}
		return core.Identity{}, errors.New("expired")
	}
	accounts, err := loadAccountsWith(context.Background(), st, "", authenticate)
	if err != nil || len(accounts) != 1 || accounts[0].Identity != healthy {
		t.Fatal("failed default blocked healthy account", err)
	}
	if _, err = loadAccountsWith(context.Background(), st, store.AccountKey(failed), authenticate); err == nil {
		t.Fatal("explicit unavailable selection silently changed identity")
	}
	t.Setenv(tokenEnvironment, healthyAccountToken)
	accounts, err = loadAccountsWith(context.Background(), st, "", authenticate)
	if err != nil || len(accounts) != 1 || accounts[0].Identity != healthy || len(st.Config.Accounts) != 2 {
		t.Fatal("environment override changed registry", err)
	}
}

func testLegacyCredentialMigration(t *testing.T, failSave bool) {
	t.Helper()
	keyring.MockInit()
	if err := keyring.Set(credentialService, credentialAccount, fakeAccountToken); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), accountTestConfig)
	st, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	id := core.Identity{Workspace: "T1", User: "U1"}
	st.Workspace(id.Workspace).Included["C1"] = true
	if failSave {
		if err = os.WriteFile(path, []byte("block"), 0o600); err != nil {
			t.Fatal(err)
		}
		st.Path = filepath.Join(path, accountTestConfig)
	}
	authenticate := func(_ context.Context, token string) (core.Identity, error) {
		if token != fakeAccountToken {
			t.Fatal("unexpected credential")
		}
		return id, nil
	}
	accounts, err := loadAccountsWith(context.Background(), st, "", authenticate)
	_, legacyErr := keyring.Get(credentialService, credentialAccount)
	if failSave {
		if err == nil || legacyErr != nil {
			t.Fatal("failed migration deleted original", err, legacyErr)
		}
		return
	}
	if err != nil || len(accounts) != 1 || accounts[0].Identity != id || !errors.Is(legacyErr, keyring.ErrNotFound) {
		t.Fatal("migration failed", err, legacyErr)
	}
	if !st.AccountWorkspace(id).Included["C1"] {
		t.Fatal("legacy inclusion lost")
	}
	saved, err := store.Open(st.Path)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Config.DefaultAccount != store.AccountKey(id) || len(saved.Config.Accounts) != 1 {
		t.Fatal("migration not registered durably")
	}
}
