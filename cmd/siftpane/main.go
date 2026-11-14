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
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "siftpane:", err)
		os.Exit(1)
	}
}

func run() error {
	demoFlag := flag.Bool("demo", false, "offline demonstration; isolated from real configuration")
	config := flag.String("config", "", "configuration path (credentials are stored separately)")
	login := flag.Bool("login", false, "read a user OAuth token securely and store in OS keyring")
	logout := flag.Bool("logout", false, "delete the saved token from the OS keyring")
	snapshot := flag.String("snapshot", "", "render offline review view: messages, compose, picker, thread, help")
	width := flag.Int("width", 100, "snapshot columns")
	height := flag.Int("height", 30, "snapshot rows")
	flag.Parse()
	if *logout {
		return keyring.Delete("siftpane", "user-token")
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
		token := os.Getenv("SIFTPANE_SLACK_TOKEN")
		if token == "" {
			token, err = keyring.Get("siftpane", "user-token")
			if err != nil {
				return fmt.Errorf("no user token: run siftpane --login, set SIFTPANE_SLACK_TOKEN, or use --demo")
			}
		}
		if !strings.HasPrefix(token, "xoxp-") {
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
	authCtx, authCancel := context.WithTimeout(ctx, 25*time.Second)
	defer authCancel()
	id, err := svc.Identity(authCtx)
	if err != nil {
		return err
	}
	m := ui.New(ctx, svc, id, st, offline)
	if *snapshot != "" {
		if *width < 40 || *height < 14 {
			return fmt.Errorf("snapshot requires at least 40 columns and 14 rows")
		}
		switch *snapshot {
		case "messages", "compose", "picker", "thread", "help":
		default:
			return fmt.Errorf("unknown snapshot view")
		}
		// Feed real service results through Update without running the terminal renderer.
		if err = ui.LoadSnapshot(ctx, m, svc); err != nil {
			return err
		}
		fmt.Println(ui.Snapshot(m, *width, *height, *snapshot))
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
	if !strings.HasPrefix(token, "xoxp-") {
		return fmt.Errorf("expected a user OAuth token beginning xoxp-; use the internal app setup in README")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	id, err := slack.New(token).Identity(ctx)
	if err != nil {
		return err
	}
	if err = keyring.Set("siftpane", "user-token", token); err != nil {
		return fmt.Errorf("OS keyring unavailable; use SIFTPANE_SLACK_TOKEN for this session instead (token was not written to disk)")
	}
	fmt.Println("Saved user token for", slack.Safe(id.WorkspaceName))
	return nil
}
