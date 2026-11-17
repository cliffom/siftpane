package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/zalando/go-keyring"
	"golang.org/x/term"

	"github.com/cliffom/siftpane/internal/core"
	"github.com/cliffom/siftpane/internal/slack"
	"github.com/cliffom/siftpane/internal/store"
	"github.com/cliffom/siftpane/internal/ui"
)

func tokenIdentity(ctx context.Context, token string) (core.Identity, error) {
	if !strings.HasPrefix(token, userTokenPrefix) {
		return core.Identity{}, fmt.Errorf("expected an xoxp- user OAuth token")
	}
	ctx, cancel := context.WithTimeout(ctx, identityTimeout)
	defer cancel()
	return slack.New(token).Identity(ctx)
}

func saveAccount(st *store.Store, id core.Identity, token string) error {
	key := store.AccountKey(id)
	if err := keyring.Set(credentialService, key, token); err != nil {
		return fmt.Errorf("OS keyring unavailable; use SIFTPANE_SLACK_TOKEN for this session instead (token was not written to disk)")
	}
	if st.Config.Accounts == nil {
		st.Config.Accounts = map[string]core.Identity{}
	}
	st.Config.Accounts[key] = id
	st.Config.DefaultAccount = key
	// Claim legacy preferences at the same time as the credential migration.
	st.AccountWorkspace(id)
	return st.Save()
}

func loginToken(ctx context.Context, st *store.Store, selected string) error {
	fmt.Fprint(os.Stderr, "User OAuth token (hidden): ")
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return fmt.Errorf("read token: %w", err)
	}
	token := strings.TrimSpace(string(b))
	id, err := tokenIdentity(ctx, token)
	if err != nil {
		return err
	}
	if selected != "" {
		err = replaceAccountCredential(st, selected, id, token)
	} else {
		err = saveLoginAccount(ctx, st, id, token, tokenIdentity)
	}
	if err != nil {
		return err
	}
	fmt.Println("Saved account", store.AccountKey(id), "for", slack.Safe(id.WorkspaceName), "@"+slack.Safe(id.Username))
	return nil
}

func logoutAccount(st *store.Store, selected string) error {
	if selected == "" {
		selected = st.Config.DefaultAccount
	}
	if selected == "" || selected == credentialAccount {
		return deleteCredential(credentialAccount)
	}
	if _, ok := st.Config.Accounts[selected]; !ok {
		return fmt.Errorf("unknown saved account; use WORKSPACE_ID/USER_ID")
	}
	if st.Config.LegacyAccount == selected {
		if err := deleteCredential(credentialAccount); err != nil {
			return err
		}
	}
	if err := deleteCredential(selected); err != nil {
		return err
	}
	delete(st.Config.Accounts, selected)
	if st.Config.DefaultAccount == selected {
		st.Config.DefaultAccount = ""
	}
	// Preferences and durable dismissals survive logout; disk cleanup remains explicit.
	return st.Save()
}

type accountAuthenticator func(context.Context, string) (core.Identity, error)

func loadAccounts(ctx context.Context, st *store.Store, selected string) ([]ui.Account, error) {
	return loadAccountsWith(ctx, st, selected, tokenIdentity)
}

func loadAccountsWith(ctx context.Context, st *store.Store, selected string, authenticate accountAuthenticator) ([]ui.Account, error) {
	explicit := selected != ""
	if token := os.Getenv(tokenEnvironment); token != "" {
		id, err := authenticate(ctx, token)
		if err != nil {
			return nil, err
		}
		if selected != "" && selected != store.AccountKey(id) {
			return nil, fmt.Errorf("--account does not match the environment token")
		}
		return []ui.Account{connectedAccount(id, token, true)}, nil
	}
	if err := migrateCredential(ctx, st, authenticate); err != nil {
		return nil, err
	}

	if selected == "" {
		selected = st.Config.DefaultAccount
	}
	if selected != "" {
		if _, ok := st.Config.Accounts[selected]; !ok {
			return nil, fmt.Errorf("unknown saved account; use WORKSPACE_ID/USER_ID")
		}
	}
	keys := make([]string, 0, len(st.Config.Accounts))
	for key := range st.Config.Accounts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if selected != "" {
		for i, key := range keys {
			if key == selected {
				keys = append([]string{key}, append(keys[:i], keys[i+1:]...)...)
				break
			}
		}
	}
	accounts := []ui.Account{}
	for _, key := range keys {
		token, err := keyring.Get(credentialService, key)
		if err == nil {
			var id core.Identity
			id, err = authenticate(ctx, token)
			if err == nil && store.AccountKey(id) == key {
				accounts = append(accounts, connectedAccount(id, token, false))
				continue
			}
		}
		if key == selected && explicit {
			return nil, fmt.Errorf("selected account unavailable; run --login or choose another --account")
		}
		fmt.Fprintln(os.Stderr, "Skipped unavailable account", slack.Safe(key), "· run --login to reconnect")
	}
	if len(accounts) == 0 {
		return nil, fmt.Errorf("no usable accounts; run --login")
	}
	return accounts, nil
}

func migrateCredential(ctx context.Context, st *store.Store, authenticate accountAuthenticator) error {
	if len(st.Config.Accounts) != 0 {
		return nil
	}
	if st.Config.LegacyAccount != "" {
		return fmt.Errorf("no saved accounts: run siftpane --login or set SIFTPANE_SLACK_TOKEN")
	}

	token, err := keyring.Get(credentialService, credentialAccount)
	if err != nil {
		if !errors.Is(err, keyring.ErrNotFound) {
			return fmt.Errorf("OS credential store unavailable; unlock it, or supply SIFTPANE_SLACK_TOKEN explicitly; --demo needs no credentials")
		}
		return fmt.Errorf("no user token: create/install your internal Slack app using docs/SLACK.md, then run siftpane --login; --demo works offline")
	}
	id, err := authenticate(ctx, token)
	if err != nil {
		return err
	}
	return saveLoginAccount(ctx, st, id, token, authenticate)
}

// Register the verified legacy owner before replacing its slot. An explicit
// login may precede normal startup, including when refreshing reaction scopes.
func saveLoginAccount(ctx context.Context, st *store.Store, id core.Identity, token string, authenticate accountAuthenticator) error {
	legacyToken, err := keyring.Get(credentialService, credentialAccount)
	if errors.Is(err, keyring.ErrNotFound) {
		return saveAccount(st, id, token)
	}
	if err != nil {
		return fmt.Errorf("read legacy credential: %w", err)
	}
	legacyID := id
	if legacyToken != token {
		legacyID, err = authenticate(ctx, legacyToken)
		if err != nil {
			return fmt.Errorf("cannot identify legacy account; preserve configuration and remove the old token with --logout --account user-token before retrying --login: %w", err)
		}
	}
	legacyKey := store.AccountKey(legacyID)
	previousOwner := st.Config.LegacyAccount
	st.Config.LegacyAccount = legacyKey
	if legacyKey != store.AccountKey(id) && previousOwner != legacyKey {
		if _, registered := st.Config.Accounts[legacyKey]; !registered {
			if err = saveAccount(st, legacyID, legacyToken); err != nil {
				st.Config.LegacyAccount = previousOwner
				return err
			}
		}
	}
	if err = saveAccount(st, id, token); err != nil {
		return err
	}
	// Registry persistence retires automatic import even if cleanup fails. Logout
	// retries this owner's old slot before reporting successful credential removal.
	if err = deleteCredential(credentialAccount); err != nil {
		fmt.Fprintln(os.Stderr, "Legacy keyring cleanup failed; account is saved. Logout retries cleanup for the original account.")
	}
	return nil
}

func deleteCredential(account string) error {
	err := keyring.Delete(credentialService, account)
	if err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return fmt.Errorf("delete saved credential: %w", err)
	}
	return nil
}
