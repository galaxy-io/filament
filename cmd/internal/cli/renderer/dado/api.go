// Package dado implements Filament's interactive terminal renderer.
package dado

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/atterpac/dado/inline"

	"github.com/galaxy-io/filament"
	cliapp "github.com/galaxy-io/filament/cmd/internal/cli/app"
	"github.com/galaxy-io/filament/cmd/internal/cli/model"
	"github.com/galaxy-io/filament/cmd/internal/cli/style"
)

// Renderer drives Dado forms using target-neutral application operations.
type Renderer struct {
	stdin                   io.Reader
	stdout                  io.Writer
	stderr                  io.Writer
	configPath              string
	catalog                 model.Catalog
	service                 *cliapp.Service
	openConfigurationEditor func(context.Context) error
	targetName              string
	interactiveRenderer     *inline.Renderer
	theme                   inline.InlineTheme
	paint                   style.Painter
	menuMode                bool
	layout                  style.Layout
	saveLayout              func(style.Layout) error
	noticeText              string
	noticeOK                bool
}

// Options configures an interactive renderer.
type Options struct {
	Stdin                   io.Reader
	Stdout                  io.Writer
	Stderr                  io.Writer
	Service                 *cliapp.Service
	Catalog                 model.Catalog
	OpenConfigurationEditor func(context.Context) error
	TargetName              string
	// MenuMode opens interactive menus for menu-shaped invocations. Off, only
	// operations render interactively.
	MenuMode bool
	// Layout draws menus and tables boxed or plain, matching the list commands.
	Layout style.Layout
	// SaveLayout persists a layout chosen in the Settings menu; nil hides it.
	SaveLayout func(style.Layout) error
}

// New constructs an interactive renderer for the selected target.
func New(options Options) *Renderer {
	configPath := ""
	if options.Service != nil {
		configPath = options.Service.ConfigurationLocation()
	}
	status := options.Stderr
	if status == nil {
		status = options.Stdout
	}
	dark := style.Dark(options.Stdin, status)
	return &Renderer{
		stdin: options.Stdin, stdout: options.Stdout, stderr: options.Stderr,
		configPath: configPath, catalog: options.Catalog, service: options.Service,
		openConfigurationEditor: options.OpenConfigurationEditor,
		targetName:              options.TargetName,
		menuMode:                options.MenuMode,
		layout:                  options.Layout,
		saveLayout:              options.SaveLayout,
		theme:                   filamentTheme(dark),
		paint:                   style.New(status),
	}
}

// Interactive reports whether both terminal streams support interactive rendering.
func (r *Renderer) Interactive() bool {
	return r.interactiveAvailable()
}

// CanHandle reports whether args identify an interactive entry point and both
// terminal streams support interactive rendering.
func (r *Renderer) CanHandle(args []string) bool {
	entry, routed := interactiveEntryForArgs(args)
	if !routed || !r.interactiveAvailable() {
		return false
	}
	// Menus open only under --interactive; operations (wizards, runs) route
	// interactively regardless.
	return r.menuMode || entry.operation != ""
}

// Run opens the interactive renderer at the entry point selected by args.
func (r *Renderer) Run(ctx context.Context, args []string) error {
	entry, ok := interactiveEntryForArgs(args)
	if !ok {
		return nil
	}
	return r.runInteractiveAt(ctx, entry)
}

func (r *Renderer) statusWriter() io.Writer {
	if r.stderr != nil {
		return r.stderr
	}
	return r.stdout
}

func (r *Renderer) connectorNames(kind string) []string {
	if kind == "sink" {
		return r.catalog.SinkNames()
	}
	return r.catalog.SourceNames()
}

func (r *Renderer) connectionSchema(kind, connector string) (filament.ConfigSchema, error) {
	return r.catalog.ConnectionSchema(kind, connector)
}

// validateName only requires a name; the deployment owns any further rule.
func validateName(kind, name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("%s name is required", kind)
	}
	return nil
}

func connectionMap(kind string, doc model.Document) map[string]model.Connection {
	if kind == "sink" {
		return doc.Sinks
	}
	return doc.Sources
}

func ensureConnectionUnreferenced(kind, name string, doc model.Document) error {
	for pipelineName, pipeline := range doc.Pipelines {
		if (kind == "source" && pipeline.Source.Ref == name) || (kind == "sink" && pipeline.Sink.Ref == name) {
			return fmt.Errorf("%s %q is referenced by pipeline %q; delete or edit that pipeline first", kind, name, pipelineName)
		}
	}
	return nil
}

func sortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func cloneMap[V any](source map[string]V) map[string]V {
	result := make(map[string]V, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
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
