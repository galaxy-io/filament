package appstoreconnect

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/rowmodel"
)

type jsonAPIObject struct {
	ID         string          `json:"id"`
	Type       string          `json:"type"`
	Attributes json.RawMessage `json:"attributes"`
}

type jsonAPIPage struct {
	Data  []jsonAPIObject `json:"data"`
	Links struct {
		Next string `json:"next"`
	} `json:"links"`
}

type recordRow struct {
	id         string
	appID      string
	category   string
	name       string
	cursorTime time.Time
	attributes json.RawMessage
}

// Schema returns the stable envelope schema for each resource.
func (s *Source) Schema(_ context.Context, name string) (rowmodel.Schema, error) {
	r, err := lookupResource(name)
	if err != nil {
		return rowmodel.Schema{}, err
	}
	return schemaFor(r), nil
}

func schemaFor(r resource) rowmodel.Schema {
	switch r.name {
	case "apps":
		return rowmodel.Schema{
			Resource:   r.name,
			PrimaryKey: []string{"id"},
			Fields: []rowmodel.Field{
				{Name: "id", Logical: rowmodel.LogicalString, Native: "text"},
				{Name: "bundle_id", Logical: rowmodel.LogicalString, Native: "text", Nullable: true},
				{Name: "name", Logical: rowmodel.LogicalString, Native: "text", Nullable: true},
				{Name: "sku", Logical: rowmodel.LogicalString, Native: "text", Nullable: true},
				{Name: "primary_locale", Logical: rowmodel.LogicalString, Native: "text", Nullable: true},
				{Name: "attributes", Logical: rowmodel.LogicalJSON, Native: "json", Nullable: true},
			},
		}
	case "customer_reviews":
		return rowmodel.Schema{
			Resource:   r.name,
			PrimaryKey: []string{"id"},
			Fields: []rowmodel.Field{
				{Name: "id", Logical: rowmodel.LogicalString, Native: "text"},
				{Name: "app_id", Logical: rowmodel.LogicalString, Native: "text"},
				{Name: "rating", Logical: rowmodel.LogicalInt64, Native: "bigint", Nullable: true},
				{Name: "title", Logical: rowmodel.LogicalString, Native: "text", Nullable: true},
				{Name: "body", Logical: rowmodel.LogicalString, Native: "text", Nullable: true},
				{Name: "reviewer_nickname", Logical: rowmodel.LogicalString, Native: "text", Nullable: true},
				{Name: "territory", Logical: rowmodel.LogicalString, Native: "text", Nullable: true},
				{Name: "attributes", Logical: rowmodel.LogicalJSON, Native: "json", Nullable: true},
				{Name: "created_at", Logical: rowmodel.LogicalTimestampTZ, Native: "timestamptz"},
			},
		}
	case "analytics_reports":
		return rowmodel.Schema{
			Resource:   r.name,
			PrimaryKey: []string{"id"},
			Fields: []rowmodel.Field{
				{Name: "id", Logical: rowmodel.LogicalString, Native: "text"},
				{Name: "app_id", Logical: rowmodel.LogicalString, Native: "text"},
				{Name: "category", Logical: rowmodel.LogicalString, Native: "text", Nullable: true},
				{Name: "report_name", Logical: rowmodel.LogicalString, Native: "text", Nullable: true},
				{Name: "metrics", Logical: rowmodel.LogicalJSON, Native: "json", Nullable: true},
				{Name: "processing_date", Logical: rowmodel.LogicalTimestampTZ, Native: "timestamptz"},
			},
		}
	default: // sales_reports
		return rowmodel.Schema{
			Resource:   r.name,
			PrimaryKey: []string{"id"},
			Fields: []rowmodel.Field{
				{Name: "id", Logical: rowmodel.LogicalString, Native: "text"},
				{Name: "sku", Logical: rowmodel.LogicalString, Native: "text", Nullable: true},
				{Name: "country_code", Logical: rowmodel.LogicalString, Native: "text", Nullable: true},
				{Name: "product_type_identifier", Logical: rowmodel.LogicalString, Native: "text", Nullable: true},
				{Name: "metrics", Logical: rowmodel.LogicalJSON, Native: "json", Nullable: true},
				{Name: "report_date", Logical: rowmodel.LogicalTimestampTZ, Native: "timestamptz"},
			},
		}
	}
}

func appendRecord(w arrowbatch.RowWriter, r resource, row recordRow, key []string) error {
	if row.id == "" {
		return fmt.Errorf("record has no id")
	}
	switch r.name {
	case "apps":
		var attrs struct {
			BundleID      string `json:"bundleId"`
			Name          string `json:"name"`
			SKU           string `json:"sku"`
			PrimaryLocale string `json:"primaryLocale"`
		}
		_ = json.Unmarshal(row.attributes, &attrs)
		w.String(row.id)
		appendOptionalString(w, attrs.BundleID)
		appendOptionalString(w, attrs.Name)
		appendOptionalString(w, attrs.SKU)
		appendOptionalString(w, attrs.PrimaryLocale)
		appendJSON(w, row.attributes)
	case "customer_reviews":
		var attrs struct {
			Rating           *int64 `json:"rating"`
			Title            string `json:"title"`
			Body             string `json:"body"`
			ReviewerNickname string `json:"reviewerNickname"`
			Territory        string `json:"territory"`
		}
		_ = json.Unmarshal(row.attributes, &attrs)
		w.String(row.id)
		w.String(row.appID)
		if attrs.Rating == nil {
			w.Null()
		} else {
			w.Int64(*attrs.Rating)
		}
		appendOptionalString(w, attrs.Title)
		appendOptionalString(w, attrs.Body)
		appendOptionalString(w, attrs.ReviewerNickname)
		appendOptionalString(w, attrs.Territory)
		appendJSON(w, row.attributes)
		w.Timestamp(row.cursorTime.UnixMicro())
	case "analytics_reports":
		w.String(row.id)
		w.String(row.appID)
		appendOptionalString(w, row.category)
		appendOptionalString(w, row.name)
		appendJSON(w, row.attributes)
		w.Timestamp(row.cursorTime.UnixMicro())
	default: // sales_reports
		w.String(row.id)
		appendOptionalString(w, row.appID)    // sku
		appendOptionalString(w, row.category) // country_code
		appendOptionalString(w, row.name)     // product_type_identifier
		appendJSON(w, row.attributes)
		w.Timestamp(row.cursorTime.UnixMicro())
	}
	return w.EndRow(rowmodel.Meta{Key: key})
}

func appendOptionalString(w arrowbatch.RowWriter, val string) {
	if val == "" {
		w.Null()
	} else {
		w.String(val)
	}
}

func appendJSON(w arrowbatch.RowWriter, raw json.RawMessage) {
	if len(raw) == 0 || string(raw) == "null" {
		w.Null()
	} else {
		w.StringBytes(raw)
	}
}
