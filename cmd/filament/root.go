package main

import (
	"context"
	"fmt"
	"strings"
	"text/template"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/galaxy-io/filament/cmd/internal/cli/style"
	"github.com/galaxy-io/filament/version"
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
		RunE:          a.menu(""),
	}
	root.SetIn(a.stdin)
	root.SetOut(a.stdout)
	root.SetErr(a.statusWriter())

	// Parsed by extractGlobalFlags before cobra runs so they work in any position.
	root.PersistentFlags().String("config", a.configPath, "Read configuration from `PATH`")
	root.PersistentFlags().String("context", "", "Run against context `NAME`")
	root.PersistentFlags().BoolP("interactive", "i", false, "Open the interactive menu")
	root.PersistentFlags().String("layout", "boxed", "Draw tables as `boxed` or plain (also FILAMENT_LAYOUT)")
	root.SetVersionTemplate("filament {{.Version}}\n")

	a.installHelpStyle(root)
	root.AddCommand(
		a.connectionCommand("source"),
		a.connectionCommand("sink"),
		a.pipelineCommand(),
		a.runCommand(),
		a.configCommand(),
		a.upCommand(),
		a.downCommand(),
		a.engineCommand(),
		a.statusCommand(),
		a.contextCommand(),
		a.authCommand(),
		a.upgradeCommand(),
		a.versionCommand(),
	)
	return root
}

func (a *cliApp) prepareTarget(cmd *cobra.Command, _ []string) error {
	return a.initializeTarget(cmd.Context())
}

// targetGroup makes group's subcommands run against the selected target and
// lets the group itself open its interactive menu or print help.
func (a *cliApp) targetGroup(group *cobra.Command, section string) *cobra.Command {
	group.RunE = a.menu(section)
	group.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
		if cmd == group {
			return nil
		}
		return a.prepareTarget(cmd, nil)
	}
	return group
}

// withoutTarget overrides a group's target hook for a subcommand that does
// not need a running deployment.
func withoutTarget(*cobra.Command, []string) error { return nil }

// dynamicCommand owns its own flag parsing because its flags come from
// connector schemas. Its use line lists each form, separated by " | ".
func (a *cliApp) dynamicCommand(use, short string, help func(context.Context, *cobra.Command, []string) error, run func(context.Context, []string) error) *cobra.Command {
	cmd := &cobra.Command{
		Use:                   use,
		Short:                 short,
		DisableFlagParsing:    true,
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if helpRequested(args) {
				return help(cmd.Context(), cmd, args)
			}
			return run(cmd.Context(), args)
		},
	}
	cmd.SetHelpFunc(func(cmd *cobra.Command, _ []string) {
		_ = help(cmd.Context(), cmd, nil)
	})
	return cmd
}

const usageTemplate = `{{heading "Usage:"}}{{if and .Runnable (not .HasAvailableSubCommands)}}
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
	rows := []flagRow{}
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
		rows = append(rows, flagRow{plain: plain, painted: painted, usage: usage})
	})
	return strings.TrimRight(alignFlagRows(rows), "\n")
}

// flagRow is one help line: the flag as plain text for measuring, as painted
// text for printing, and its usage.
type flagRow struct {
	plain, painted, usage string
}

// alignFlagRows indents rows and lines their usage up in one column.
func alignFlagRows(rows []flagRow) string {
	width := 0
	for _, row := range rows {
		width = max(width, len(row.plain))
	}
	var out strings.Builder
	for _, row := range rows {
		out.WriteString("  " + row.painted + strings.Repeat(" ", width-len(row.plain)) + "   " + row.usage + "\n")
	}
	return out.String()
}
