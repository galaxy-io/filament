package hubspot

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/rowmodel"
)

type object struct {
	ID         string          `json:"id"`
	CreatedAt  time.Time       `json:"createdAt"`
	UpdatedAt  time.Time       `json:"updatedAt"`
	ArchivedAt *time.Time      `json:"archivedAt"`
	Archived   bool            `json:"archived"`
	Properties json.RawMessage `json:"properties"`
}

type objectPage struct {
	Results []object `json:"results"`
	Paging  struct {
		Next struct {
			After string `json:"after"`
		} `json:"next"`
	} `json:"paging"`
}

// Schema describes a stable envelope; new custom properties do not require DDL.
func (s *Source) Schema(_ context.Context, name string) (rowmodel.Schema, error) {
	r, err := lookupResource(name)
	if err != nil {
		return rowmodel.Schema{}, err
	}
	return schemaFor(r), nil
}

func schemaFor(r resource) rowmodel.Schema {
	schema := rowmodel.Schema{Resource: r.name, PrimaryKey: []string{"id"}, Fields: []rowmodel.Field{
		{Name: "id", Logical: rowmodel.LogicalString, Native: "text"},
		{Name: "created_at", Logical: rowmodel.LogicalTimestampTZ, Native: "timestamptz", Nullable: true},
		{Name: "updated_at", Logical: rowmodel.LogicalTimestampTZ, Native: "timestamptz", Nullable: true},
		{Name: "archived", Logical: rowmodel.LogicalBool, Native: "bool"},
		{Name: "archived_at", Logical: rowmodel.LogicalTimestampTZ, Native: "timestamptz", Nullable: true},
		{Name: "properties", Logical: rowmodel.LogicalJSON, Native: "json", Nullable: true},
	}}
	if r.modified != "" {
		schema.Fields = append(schema.Fields, rowmodel.Field{Name: "modified_at", Logical: rowmodel.LogicalTimestampTZ, Native: "timestamptz"})
	}
	return schema
}

func appendObject(w arrowbatch.RowWriter, r resource, row object, key []string) error {
	if row.ID == "" {
		return fmt.Errorf("record has no id")
	}
	var modified time.Time
	if r.modified != "" {
		var err error
		modified, err = modificationTime(row, r.modified)
		if err != nil {
			return err
		}
	}
	w.String(row.ID)
	appendTime(w, row.CreatedAt)
	appendTime(w, row.UpdatedAt)
	w.Bool(row.Archived)
	if row.ArchivedAt == nil {
		w.Null()
	} else {
		appendTime(w, *row.ArchivedAt)
	}
	if len(row.Properties) == 0 || string(row.Properties) == "null" {
		w.Null()
	} else {
		w.StringBytes(row.Properties)
	}
	if r.modified != "" {
		w.Timestamp(modified.UnixMicro())
	}
	return w.EndRow(rowmodel.Meta{Key: key})
}

func appendTime(w arrowbatch.RowWriter, value time.Time) {
	if value.IsZero() {
		w.Null()
	} else {
		w.Timestamp(value.UnixMicro())
	}
}

func modificationTime(row object, name string) (time.Time, error) {
	var properties map[string]json.RawMessage
	if err := json.Unmarshal(row.Properties, &properties); err != nil {
		return time.Time{}, fmt.Errorf("record %s properties: %w", row.ID, err)
	}
	var value string
	if err := json.Unmarshal(properties[name], &value); err != nil || value == "" {
		return time.Time{}, fmt.Errorf("record %s has no valid %s", row.ID, name)
	}
	if instant, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return instant, nil
	}
	if millis, err := strconv.ParseInt(value, 10, 64); err == nil {
		return time.UnixMilli(millis).UTC(), nil
	}
	return time.Time{}, fmt.Errorf("record %s has an invalid %s timestamp", row.ID, name)
}
