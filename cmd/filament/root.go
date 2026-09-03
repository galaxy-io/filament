package main

import (
	"context"
	"fmt"
	"strings"
	"text/template"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/galaxy-io/filament/cmd/internal/cli/style"
	"github.com/galaxy-io/filament/cmd/internal/version"
)

func init() {
	cobra.EnableCommandSorting = false
}

func (a *cliApp) rootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:           "filament",
		Long:          "Filament configures and runs data pipelines.",
		Version:       version.String(),
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetIn(a.stdin)
	root.SetOut(a.stdout)
	root.SetErr(a.statusWriter())

	// Parsed by extractGlobalFlags before cobra runs so they work in any position.
	root.PersistentFlags().String("config", a.configPath, "Read configuration from `PATH`")
	root.PersistentFlags().String("context", "", "Run against context `NAME`")
	root.SetVersionTemplate("filament {{.Version}}\n")

	a.installHelpStyle(root)
	root.AddCommand(
		a.connectionCommand("source"),
		a.connectionCommand("sink"),
		a.pipelineCommand(),
		a.runCommandDefinition(),
		a.configCommand(),
		a.upCommand(),
		a.downCommand(),
		a.statusCommand(),
		a.contextCommand(),
		a.authCommand(),
		a.versionCommand(),
	)
	return root
}

func (a *cliApp) prepareTarget(cmd *cobra.Command, _ []string) error {
	return a.initializeTarget(cmd.Context())
}

// dynamicCommand owns its own flag parsing because its flags come from connector schemas.
func (a *cliApp) dynamicCommand(use, short string, help, run func(context.Context, []string) error) *cobra.Command {
	cmd := &cobra.Command{
		Use:                   use,
		Short:                 short,
		DisableFlagParsing:    true,
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if helpRequested(args) {
				return help(cmd.Context(), args)
			}
			return run(cmd.Context(), args)
		},
	}
	cmd.SetHelpFunc(func(cmd *cobra.Command, _ []string) {
		_ = help(cmd.Context(), nil)
	})
	return cmd
}

const usageTemplate = `{{heading "Usage:"}}{{if .Runnable}}
  {{useLine .}}{{end}}{{if .HasAvailableSubCommands}}
  {{literal .CommandPath}} {{muted "[command]"}}{{end}}{{if .HasAvailableSubCommands}}

{{heading "Available Commands:"}}{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{literal (rpad .Name .NamePadding)}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

{{heading "Flags:"}}
{{flagUsages .LocalFlags}}{{end}}{{if .HasAvailableInheritedFlags}}

{{heading "Global Flags:"}}
{{flagUsages .InheritedFlags}}{{end}}{{if .HasAvailableSubCommands}}

{{muted (printf "Use \"%s [command] --help\" for more information about a command." .CommandPath)}}{{end}}
`

// installHelpStyle colours cobra's help the way cargo does: headings, literals, placeholders.
// The template is private to this root so parallel apps never share cobra's global func map.
func (a *cliApp) installHelpStyle(root *cobra.Command) {
	p := style.New(a.stdout)
	funcs := template.FuncMap{
		"heading": p.Bold,
		"literal": p.Accent,
		"muted":   p.Muted,
		"rpad":    func(text string, width int) string { return fmt.Sprintf("%-*s", width, text) },
		"useLine": func(cmd *cobra.Command) string {
			path := cmd.CommandPath()
			return p.Accent(path) + p.Muted(strings.TrimPrefix(cmd.UseLine(), path))
		},
		"flagUsages": func(flags *pflag.FlagSet) string { return flagUsages(p, flags) },
	}
	usage := template.Must(template.New("usage").Funcs(funcs).Parse(usageTemplate))
	root.SetUsageFunc(func(cmd *cobra.Command) error {
		return usage.Execute(cmd.OutOrStdout(), cmd)
	})
}

func flagUsages(p style.Painter, flags *pflag.FlagSet) string {
	type entry struct {
		plain, painted, usage string
	}
	entries := []entry{}
	flags.VisitAll(func(flag *pflag.Flag) {
		if flag.Hidden {
			return
		}
		placeholder, usage := pflag.UnquoteUsage(flag)
		short, name := "    ", "--"+flag.Name
		if flag.Shorthand != "" {
			short = "-" + flag.Shorthand + ", "
		}
		plain, painted := short+name, short+p.Accent(name)
		if placeholder != "" {
			plain += " " + placeholder
			painted += " " + p.Muted(placeholder)
		}
		if flag.DefValue != "" && flag.DefValue != "false" && flag.DefValue != "0" && flag.DefValue != "[]" {
			usage += " " + p.Muted("(default "+flag.DefValue+")")
		}
		entries = append(entries, entry{plain: plain, painted: painted, usage: usage})
	})
	width := 0
	for _, item := range entries {
		width = max(width, len(item.plain))
	}
	var out strings.Builder
	for _, item := range entries {
		out.WriteString("  " + item.painted + strings.Repeat(" ", width-len(item.plain)) + "   " + item.usage + "\n")
	}
	return strings.TrimRight(out.String(), "\n")
}
