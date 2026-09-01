package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/galaxy-io/filament"
	cliapp "github.com/galaxy-io/filament/cmd/internal/cli/app"
)

type commandArgs struct {
	positionals []string
	flags       map[string][]string
}

func (a *cliApp) parseCommandArgs(args []string) (commandArgs, error) {
	parsed := commandArgs{flags: map[string][]string{}}
	booleans := a.booleanFlags()
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "--") {
			parsed.positionals = append(parsed.positionals, arg)
			continue
		}
		nameValue := strings.TrimPrefix(arg, "--")
		name, value, hasValue := strings.Cut(nameValue, "=")
		if name == "" {
			return parsed, fmt.Errorf("invalid empty flag")
		}
		if !hasValue {
			switch {
			case booleans[name]:
				value = "true"
				if i+1 < len(args) && (args[i+1] == "true" || args[i+1] == "false") {
					value = args[i+1]
					i++
				}
			case i+1 < len(args) && !strings.HasPrefix(args[i+1], "--"):
				value = args[i+1]
				i++
			default:
				return parsed, fmt.Errorf("--%s requires a value", name)
			}
		}
		parsed.flags[name] = append(parsed.flags[name], value)
	}
	if len(parsed.positionals) > 1 {
		return parsed, fmt.Errorf("unexpected argument %q", parsed.positionals[1])
	}
	return parsed, nil
}

func (a *cliApp) booleanFlags() map[string]bool {
	result := map[string]bool{"force": true, "refresh": true, "help": true}
	for _, spec := range a.catalog.Sources {
		for _, field := range spec.Config.Fields {
			if field.Type == filament.FieldBool {
				result["source-"+strings.ReplaceAll(field.Name, "_", "-")] = true
			}
		}
	}
	for _, spec := range a.catalog.Sinks {
		for _, field := range spec.Config.Fields {
			if field.Type == filament.FieldBool {
				result["sink-"+strings.ReplaceAll(field.Name, "_", "-")] = true
			}
		}
	}
	return result
}

func configPatchFromFlags(schema filament.ConfigSchema, scope filament.FieldScope, prefix string, flags map[string][]string, allowed map[string]bool, secretEnvironmentFlags bool) (cliapp.ConfigPatch, map[string]string, error) {
	return configPatchFromFieldList(cliapp.OrderedFields(schema, scope), prefix, flags, allowed, secretEnvironmentFlags)
}

func configPatchFromAllFlags(schema filament.ConfigSchema, prefix string, flags map[string][]string, allowed map[string]bool, secretEnvironmentFlags bool) (cliapp.ConfigPatch, map[string]string, error) {
	return configPatchFromFieldList(schema.Fields, prefix, flags, allowed, secretEnvironmentFlags)
}

func configPatchFromFieldList(fields []filament.ConfigField, prefix string, flags map[string][]string, allowed map[string]bool, secretEnvironmentFlags bool) (cliapp.ConfigPatch, map[string]string, error) {
	patch := cliapp.ConfigPatch{Values: map[string]any{}}
	unsetNames := map[string]string{}
	for _, field := range fields {
		name := prefix + strings.ReplaceAll(field.Name, "_", "-")
		allowed[name] = true
		unsetNames[name] = field.Name
		raw, direct := flagValue(flags, name)
		if cliapp.IsSecretField(field) && secretEnvironmentFlags {
			environmentFlag := name + "-env"
			allowed[environmentFlag] = true
			environmentName, fromEnvironment := flagValue(flags, environmentFlag)
			if direct && fromEnvironment {
				return patch, nil, fmt.Errorf("--%s and --%s cannot be combined", name, environmentFlag)
			}
			if fromEnvironment {
				normalized, err := cliapp.NormalizeEnvironmentName(environmentName)
				if err != nil {
					return patch, nil, fmt.Errorf("--%s: %w", environmentFlag, err)
				}
				patch.Values[field.Name] = "env:" + normalized
				continue
			}
		}
		if direct {
			value, err := cliapp.ParseFieldValue(field, raw)
			if err != nil {
				return patch, nil, fmt.Errorf("--%s: %w", name, err)
			}
			patch.Values[field.Name] = value
		}
	}
	return patch, unsetNames, nil
}

func firstPositional(args commandArgs) string {
	if len(args.positionals) == 0 {
		return ""
	}
	return args.positionals[0]
}

func lastFlag(flags map[string][]string, name string) string {
	values := flags[name]
	if len(values) == 0 {
		return ""
	}
	return values[len(values)-1]
}

func flagValue(flags map[string][]string, name string) (string, bool) {
	values, ok := flags[name]
	if !ok || len(values) == 0 {
		return "", false
	}
	return values[len(values)-1], true
}

func rejectUnknownFlags(flags map[string][]string, allowed map[string]bool) error {
	unknown := []string{}
	for name := range flags {
		if !allowed[name] {
			unknown = append(unknown, "--"+name)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return fmt.Errorf("unknown flag(s): %s", strings.Join(unknown, ", "))
	}
	return nil
}

func applyFieldUnsets(flags map[string][]string, targets map[string]func(), allowed map[string]bool) error {
	allowed["unset"] = true
	for _, raw := range flags["unset"] {
		for _, requested := range splitComma(raw) {
			requested = strings.TrimPrefix(requested, "--")
			unset, ok := targets[requested]
			if !ok {
				return fmt.Errorf("--unset: unknown or misplaced field %q", requested)
			}
			if _, alsoSet := flags[requested]; alsoSet {
				return fmt.Errorf("--%s and --unset %s cannot be combined", requested, requested)
			}
			unset()
		}
	}
	return nil
}

func splitComma(value string) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(item)
		if item != "" && !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	return result
}

func cloneMap[V any](source map[string]V) map[string]V {
	result := make(map[string]V, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}
