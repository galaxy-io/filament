package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	cliapp "github.com/galaxy-io/filament/cmd/internal/cli/app"
	"github.com/galaxy-io/filament/cmd/internal/cli/contexts"
	climodel "github.com/galaxy-io/filament/cmd/internal/cli/model"
	localtarget "github.com/galaxy-io/filament/cmd/internal/cli/target/local"
)

type cliApp struct {
	stdin          io.Reader
	stdout         io.Writer
	stderr         io.Writer
	configPath     string
	contextPath    string
	contextName    string
	catalog        climodel.Catalog
	service        *cliapp.Service
	configOverride bool
	target         contexts.NamedTarget
}

func (a *cliApp) run(ctx context.Context, args []string) error {
	args, err := a.extractGlobalFlags(args)
	if err != nil {
		return err
	}
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		_, err := fmt.Fprint(a.stdout, rootHelp)
		return err
	}
	if args[0] == "context" {
		return a.runContextCommand(args[1:])
	}
	switch args[0] {
	case "source", "sink", "pipeline", "config", "run":
	default:
		return fmt.Errorf("unknown command %q\n\n%s", args[0], rootHelp)
	}
	if err := a.initializeTarget(ctx); err != nil {
		return err
	}
	switch args[0] {
	case "source", "sink":
		return a.runConnectionCommand(ctx, args[0], args[1:])
	case "pipeline":
		return a.runPipelineCommand(ctx, args[1:])
	case "config":
		return a.runConfigCommand(ctx, args[1:])
	case "run":
		return a.runCommand(ctx, args[1:])
	}
	return nil
}

type unimplementedTargetError struct {
	kind contexts.Kind
}

func (e *unimplementedTargetError) Error() string {
	return fmt.Sprintf("%s target is not implemented", e.kind)
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
	if selected.Target.Kind != contexts.KindLocal {
		return &unimplementedTargetError{kind: selected.Target.Kind}
	}
	a.configPath = selected.Target.ConfigPath
	target := localtarget.NewTarget(localtarget.Store{Path: a.configPath}, a.catalog)
	a.service = cliapp.NewService(target)
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

func parseForceFlag(flags map[string][]string) (bool, error) {
	raw, present := flagValue(flags, "force")
	if !present {
		return false, nil
	}
	force, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("--force must be true or false")
	}
	return force, nil
}

func pastTense(operation string) string {
	if operation == "edit" {
		return "Updated"
	}
	return "Created"
}
