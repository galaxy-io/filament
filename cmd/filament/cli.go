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

	"github.com/spf13/cobra"

	cliapp "github.com/galaxy-io/filament/cmd/internal/cli/app"
	cliauth "github.com/galaxy-io/filament/cmd/internal/cli/auth"
	"github.com/galaxy-io/filament/cmd/internal/cli/contexts"
	climodel "github.com/galaxy-io/filament/cmd/internal/cli/model"
	dadorenderer "github.com/galaxy-io/filament/cmd/internal/cli/renderer/dado"
	textrenderer "github.com/galaxy-io/filament/cmd/internal/cli/renderer/text"
	"github.com/galaxy-io/filament/cmd/internal/cli/settings"
	"github.com/galaxy-io/filament/cmd/internal/cli/style"
	remotetarget "github.com/galaxy-io/filament/cmd/internal/cli/target/remote"
)

type cliApp struct {
	stdin          io.Reader
	stopEngine     func()
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
		if a.stopEngine != nil {
			a.stopEngine()
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
	if a.catalog.Sources == nil {
		a.catalog = a.loadCatalog()
	}
	if err := a.printBanner(args); err != nil {
		return err
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

// openInteractive hands the terminal to the interactive renderer at section,
// or the main menu when section is empty. The target must be initialized.
func (a *cliApp) openInteractive(ctx context.Context, section, operation string) error {
	if _, err := fmt.Fprintln(a.statusWriter()); err != nil {
		return err
	}
	return a.renderer().Open(ctx, section, operation)
}

// menu opens section's interactive menu under --interactive on a terminal,
// and prints the command's help otherwise.
func (a *cliApp) menu(section string) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 || !a.menuMode || !a.renderer().Interactive() {
			return cmd.Help()
		}
		if err := a.initializeTarget(cmd.Context()); err != nil {
			return err
		}
		return a.openInteractive(cmd.Context(), section, "")
	}
}

// selectContext resolves the context this invocation runs against, with a
// --config override applied to a local context.
func (a *cliApp) selectContext() (contexts.NamedTarget, error) {
	selected := contexts.NamedTarget{
		Name:   "local",
		Target: contexts.Target{Kind: contexts.KindLocal, ConfigPath: a.configPath},
	}
	if a.contextPath != "" {
		var err error
		if selected, err = a.contextRegistry().Resolve(a.contextName); err != nil {
			return selected, err
		}
	}
	if a.configOverride {
		if selected.Target.Kind != contexts.KindLocal {
			return selected, fmt.Errorf("--config can only override a local context")
		}
		selected.Target.ConfigPath = a.configPath
	}
	return selected, nil
}

func (a *cliApp) initializeTarget(ctx context.Context) error {
	selected, err := a.selectContext()
	if err != nil {
		return err
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
		target, stop, err := a.openLocal(ctx)
		if err != nil {
			return err
		}
		a.stopEngine = stop
		a.service = cliapp.NewService(target)
	}
	catalog, err := a.service.Catalog(ctx)
	if err != nil {
		return err
	}
	a.catalog = catalog
	if selected.Target.Kind == contexts.KindLocal {
		a.saveCatalog(catalog)
	}
	return nil
}

// extractGlobalFlags removes the global flags from any position. Dynamic
// commands parse their own flags, so cobra cannot see globals that follow them.
func (a *cliApp) extractGlobalFlags(args []string) ([]string, error) {
	values := map[string]func(string) error{
		"config": func(value string) error {
			a.configPath, a.configOverride = value, true
			return nil
		},
		"context": func(value string) error {
			a.contextName = value
			return nil
		},
		"layout": func(value string) error {
			layout, err := style.ParseLayout(value)
			a.layout = layout
			return err
		},
	}
	result := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		if args[i] == "-i" || args[i] == "--interactive" {
			a.menuMode = true
			continue
		}
		name, value, inline := strings.Cut(strings.TrimPrefix(args[i], "--"), "=")
		set, global := values[name]
		if !global || !strings.HasPrefix(args[i], "--") {
			result = append(result, args[i])
			continue
		}
		if !inline {
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--%s requires a value", name)
			}
			i++
			value = args[i]
		}
		if value == "" {
			return nil, fmt.Errorf("--%s requires a value", name)
		}
		if err := set(value); err != nil {
			return nil, err
		}
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
	return a.confirm(fmt.Sprintf("Delete %s %q?", kind, name))
}

// confirm asks a yes/no question on the status writer and reads one line of
// stdin. Empty input on a closed stdin is an error, not a refusal, so scripts
// learn to pass --force rather than silently no-op.
func (a *cliApp) confirm(question string) (bool, error) {
	in := a.stdin
	if in == nil {
		in = os.Stdin
	}
	out := a.statusWriter()
	if _, err := fmt.Fprintf(out, "%s [y/N] ", question); err != nil {
		return false, err
	}
	answer, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, fmt.Errorf("read confirmation: %w", err)
	}
	answer = strings.ToLower(strings.TrimSpace(answer))
	if errors.Is(err, io.EOF) && answer == "" {
		return false, fmt.Errorf("confirmation requires input; rerun with --force to bypass")
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
