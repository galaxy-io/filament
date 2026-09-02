package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/spf13/cobra"

	cliauth "github.com/galaxy-io/filament/cmd/internal/cli/auth"
	"github.com/galaxy-io/filament/cmd/internal/cli/contexts"
)

func (a *cliApp) statusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the current context and its deployment state",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.status(cmd.Context())
		},
	}
}

func (a *cliApp) status(ctx context.Context) error {
	current, err := a.contextRegistry().Resolve(a.contextName)
	if err != nil {
		return err
	}
	rows := [][2]string{
		{"Context", current.Name},
		{"Kind", string(current.Target.Kind)},
	}
	if current.Target.Kind == contexts.KindRemote {
		rows = append(rows, [2]string{"Server", current.Target.Endpoint})
		auth := "none"
		if profile := current.Target.AuthProfile; profile != "" {
			auth = "profile " + profile
			if stored, err := (cliauth.Store{Path: a.credentialsPath()}).Get(profile); err == nil {
				auth += " (token none)"
				if c := stored.Cache; c != nil {
					if time.Now().Before(c.ExpiresAt) {
						auth = fmt.Sprintf("profile %s (token valid until %s)", profile, c.ExpiresAt.Local().Format("15:04:05"))
					} else {
						auth = "profile " + profile + " (token expired)"
					}
				}
			}
		}
		rows = append(rows,
			[2]string{"Auth", auth},
			[2]string{"Reachable", reachable(ctx, current.Target.Endpoint)},
		)
	} else {
		configPath := current.Target.ConfigPath
		if a.configOverride {
			configPath = a.configPath
		}
		rows = append(rows, [2]string{"Config", configPath})
		if state, running := a.readUpState(); running {
			deployment := fmt.Sprintf("running (pid %d)", state.PID)
			if state.Config != configPath {
				deployment += " serving " + state.Config
			}
			rows = append(rows,
				[2]string{"Deployment", deployment},
				[2]string{"UI", "http://" + state.Addr},
				[2]string{"Log", a.upLogPath()},
			)
		} else {
			rows = append(rows, [2]string{"Deployment", "per-command; start a persistent one with filament up"})
		}
	}
	for _, pair := range rows {
		if _, err := fmt.Fprintf(a.stdout, "%-10s  %s\n", pair[0], pair[1]); err != nil {
			return err
		}
	}
	return nil
}

// reachable pings the deployment without authenticating; any HTTP answer
// counts.
func reachable(ctx context.Context, endpoint string) string {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return "no (" + err.Error() + ")"
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return "no (" + err.Error() + ")"
	}
	_ = response.Body.Close()
	return "yes"
}
