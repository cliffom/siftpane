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

func TestReplacementWritesOnlyMatchingCredential(t *testing.T) {
	keyring.MockInit()
	id := core.Identity{Workspace: "T1", User: "U1"}
	st, err := store.Open(filepath.Join(t.TempDir(), "preferences.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = saveAccount(st, id, fakeAccountToken); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(st.Path)
	if err != nil {
		t.Fatal(err)
	}
	other := core.Identity{Workspace: "T2", User: "U1"}
	if replaceAccountCredential(st, store.AccountKey(id), other, healthyAccountToken) == nil {
		t.Fatal("wrong account replacement accepted")
	}
	if err = replaceAccountCredential(st, store.AccountKey(id), id, healthyAccountToken); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(st.Path)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := keyring.Get(credentialService, store.AccountKey(id))
	if err != nil || saved != healthyAccountToken || string(before) != string(after) {
		t.Fatal("replacement changed preferences or failed secure storage", err)
	}
}

func TestCredentialReloadReadFailureAndTimeoutBoundary(t *testing.T) {
	for _, locked := range []bool{false, true} {
		calls := 0
		_, _, err := reloadSavedCredential(context.Background(), func() (string, error) {
			if locked {
				return "", errors.New("private store details")
			}
			return "wrong-type", nil
		}, func(context.Context, string) (core.Service, core.Identity, error) {
			calls++
			return nil, core.Identity{}, nil
		})
		if err == nil || calls != 0 || strings.Contains(err.Error(), "private store details") {
			t.Fatal("unsafe credential-read recovery", err)
		}
	}
	_, _, err := reloadSavedCredential(context.Background(), func() (string, error) {
		return fakeAccountToken, nil
	}, func(ctx context.Context, token string) (core.Service, core.Identity, error) {
		if _, bounded := ctx.Deadline(); !bounded || token != fakeAccountToken {
			t.Fatal("missing authentication deadline or wrong credential")
		}
		return nil, core.Identity{}, errors.New("offline rejection")
	})
	if err == nil {
		t.Fatal("validation failure hidden")
	}
}
