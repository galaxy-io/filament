package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	cliauth "github.com/galaxy-io/filament/cmd/internal/cli/auth"
	"github.com/galaxy-io/filament/cmd/internal/cli/contexts"
)

func (a *cliApp) statusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the current context and its deployment",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			return a.status()
		},
	}
}

func (a *cliApp) status() error {
	current, err := a.contextRegistry().Resolve(a.contextName)
	if err != nil {
		return err
	}
	rows := [][2]string{{"Context", current.Name}, {"Kind", string(current.Target.Kind)}}
	if current.Target.Kind == contexts.KindRemote {
		reachable := "no"
		if answers(current.Target.Endpoint) {
			reachable = "yes"
		}
		rows = append(rows,
			[2]string{"Server", current.Target.Endpoint},
			[2]string{"Auth", a.authSummary(current.Target.AuthProfile)},
			[2]string{"Reachable", reachable},
		)
	} else {
		configPath := current.Target.ConfigPath
		if a.configOverride {
			configPath = a.configPath
		}
		rows = append(rows, [2]string{"Config", configPath}, [2]string{"State", a.dbPath()})
		if state, running := a.readUpState(); running {
			rows = append(rows,
				[2]string{"Deployment", fmt.Sprintf("running (pid %d)", state.PID)},
				[2]string{"UI", "http://" + state.Addr},
				[2]string{"Log", a.upLogPath()},
			)
		} else {
			rows = append(rows, [2]string{"Deployment", "per command; keep one running with filament up"})
		}
	}
	for _, pair := range rows {
		if _, err := fmt.Fprintf(a.stdout, "%-10s  %s\n", pair[0], pair[1]); err != nil {
			return err
		}
	}
	return nil
}

func (a *cliApp) authSummary(profile string) string {
	if profile == "" {
		return "none"
	}
	stored, err := (cliauth.Store{Path: a.credentialsPath()}).Get(profile)
	switch {
	case errors.Is(err, cliauth.ErrProfileNotFound):
		return "profile " + profile + " (missing credentials)"
	case err != nil:
		return "profile " + profile + " (credentials unreadable)"
	}
	switch {
	case stored.Cache == nil:
		return "profile " + profile + " (no token)"
	case time.Now().Before(stored.Cache.ExpiresAt):
		return fmt.Sprintf("profile %s (token valid until %s)", profile, stored.Cache.ExpiresAt.Local().Format("15:04:05"))
	default:
		return "profile " + profile + " (token expired)"
	}
}
