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
)

type cliApp struct {
	stdin      io.Reader
	stdout     io.Writer
	stderr     io.Writer
	configPath string
	catalog    connectorCatalog
	executeRun runExecutor
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
	switch args[0] {
	case "source", "sink":
		return a.runConnectionCommand(ctx, args[0], args[1:])
	case "pipeline":
		return a.runPipelineCommand(args[1:])
	case "config":
		return a.runConfigCommand(ctx, args[1:])
	case "run":
		return a.runCommand(ctx, args[1:])
	default:
		return fmt.Errorf("unknown command %q\n\n%s", args[0], rootHelp)
	}
}

func (a *cliApp) extractGlobalFlags(args []string) ([]string, error) {
	result := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		if args[i] == "--config" {
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--config requires a path")
			}
			a.configPath = args[i+1]
			i++
			continue
		}
		if strings.HasPrefix(args[i], "--config=") {
			a.configPath = strings.TrimPrefix(args[i], "--config=")
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
