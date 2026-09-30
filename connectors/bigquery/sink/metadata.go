package bigquery

import (
	"context"
	"fmt"
	"strings"
	"time"

	"cloud.google.com/go/bigquery"

	"github.com/galaxy-io/filament/rowmodel"
)

func tableSchema(model rowmodel.Schema, stage bool) bigquery.Schema {
	schema := make(bigquery.Schema, 0, len(model.Fields)+2)
	for _, field := range model.Fields {
		typ := strings.Split(columnType(field), "(")[0]
		switch typ {
		case "INT64":
			typ = "INTEGER"
		case "FLOAT64":
			typ = "FLOAT"
		case "BOOL":
			typ = "BOOLEAN"
		}
		if stage && (field.Logical == rowmodel.LogicalTime || field.Logical == rowmodel.LogicalDecimal) {
			typ = "STRING"
		}
		// Staging must accept key-only delete records even if destination fields
		// are required. Final publication enforces destination constraints.
		column := &bigquery.FieldSchema{Name: field.Name, Type: bigquery.FieldType(typ), Required: !stage && !field.Nullable}
		if typ == "NUMERIC" || typ == "BIGNUMERIC" {
			column.Precision = int64(field.Precision)
			column.Scale = int64(field.Scale)
		}
		schema = append(schema, column)
	}
	return schema
}

func (s *Sink) ensureDestination(ctx context.Context, state *tableState) error {
	table := s.client.DatasetInProject(s.project, s.dataset).Table(state.definition.name)
	wanted := tableSchema(state.model, false)
	metadata, err := table.Metadata(ctx)
	if isHTTPCode(err, 404) {
		desired := &bigquery.TableMetadata{Schema: wanted}
		// Key constraints are NOT ENFORCED. Omitting them avoids telling the query
		// optimizer keys are unique when append mode permits repeated keys.
		err = table.Create(ctx, desired)
		if err == nil {
			return nil
		}
		if !isHTTPCode(err, 409) {
			return err
		}
		metadata, err = table.Metadata(ctx)
	}
	if err != nil {
		return err
	}
	for attempt := 0; attempt < 3; attempt++ {
		existing := make(map[string]*bigquery.FieldSchema, len(metadata.Schema))
		updated := append(bigquery.Schema(nil), metadata.Schema...)
		for _, field := range metadata.Schema {
			existing[field.Name] = field
		}
		for _, field := range wanted {
			old, ok := existing[field.Name]
			if !ok {
				added := *field
				added.Required = false
				updated = append(updated, &added)
				continue
			}
			if old.Type != field.Type || old.Repeated || len(old.Schema) > 0 {
				return fmt.Errorf("field %q has type %s, expected %s", field.Name, old.Type, field.Type)
			}
			if (field.Type == bigquery.NumericFieldType || field.Type == bigquery.BigNumericFieldType) && old.Precision != 0 && (old.Precision != field.Precision || old.Scale != field.Scale) {
				return fmt.Errorf("field %q decimal precision/scale differs from destination", field.Name)
			}
		}
		if len(updated) == len(metadata.Schema) {
			return nil
		}
		_, err = table.Update(ctx, bigquery.TableMetadataToUpdate{Schema: updated}, metadata.ETag)
		if err == nil {
			return nil
		}
		if !isHTTPCode(err, 412) {
			return err
		}
		metadata, err = table.Metadata(ctx)
		if err != nil {
			return err
		}
	}
	return fmt.Errorf("destination schema kept changing during update")
}

func (s *Sink) createStage(ctx context.Context, state *tableState) error {
	schema := tableSchema(state.model, true)
	schema = append(schema, &bigquery.FieldSchema{Name: state.definition.operation.name, Type: bigquery.IntegerFieldType}, &bigquery.FieldSchema{Name: state.definition.ordinal.name, Type: bigquery.IntegerFieldType})
	err := s.client.DatasetInProject(s.project, s.dataset).Table(state.stage).Create(ctx, &bigquery.TableMetadata{Schema: schema, ExpirationTime: time.Now().Add(stageLifetime)})
	if err != nil {
		return err
	}
	state.stageCreated = true
	return nil
}
