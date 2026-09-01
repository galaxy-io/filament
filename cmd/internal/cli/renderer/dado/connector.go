package dado

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/atterpac/dado/inline"

	"github.com/galaxy-io/filament"
	cliapp "github.com/galaxy-io/filament/cmd/internal/cli/app"
	"github.com/galaxy-io/filament/cmd/internal/cli/model"
)

type schemaWizard struct {
	fields []schemaWizardField
}

type schemaWizardField struct {
	schema  filament.ConfigField
	text    string
	boolean bool
	list    []string
	fields  []schemaWizardField
}

type schemaWizardStep struct {
	id    string
	label string
	path  string
	field *schemaWizardField
}

func (r *Renderer) manageConnections(ctx context.Context, kind string) error {
	for {
		listed, err := r.service.Connections(ctx, kind)
		if err != nil {
			return err
		}
		options := []interactiveOption{{label: "+ Create " + kind, value: "__create__", tone: inline.ChoiceToneSuccess}}
		for _, connection := range listed.Items {
			options = append(options, interactiveOption{label: connection.Name + "  ·  " + connection.Connector, value: connection.Name})
		}
		options = append(options, interactiveOption{label: "Back", value: interactiveBack})
		selected, err := r.chooseInteractive(ctx, strings.ToUpper(kind[:1])+kind[1:]+"s", "Create or manage saved connections", options)
		if interactiveCancelled(err) || selected == interactiveBack {
			return nil
		}
		if err != nil {
			return err
		}
		if selected == "__create__" {
			if err := r.connectionWizard(ctx, kind, "", nil); err != nil && !interactiveCancelled(err) {
				if showErr := r.showInteractiveMessage(ctx, "Unable to create "+kind, err.Error()); showErr != nil && !interactiveCancelled(showErr) {
					return showErr
				}
			}
			continue
		}
		doc, err := r.service.Configuration(ctx)
		if err != nil {
			return err
		}
		if err := r.manageConnection(ctx, kind, selected, doc); err != nil && !interactiveCancelled(err) {
			if showErr := r.showInteractiveMessage(ctx, "Unable to manage "+kind, err.Error()); showErr != nil && !interactiveCancelled(showErr) {
				return showErr
			}
		}
	}
}

func (r *Renderer) manageConnection(ctx context.Context, kind, name string, doc model.Document) error {
	connection := connectionMap(kind, doc)[name]
	options := []interactiveOption{{label: "View", value: "view"}, {label: "Edit", value: "edit"}}
	if kind == "source" {
		options = append(options, interactiveOption{label: "Discover resources", value: "discover"})
	}
	options = append(options,
		interactiveOption{label: "Delete", value: "delete"},
		interactiveOption{label: "Back", value: interactiveBack},
	)
	action, err := r.chooseInteractive(ctx, name, fmt.Sprintf("%s connection using %s", kind, connection.Type), options)
	if err != nil || action == interactiveBack {
		return err
	}
	switch action {
	case "view":
		schema, err := r.connectionSchema(kind, connection.Type)
		if err != nil {
			return err
		}
		return r.showInteractiveMessage(ctx, name, connectionDescription(connection, schema))
	case "edit":
		return r.connectionWizard(ctx, kind, name, &connection)
	case "discover":
		return r.interactiveDiscoverConnection(ctx, name)
	case "delete":
		if err := ensureConnectionUnreferenced(kind, name, doc); err != nil {
			return err
		}
		confirmed, err := r.confirmInteractive(ctx, "Delete "+kind+"?", fmt.Sprintf("Delete %q permanently", name))
		if err != nil || !confirmed {
			return err
		}
		return r.service.DeleteSavedConnection(ctx, kind, name)
	default:
		return nil
	}
}

func (r *Renderer) connectionWizard(ctx context.Context, kind, name string, existing *model.Connection) error {
	creating := existing == nil
	connectorName := ""
	if existing != nil {
		connectorName = existing.Type
	}
	connectorNames := r.connectorNames(kind)
	if len(connectorNames) == 0 {
		return fmt.Errorf("no %s connectors are registered", kind)
	}
	connectorOptions := make([]interactiveOption, 0, len(connectorNames))
	for _, connector := range connectorNames {
		displayName := connector
		description := ""
		if kind == "source" && r.catalog.Sources[connector].DisplayName != "" {
			displayName = r.catalog.Sources[connector].DisplayName
		}
		if kind == "source" {
			description = r.catalog.Sources[connector].Description
		}
		if kind == "sink" && r.catalog.Sinks[connector].DisplayName != "" {
			displayName = r.catalog.Sinks[connector].DisplayName
		}
		if kind == "sink" {
			description = r.catalog.Sinks[connector].Description
		}
		connectorOptions = append(connectorOptions, interactiveOption{label: displayName, value: connector, description: description})
	}
	if connectorName == "" {
		connectorName = connectorNames[0]
	}
	fields := []inline.FormField{}
	if creating {
		fields = append(fields, inline.NewTextField("name", strings.ToUpper(kind[:1])+kind[1:]+" name").
			SetValue(name).
			Required().
			Validate(func(value string) error { return validateName(kind, value) }))
	}
	fields = append(fields, connectorSelectField(connectorName, connectorOptions))
	result, err := r.runInteractiveForm(ctx, inline.NewForm("Connection").Add(fields...))
	if err != nil {
		return err
	}
	if creating {
		name, _ = result["name"].(string)
	}
	connectorName, _ = result["connector"].(string)

	doc, err := r.service.Configuration(ctx)
	if err != nil {
		return err
	}
	if creating {
		if _, exists := connectionMap(kind, doc)[name]; exists {
			return fmt.Errorf("%s %q already exists", kind, name)
		}
	}
	var initial map[string]any
	if existing != nil && existing.Type == connectorName {
		initial = existing.Config
	}
	schema, err := r.connectionSchema(kind, connectorName)
	if err != nil {
		return err
	}
	wizard := newSchemaWizard(schema, filament.ScopeConnection, initial)
	if err := wizard.run(ctx, r, "Connection settings"); err != nil {
		return err
	}
	_, err = r.service.SaveConnection(ctx, cliapp.SaveConnectionRequest{
		Create: creating, Kind: kind, Name: name, Connector: connectorName, Config: wizard.patch(),
	})
	return err
}

func connectorSelectField(preferred string, options []interactiveOption) *inline.SelectField {
	return inline.NewSelectField("connector", "Connector", preferredChoices(preferred, options)...).
		Filterable(true).
		Required()
}

func (r *Renderer) interactiveDiscoverConnection(ctx context.Context, name string) error {
	resources, err := r.service.DiscoverSource(ctx, cliapp.DiscoverSourceRequest{
		Source: name, Refresh: true,
	})
	if err != nil {
		return err
	}
	if len(resources.Items) == 0 {
		return r.showInteractiveMessage(ctx, "Resources", "No resources discovered")
	}
	lines := make([]string, 0, len(resources.Items))
	for _, resource := range resources.Items {
		label := resource.Name
		if resource.DisplayName != "" && resource.DisplayName != resource.Name {
			label += "  ·  " + resource.DisplayName
		}
		if resource.EstimatedRows > 0 {
			label += fmt.Sprintf("  ·  %d estimated rows", resource.EstimatedRows)
		}
		lines = append(lines, label)
	}
	return r.showInteractiveMessage(ctx, "Discovered resources", strings.Join(lines, "\n"))
}

func newSchemaWizard(schema filament.ConfigSchema, scope filament.FieldScope, initial map[string]any) *schemaWizard {
	wizard := &schemaWizard{}
	for _, field := range cliapp.OrderedFields(schema, scope) {
		value := schemaWizardField{schema: field}
		raw, exists := initial[field.Name]
		if !exists {
			raw = field.Default
		}
		value.set(raw)
		wizard.fields = append(wizard.fields, value)
	}
	return wizard
}

func (w *schemaWizard) run(ctx context.Context, renderer *Renderer, title string) error {
	completed := map[*schemaWizardField]bool{}
	for {
		steps := w.steps()
		currentIndex := -1
		for index := range steps {
			if !completed[steps[index].field] {
				currentIndex = index
				break
			}
		}
		if currentIndex < 0 {
			return nil
		}
		current := steps[currentIndex]
		stepper, err := schemaStepper(title, steps, currentIndex, completed)
		if err != nil {
			return err
		}
		field, apply := current.field.dadoField()
		form := inline.NewForm(current.path).
			SetHeader(stepper).
			SetHeaderGap(1).
			Add(field)
		result, err := renderer.runInteractiveForm(ctx, form)
		if err != nil {
			return err
		}
		apply(result[current.field.schema.Name])
		completed[current.field] = true
	}
}

func (w *schemaWizard) steps() []schemaWizardStep {
	steps := []schemaWizardStep{}
	collectSchemaWizardSteps(&steps, w.fields, w.values(), "", nil)
	return steps
}

func collectSchemaWizardSteps(steps *[]schemaWizardStep, fields []schemaWizardField, values map[string]any, idPrefix string, path []string) {
	for index := range fields {
		field := &fields[index]
		if !cliapp.FieldVisible(field.schema, values) {
			continue
		}
		id := fmt.Sprintf("%s%d", idPrefix, index)
		label := schemaFieldLabel(field.schema.Name)
		fieldPath := append(append([]string(nil), path...), label)
		if field.structuredObject() {
			collectSchemaWizardSteps(steps, field.fields, field.fieldValues(), id+"-", fieldPath)
			continue
		}
		*steps = append(*steps, schemaWizardStep{
			id:    id,
			label: label,
			path:  strings.Join(fieldPath, " › "),
			field: field,
		})
	}
}

func schemaStepper(title string, steps []schemaWizardStep, currentIndex int, completed map[*schemaWizardField]bool) (*inline.Stepper, error) {
	stepper := inline.NewStepper(
		fmt.Sprintf("%s · %d of %d", title, currentIndex+1, len(steps)),
		inline.WithStepOrientation(inline.StepHorizontal),
		inline.WithMaxVisibleSteps(4),
		inline.WithStepDetails(false),
	)
	for _, step := range steps {
		if err := stepper.Add(step.id, step.label); err != nil {
			return nil, err
		}
	}
	for index := range steps {
		step := steps[index]
		switch {
		case completed[step.field]:
			if err := stepper.Complete(step.id); err != nil {
				return nil, err
			}
		case index == currentIndex:
			if err := stepper.Activate(step.id); err != nil {
				return nil, err
			}
		}
	}
	return stepper, nil
}

func (w *schemaWizard) values() map[string]any {
	values := make(map[string]any, len(w.fields))
	for index := range w.fields {
		value := &w.fields[index]
		values[value.schema.Name] = value.value()
	}
	return values
}

func (w *schemaWizard) patch() cliapp.ConfigPatch {
	values := w.values()
	patch := cliapp.ConfigPatch{Values: map[string]any{}}
	for index := range w.fields {
		value := &w.fields[index]
		if !cliapp.FieldVisible(value.schema, values) {
			continue
		}
		if !value.schema.Required && value.schema.Default == nil && schemaWizardValueEmpty(value.value()) {
			patch.Unset = append(patch.Unset, value.schema.Name)
			continue
		}
		patch.Values[value.schema.Name] = value.value()
	}
	return patch
}

func (v *schemaWizardField) set(raw any) {
	if v.structuredObject() {
		initial, _ := raw.(map[string]any)
		v.fields = make([]schemaWizardField, 0, len(v.schema.Fields))
		for _, field := range v.schema.Fields {
			child := schemaWizardField{schema: field}
			childRaw, exists := initial[field.Name]
			if !exists {
				childRaw = field.Default
			}
			child.set(childRaw)
			v.fields = append(v.fields, child)
		}
		return
	}
	switch value := raw.(type) {
	case bool:
		v.boolean = value
	case []string:
		v.list = append([]string(nil), value...)
	case []any:
		for _, item := range value {
			v.list = append(v.list, fmt.Sprint(item))
		}
	case map[string]any:
		encoded, _ := json.Marshal(value)
		v.text = string(encoded)
	case nil:
	default:
		v.text = fmt.Sprint(value)
	}
}

func (v *schemaWizardField) dadoField() (inline.FormField, func(any)) {
	name := v.schema.Name
	label := schemaFieldLabel(name)
	switch v.schema.Type {
	case filament.FieldBool:
		preferred := strconv.FormatBool(v.boolean)
		choices := preferredChoices(preferred, []interactiveOption{{label: "False", value: "false"}, {label: "True", value: "true"}})
		return inline.NewSelectField(name, label, choices...).Required(), func(raw any) {
			v.boolean = raw == "true"
		}
	case filament.FieldEnum:
		options := make([]interactiveOption, 0, len(v.schema.Enum)+1)
		if !v.schema.Required {
			options = append(options, interactiveOption{label: "Unset", value: ""})
		}
		for _, option := range v.schema.Enum {
			optionLabel := option.Label
			if optionLabel == "" {
				optionLabel = option.Value
			}
			options = append(options, interactiveOption{label: optionLabel, value: option.Value})
		}
		field := inline.NewSelectField(name, label, preferredChoices(v.text, options)...)
		if v.schema.Required {
			field.Required()
		}
		return field, func(raw any) { v.text, _ = raw.(string) }
	case filament.FieldList:
		if len(v.schema.Enum) > 0 {
			choices := make([]inline.Choice, 0, len(v.schema.Enum))
			for _, option := range v.schema.Enum {
				optionLabel := option.Label
				if optionLabel == "" {
					optionLabel = option.Value
				}
				choices = append(choices, inline.NewChoice(option.Value, optionLabel))
			}
			field := inline.NewMultiSelectField(name, label, choices...).
				SetSelectedValues(v.list...).
				SetIndicator(inline.IndicatorMinimal)
			if v.schema.Required {
				field.MinSelected(1)
			}
			return field, func(raw any) {
				v.list, _ = raw.([]string)
			}
		}
		fallthrough
	default:
		value := v.text
		placeholder := v.schema.Help
		if v.schema.Type == filament.FieldList {
			value = strings.Join(v.list, ",")
			if placeholder == "" {
				placeholder = "comma-separated values"
			}
		}
		field := inline.NewTextField(name, label).SetValue(value).SetPlaceholder(placeholder).Validate(func(raw string) error {
			if raw == "" && v.schema.Required {
				return fmt.Errorf("%s is required", name)
			}
			if raw == "" {
				return nil
			}
			_, err := cliapp.ParseFieldValue(v.schema, raw)
			return err
		})
		if v.schema.Required {
			field.Required()
		}
		if cliapp.IsSecretField(v.schema) {
			field.Password()
		}
		return field, func(raw any) {
			text, _ := raw.(string)
			if v.schema.Type == filament.FieldList {
				v.list = splitComma(text)
				return
			}
			if cliapp.IsSecretField(v.schema) {
				if name, referenced := cliapp.EnvironmentReferenceName(text); referenced {
					text = "env:" + name
				}
			}
			v.text = text
		}
	}
}

func (v *schemaWizardField) value() any {
	if v.structuredObject() {
		values := v.fieldValues()
		result := map[string]any{}
		for index := range v.fields {
			field := &v.fields[index]
			if !cliapp.FieldVisible(field.schema, values) {
				continue
			}
			value := field.value()
			if !field.schema.Required && field.schema.Default == nil && schemaWizardValueEmpty(value) {
				continue
			}
			result[field.schema.Name] = value
		}
		return result
	}
	switch v.schema.Type {
	case filament.FieldBool:
		return v.boolean
	case filament.FieldList:
		return v.list
	default:
		parsed, err := cliapp.ParseFieldValue(v.schema, v.text)
		if err != nil {
			return v.text
		}
		return parsed
	}
}

func (v *schemaWizardField) structuredObject() bool {
	return v.schema.Type == filament.FieldObject && len(v.schema.Fields) > 0
}

func (v *schemaWizardField) fieldValues() map[string]any {
	values := make(map[string]any, len(v.fields))
	for index := range v.fields {
		field := &v.fields[index]
		values[field.schema.Name] = field.value()
	}
	return values
}

func schemaFieldLabel(name string) string {
	return strings.ReplaceAll(strings.ToUpper(name[:1])+name[1:], "_", " ")
}

func schemaWizardValueEmpty(value any) bool {
	switch typed := value.(type) {
	case nil:
		return true
	case string:
		return typed == ""
	case []string:
		return len(typed) == 0
	case map[string]any:
		return len(typed) == 0
	default:
		return false
	}
}

func connectionDescription(connection model.Connection, schema filament.ConfigSchema) string {
	lines := make([]string, 0, len(connection.Config)+1)
	lines = append(lines, "Connector: "+connection.Type)
	fields := make(map[string]filament.ConfigField)
	for _, field := range cliapp.OrderedFields(schema, filament.ScopeConnection) {
		fields[field.Name] = field
	}
	for _, name := range sortedKeys(connection.Config) {
		value := connection.Config[name]
		if field, ok := fields[name]; ok {
			value = redactSchemaValue(field, value)
		}
		lines = append(lines, fmt.Sprintf("%s: %v", name, value))
	}
	return strings.Join(lines, "\n")
}

func redactSchemaValue(field filament.ConfigField, value any) any {
	if cliapp.IsSecretField(field) {
		text, _ := value.(string)
		if strings.HasPrefix(text, "env:") {
			return text
		}
		return "••••••••"
	}
	if field.Type != filament.FieldObject || len(field.Fields) == 0 {
		return value
	}
	object, ok := value.(map[string]any)
	if !ok {
		return value
	}
	redacted := cloneMap(object)
	for _, child := range field.Fields {
		if !cliapp.FieldVisible(child, object) {
			continue
		}
		childValue, exists := object[child.Name]
		if !exists {
			continue
		}
		redacted[child.Name] = redactSchemaValue(child, childValue)
	}
	return redacted
}
