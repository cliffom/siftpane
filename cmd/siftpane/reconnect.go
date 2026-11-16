package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/zalando/go-keyring"

	"github.com/cliffom/siftpane/internal/core"
	"github.com/cliffom/siftpane/internal/slack"
	"github.com/cliffom/siftpane/internal/store"
	"github.com/cliffom/siftpane/internal/ui"
)

func connectedAccount(id core.Identity, token string, environment bool) ui.Account {
	client := slack.New(token)
	return ui.Account{Service: client, Identity: id, Reload: func(ctx context.Context) (core.Service, core.Identity, error) {
		read := func() (string, error) { return keyring.Get(credentialService, store.AccountKey(id)) }
		if environment {
			read = func() (string, error) { return os.Getenv(tokenEnvironment), nil }
		}
		return reloadSavedCredential(ctx, read, func(ctx context.Context, replacement string) (core.Service, core.Identity, error) {
			return client.Reauthorize(ctx, replacement, id)
		})
	}}
}

func reloadSavedCredential(ctx context.Context, read func() (string, error), validate func(context.Context, string) (core.Service, core.Identity, error)) (core.Service, core.Identity, error) {
	token, err := read()
	if err != nil {
		return nil, core.Identity{}, fmt.Errorf("cannot read OS credential store; unlock it and run --login --account WORKSPACE_ID/USER_ID in another terminal, then Ctrl+R here")
	}
	if !strings.HasPrefix(token, userTokenPrefix) {
		return nil, core.Identity{}, fmt.Errorf("expected a saved xoxp- user token; run --login --account WORKSPACE_ID/USER_ID in another terminal, then Ctrl+R here")
	}
	ctx, cancel := context.WithTimeout(ctx, identityTimeout)
	defer cancel()
	return validate(ctx, token)
}

// Replacing an existing credential writes only its keyring slot. A concurrently
// running UI remains the sole preference writer; no config snapshot is saved.
func replaceAccountCredential(st *store.Store, selected string, id core.Identity, token string) error {
	if _, ok := st.Config.Accounts[selected]; !ok {
		return fmt.Errorf("unknown saved account; use --login without --account to register it first")
	}
	if store.AccountKey(id) != selected {
		return fmt.Errorf("replacement belongs to another account; original credential retained")
	}
	if err := keyring.Set(credentialService, selected, token); err != nil {
		return fmt.Errorf("OS credential store unavailable; unlock it and retry --login --account (original session retained)")
	}
	return nil
}
