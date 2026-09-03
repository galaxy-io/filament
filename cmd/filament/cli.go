package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	cliapp "github.com/galaxy-io/filament/cmd/internal/cli/app"
	cliauth "github.com/galaxy-io/filament/cmd/internal/cli/auth"
	"github.com/galaxy-io/filament/cmd/internal/cli/contexts"
	climodel "github.com/galaxy-io/filament/cmd/internal/cli/model"
	dadorenderer "github.com/galaxy-io/filament/cmd/internal/cli/renderer/dado"
	textrenderer "github.com/galaxy-io/filament/cmd/internal/cli/renderer/text"
	"github.com/galaxy-io/filament/cmd/internal/cli/settings"
	"github.com/galaxy-io/filament/cmd/internal/cli/style"
	localtarget "github.com/galaxy-io/filament/cmd/internal/cli/target/local"
	remotetarget "github.com/galaxy-io/filament/cmd/internal/cli/target/remote"
)

type cliApp struct {
	stdin          io.Reader
	stopEmbedded   func()
	stdout         io.Writer
	stderr         io.Writer
	configPath     string
	contextPath    string
	contextName    string
	catalog        climodel.Catalog
	service        *cliapp.Service
	configOverride bool
	menuMode       bool
	layout         style.Layout
	target         contexts.NamedTarget
}

func (a *cliApp) run(ctx context.Context, args []string) error {
	defer func() {
		if a.stopEmbedded != nil {
			a.stopEmbedded()
		}
	}()
	saved, err := settings.Load(a.settingsPath())
	if err != nil {
		return err
	}
	if a.layout, err = style.ParseLayout(saved.Layout); err != nil {
		return fmt.Errorf("%s: %w", a.settingsPath(), err)
	}
	if value := os.Getenv("FILAMENT_LAYOUT"); value != "" {
		if a.layout, err = style.ParseLayout(value); err != nil {
			return fmt.Errorf("FILAMENT_LAYOUT: %w", err)
		}
	}
	args, err = a.extractGlobalFlags(args)
	if err != nil {
		return err
	}
	if err := a.printBanner(args); err != nil {
		return err
	}
	if a.renderer().CanHandle(args) {
		if err := a.initializeTarget(ctx); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(a.statusWriter()); err != nil {
			return err
		}
		return a.renderer().Run(ctx, args)
	}
	root := a.rootCommand()
	root.SetArgs(args)
	return root.ExecuteContext(ctx)
}

// renderer builds the interactive renderer for the current target, if any.
func (a *cliApp) renderer() *dadorenderer.Renderer {
	return dadorenderer.New(dadorenderer.Options{
		Stdin: a.stdin, Stdout: a.stdout, Stderr: a.statusWriter(),
		Service: a.service, Catalog: a.catalog, OpenConfigurationEditor: a.editConfig,
		TargetName: a.target.Name, MenuMode: a.menuMode, Layout: a.layout, SaveLayout: a.saveLayout,
	})
}

func (a *cliApp) initializeTarget(ctx context.Context) error {
	selected := contexts.NamedTarget{
		Name:   "local",
		Target: contexts.Target{Kind: contexts.KindLocal, ConfigPath: a.configPath},
	}
	if a.contextPath != "" {
		var err error
		selected, err = a.contextRegistry().Resolve(a.contextName)
		if err != nil {
			return err
		}
	}
	if a.configOverride {
		if selected.Target.Kind != contexts.KindLocal {
			return fmt.Errorf("--config can only override a local context")
		}
		selected.Target.ConfigPath = a.configPath
	}
	a.target = selected
	if selected.Target.Kind == contexts.KindRemote {
		options := remotetarget.Options{Endpoint: selected.Target.Endpoint}
		if profile := selected.Target.AuthProfile; profile != "" {
			options.Tokens = cliauth.Source{Store: cliauth.Store{Path: a.credentialsPath()}, Profile: profile}
		}
		a.service = cliapp.NewService(remotetarget.NewTarget(options))
	} else {
		a.configPath = selected.Target.ConfigPath
		endpoint, secrets, stop, err := a.startEmbedded(ctx)
		if err != nil {
			return err
		}
		a.stopEmbedded = stop
		target := localtarget.NewTarget(
			localtarget.Store{Path: a.configPath},
			remotetarget.NewTarget(remotetarget.Options{Endpoint: endpoint}),
			secrets,
		)
		if err := target.ApplyIfChanged(ctx, a.markerPath()); err != nil {
			return fmt.Errorf("apply %s: %w", a.configPath, err)
		}
		a.service = cliapp.NewService(target)
	}
	catalog, err := a.service.Catalog(ctx)
	if err != nil {
		return err
	}
	a.catalog = catalog
	return nil
}

func (a *cliApp) extractGlobalFlags(args []string) ([]string, error) {
	result := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		if args[i] == "--config" {
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--config requires a path")
			}
			a.configPath = args[i+1]
			a.configOverride = true
			i++
			continue
		}
		if strings.HasPrefix(args[i], "--config=") {
			a.configPath = strings.TrimPrefix(args[i], "--config=")
			a.configOverride = true
			continue
		}
		if args[i] == "--context" {
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--context requires a name")
			}
			a.contextName = args[i+1]
			i++
			continue
		}
		if args[i] == "--layout" {
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--layout requires boxed or plain")
			}
			layout, err := style.ParseLayout(args[i+1])
			if err != nil {
				return nil, err
			}
			a.layout = layout
			i++
			continue
		}
		if strings.HasPrefix(args[i], "--layout=") {
			layout, err := style.ParseLayout(strings.TrimPrefix(args[i], "--layout="))
			if err != nil {
				return nil, err
			}
			a.layout = layout
			continue
		}
		if args[i] == "-i" || args[i] == "--interactive" {
			a.menuMode = true
			continue
		}
		if strings.HasPrefix(args[i], "--context=") {
			a.contextName = strings.TrimPrefix(args[i], "--context=")
			if a.contextName == "" {
				return nil, fmt.Errorf("--context requires a name")
			}
			continue
		}
		result = append(result, args[i])
	}
	return result, nil
}

func (a *cliApp) settingsPath() string { return filepath.Join(a.stateDir(), "settings.yaml") }

// saveLayout persists the layout as the user's preference.
func (a *cliApp) saveLayout(layout style.Layout) error {
	saved, err := settings.Load(a.settingsPath())
	if err != nil {
		return err
	}
	saved.Layout = layout.String()
	a.layout = layout
	return settings.Save(a.settingsPath(), saved)
}

// text renders results in the selected table layout.
func (a *cliApp) text() textrenderer.Renderer {
	return textrenderer.New(a.stdout, a.layout)
}

func printSuccess(w io.Writer, message string) error {
	_, err := fmt.Fprintln(w, message+".")
	return err
}

func (a *cliApp) statusWriter() io.Writer {
	if a.stderr != nil {
		return a.stderr
	}
	return a.stdout
}

func (a *cliApp) confirmDelete(kind, name string) (bool, error) {
	in := a.stdin
	if in == nil {
		in = os.Stdin
	}
	out := a.statusWriter()
	if _, err := fmt.Fprintf(out, "Delete %s %q? [y/N] ", kind, name); err != nil {
		return false, err
	}
	answer, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, fmt.Errorf("read deletion confirmation: %w", err)
	}
	answer = strings.ToLower(strings.TrimSpace(answer))
	if errors.Is(err, io.EOF) && answer == "" {
		return false, fmt.Errorf("deletion confirmation requires input; rerun with --force to bypass")
	}
	if answer == "y" || answer == "yes" {
		return true, nil
	}
	_, err = fmt.Fprintln(out, "Cancelled.")
	return false, err
}

func pastTense(operation string) string {
	if operation == "edit" {
		return "Updated"
	}
	return "Created"
}
