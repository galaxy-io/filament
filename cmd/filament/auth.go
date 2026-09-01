package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	authv1 "github.com/galaxy-io/filament/api/auth/v1"
	"github.com/galaxy-io/filament/api/auth/v1/authv1connect"
	cliauth "github.com/galaxy-io/filament/cmd/internal/cli/auth"
	"github.com/galaxy-io/filament/cmd/internal/cli/contexts"
)

func (a *cliApp) authCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Authenticate with a Filament deployment",
	}
	var server, clientID, clientSecret string
	login := &cobra.Command{
		Use:   "login",
		Short: "Log in with a service account",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.authLogin(cmd.Context(), server, clientID, clientSecret)
		},
	}
	login.Flags().StringVar(&server, "server", "", "Filament server `URL`")
	login.Flags().StringVar(&clientID, "client-id", "", "Service account client `ID`")
	login.Flags().StringVar(&clientSecret, "client-secret", "", "Service account client `SECRET`")
	_ = login.MarkFlagRequired("server")
	cmd.AddCommand(
		login,
		&cobra.Command{
			Use:   "status",
			Short: "Show the current context's authentication state",
			Args:  cobra.NoArgs,
			RunE: func(*cobra.Command, []string) error {
				return a.authStatus()
			},
		},
		&cobra.Command{
			Use:   "logout",
			Short: "Delete the current context's credentials",
			Args:  cobra.NoArgs,
			RunE: func(*cobra.Command, []string) error {
				return a.authLogout()
			},
		},
	)
	return cmd
}

// authLogin verifies service-account credentials against the server before
// anything is kept: a failed login leaves no profile and no context behind.
func (a *cliApp) authLogin(ctx context.Context, server, clientID, clientSecret string) error {
	server = strings.TrimRight(server, "/")
	parsed, err := url.ParseRequestURI(server)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return errors.New("--server must be an absolute HTTP(S) URL")
	}
	client := authv1connect.NewAuthServiceClient(http.DefaultClient, server)
	config, err := client.GetAuthConfig(ctx, connect.NewRequest(&authv1.GetAuthConfigRequest{}))
	if err != nil {
		return fmt.Errorf("reach %s: %w", server, err)
	}
	if config.Msg.GetIssuer() == "" {
		return fmt.Errorf("%s runs without authentication; there is nothing to log in to", server)
	}
	if clientID, clientSecret, err = a.promptCredentials(clientID, clientSecret); err != nil {
		return err
	}

	name := a.contextName
	if name == "" {
		name = parsed.Hostname()
	}
	store := cliauth.Store{Path: a.credentialsPath()}
	if err := store.Put(name, cliauth.Profile{
		Issuer:       config.Msg.GetIssuer(),
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scopes:       config.Msg.GetServiceAccountScopes(),
	}); err != nil {
		return err
	}
	token, err := cliauth.Source{Store: store, Profile: name}.Token(ctx)
	if err != nil {
		_ = store.Delete(name)
		return err
	}
	verify := connect.NewRequest(&authv1.ListServiceAccountsRequest{})
	verify.Header().Set("Authorization", "Bearer "+token)
	if _, err := client.ListServiceAccounts(ctx, verify); err != nil {
		_ = store.Delete(name)
		return fmt.Errorf("%s rejected the minted token: %w", server, err)
	}

	registry := a.contextRegistry()
	if err := registry.Set(name, contexts.Target{Kind: contexts.KindRemote, Endpoint: server, AuthProfile: name}); err != nil {
		_ = store.Delete(name)
		return err
	}
	if _, err := registry.Use(name); err != nil {
		return err
	}
	return printSuccess(a.statusWriter(), fmt.Sprintf("Logged in to %s as %s (context %s)", server, clientID, name))
}

func (a *cliApp) authStatus() error {
	current, err := a.contextRegistry().Resolve(a.contextName)
	if err != nil {
		return err
	}
	if current.Target.Kind == contexts.KindLocal {
		_, err := fmt.Fprintf(a.stdout, "Context %s is local; authentication is not used.\n", current.Name)
		return err
	}
	profile, err := cliauth.Store{Path: a.credentialsPath()}.Get(current.Target.AuthProfile)
	if err != nil {
		_, err := fmt.Fprintf(a.stdout, "Context %s is not logged in. Run filament auth login.\n", current.Name)
		return err
	}
	token := "none"
	if c := profile.Cache; c != nil {
		token = "expired"
		if time.Now().Before(c.ExpiresAt) {
			token = "valid until " + c.ExpiresAt.Local().Format("15:04:05")
		}
	}
	for _, pair := range [][2]string{
		{"Context", current.Name},
		{"Server", current.Target.Endpoint},
		{"Client ID", profile.ClientID},
		{"Token", token},
	} {
		if _, err := fmt.Fprintf(a.stdout, "%-9s  %s\n", pair[0], pair[1]); err != nil {
			return err
		}
	}
	return nil
}

func (a *cliApp) authLogout() error {
	current, err := a.contextRegistry().Resolve(a.contextName)
	if err != nil {
		return err
	}
	if current.Target.AuthProfile == "" {
		return fmt.Errorf("context %q holds no credentials", current.Name)
	}
	if err := (cliauth.Store{Path: a.credentialsPath()}).Delete(current.Target.AuthProfile); err != nil {
		return err
	}
	return printSuccess(a.statusWriter(), fmt.Sprintf("Logged out of context %s", current.Name))
}

// promptCredentials fills whichever credential flags were omitted. The secret
// is read without echo on a terminal.
func (a *cliApp) promptCredentials(clientID, clientSecret string) (string, string, error) {
	in := a.stdin
	if in == nil {
		in = os.Stdin
	}
	reader := bufio.NewReader(in)
	var err error
	if clientID == "" {
		if clientID, err = promptValue(reader, a.statusWriter(), "Client ID: "); err != nil {
			return "", "", err
		}
	}
	if clientSecret == "" {
		if clientSecret, err = promptSecret(in, reader, a.statusWriter(), "Client secret: "); err != nil {
			return "", "", err
		}
	}
	return clientID, clientSecret, nil
}

func promptValue(reader *bufio.Reader, out io.Writer, label string) (string, error) {
	if _, err := fmt.Fprint(out, label); err != nil {
		return "", err
	}
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	value := strings.TrimSpace(line)
	if value == "" {
		return "", errors.New("a value is required")
	}
	return value, nil
}

func promptSecret(in io.Reader, reader *bufio.Reader, out io.Writer, label string) (string, error) {
	file, ok := in.(*os.File)
	if !ok || !term.IsTerminal(int(file.Fd())) {
		return promptValue(reader, out, label)
	}
	if _, err := fmt.Fprint(out, label); err != nil {
		return "", err
	}
	secret, err := term.ReadPassword(int(file.Fd()))
	if _, printErr := fmt.Fprintln(out); printErr != nil {
		return "", printErr
	}
	if err != nil {
		return "", err
	}
	if len(secret) == 0 {
		return "", errors.New("a value is required")
	}
	return string(secret), nil
}

func (a *cliApp) credentialsPath() string {
	return filepath.Join(filepath.Dir(a.contextPath), "credentials.yaml")
}
