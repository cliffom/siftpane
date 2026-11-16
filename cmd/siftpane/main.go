// Command siftpane runs the focused Slack terminal client or its offline demo.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/cliffom/siftpane/internal/core"
	"github.com/cliffom/siftpane/internal/demo"
	"github.com/cliffom/siftpane/internal/store"
	"github.com/cliffom/siftpane/internal/ui"
	"github.com/cliffom/siftpane/internal/version"
)

const (
	credentialService = "siftpane"
	credentialAccount = "user-token"
	tokenEnvironment  = "SIFTPANE_SLACK_TOKEN"
	userTokenPrefix   = "xoxp-"
	identityTimeout   = 25 * time.Second
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "siftpane:", err)
		os.Exit(1)
	}
}

func run() error {
	versionFlag := flag.Bool("version", false, "print version and exit")
	demoFlag := flag.Bool("demo", false, "offline demonstration; isolated from real configuration")
	config := flag.String("config", "", "configuration path (credentials are stored separately)")
	account := flag.String("account", "", "saved account WORKSPACE_ID/USER_ID (default: last login)")
	login := flag.Bool("login", false, "read a user OAuth token securely and store in OS keyring")
	logout := flag.Bool("logout", false, "delete the saved token from the OS keyring")
	snapshot := flag.String("snapshot", "", "render offline review view (see docs/DEVELOPMENT.md)")
	width := flag.Int("width", ui.DefaultWidth, "snapshot columns")
	height := flag.Int("height", ui.DefaultHeight, "snapshot rows")
	flag.Parse()
	if *versionFlag {
		fmt.Println("siftpane " + version.String())
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	offline := *demoFlag || *snapshot != ""
	var svc core.Service
	var st *store.Store
	var err error
	if offline {
		svc = demo.New()
		st = &store.Store{Config: store.Config{Workspaces: map[string]*store.Workspace{}}}
	} else {
		path := *config
		if path == "" {
			path, err = store.DefaultPath()
			if err != nil {
				return err
			}
		}
		st, err = store.Open(path)
		if err != nil {
			return err
		}
		if *login {
			return loginToken(ctx, st, *account)
		}
		if *logout {
			return logoutAccount(st, *account)
		}
		accounts, loadErr := loadAccounts(ctx, st, *account)
		if loadErr != nil {
			return loadErr
		}
		_, err = tea.NewProgram(ui.NewAccounts(ctx, accounts, st), tea.WithContext(ctx)).Run()
		return err
	}
	authCtx, authCancel := context.WithTimeout(ctx, identityTimeout)
	defer authCancel()
	id, err := svc.Identity(authCtx)
	if err != nil {
		return err
	}
	m := ui.New(ctx, svc, id, st, offline)
	if *snapshot != "" {
		if *width < ui.MinWidth || *height < ui.MinHeight {
			return fmt.Errorf("snapshot requires at least %d columns and %d rows", ui.MinWidth, ui.MinHeight)
		}
		switch ui.SnapshotMode(*snapshot) {
		case ui.SnapshotRefreshDetails, ui.SnapshotCredentialRecovery, ui.SnapshotPartialThread, ui.SnapshotWorkspaces, ui.SnapshotAccountCompose, ui.SnapshotAccountFailed, ui.SnapshotReadability, ui.SnapshotCached, ui.SnapshotCacheError, ui.SnapshotCoverage, ui.SnapshotFollowed, ui.SnapshotAttention, ui.SnapshotMessages, ui.SnapshotCompose, ui.SnapshotPicker, ui.SnapshotSearch, ui.SnapshotThread, ui.SnapshotHelp, ui.SnapshotStartup, ui.SnapshotLoading, ui.SnapshotLoadError:
		default:
			return fmt.Errorf("unknown snapshot view")
		}
		// Feed real service results through Update without running the terminal renderer.
		if err = ui.LoadSnapshot(ctx, m, svc); err != nil {
			return err
		}
		fmt.Println(ui.Snapshot(m, *width, *height, ui.SnapshotMode(*snapshot)))
		return nil
	}
	other := id
	other.Workspace, other.WorkspaceName = "DEMO2", "Workshop · demo"
	accounts := []ui.Account{{Service: svc, Identity: id, Offline: true}, {Service: demo.New(), Identity: other, Offline: true}}
	_, err = tea.NewProgram(ui.NewAccounts(ctx, accounts, st), tea.WithContext(ctx)).Run()
	return err
}
