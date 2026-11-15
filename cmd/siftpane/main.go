// Command siftpane runs the focused Slack terminal client or its offline demo.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/zalando/go-keyring"
	"golang.org/x/term"

	"github.com/cliffom/siftpane/internal/core"
	"github.com/cliffom/siftpane/internal/demo"
	"github.com/cliffom/siftpane/internal/slack"
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
	login := flag.Bool("login", false, "read a user OAuth token securely and store in OS keyring")
	logout := flag.Bool("logout", false, "delete the saved token from the OS keyring")
	snapshot := flag.String("snapshot", "", "render offline review view: attention, messages, compose, picker, search, thread, help, startup, loading, load-error")
	width := flag.Int("width", ui.DefaultWidth, "snapshot columns")
	height := flag.Int("height", ui.DefaultHeight, "snapshot rows")
	flag.Parse()
	if *versionFlag {
		fmt.Println("siftpane " + version.String())
		return nil
	}
	if *logout {
		return keyring.Delete(credentialService, credentialAccount)
	}
	if *login {
		return loginToken()
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
		token := os.Getenv(tokenEnvironment)
		if token == "" {
			token, err = keyring.Get(credentialService, credentialAccount)
			if err != nil {
				return fmt.Errorf("no user token: run siftpane --login, set SIFTPANE_SLACK_TOKEN, or use --demo")
			}
		}
		if !strings.HasPrefix(token, userTokenPrefix) {
			return fmt.Errorf("expected an xoxp- user OAuth token; bot and app-level tokens are not supported")
		}
		svc = slack.New(token)
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
		case ui.SnapshotAttention, ui.SnapshotMessages, ui.SnapshotCompose, ui.SnapshotPicker, ui.SnapshotSearch, ui.SnapshotThread, ui.SnapshotHelp, ui.SnapshotStartup, ui.SnapshotLoading, ui.SnapshotLoadError:
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
	_, err = tea.NewProgram(m, tea.WithContext(ctx)).Run()
	return err
}

func loginToken() error {
	fmt.Fprint(os.Stderr, "User OAuth token (hidden): ")
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return fmt.Errorf("read token: %w", err)
	}
	token := strings.TrimSpace(string(b))
	if !strings.HasPrefix(token, userTokenPrefix) {
		return fmt.Errorf("expected a user OAuth token beginning xoxp-; use the internal app setup in README")
	}
	ctx, cancel := context.WithTimeout(context.Background(), identityTimeout)
	defer cancel()
	id, err := slack.New(token).Identity(ctx)
	if err != nil {
		return err
	}
	if err = keyring.Set(credentialService, credentialAccount, token); err != nil {
		return fmt.Errorf("OS keyring unavailable; use SIFTPANE_SLACK_TOKEN for this session instead (token was not written to disk)")
	}
	fmt.Println("Saved user token for", slack.Safe(id.WorkspaceName))
	return nil
}
