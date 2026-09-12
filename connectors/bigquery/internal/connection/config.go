package connection

import (
	"fmt"
	"strings"

	"github.com/galaxy-io/filament"
)

// Resolved contains the validated options needed to create a BigQuery client.
// Authentication is intentionally absent: the official client discovers
// Application Default Credentials from the runtime environment.
type Resolved struct {
	ProjectID string
	Location  string
}

// Fields returns the connection-scoped configuration for BigQuery.
func Fields() []filament.ConfigField {
	return []filament.ConfigField{
		{Name: "project_id", Type: filament.FieldString, Required: true, Scope: filament.ScopeConnection, Help: "Google Cloud project used for BigQuery jobs. Filament authenticates with Application Default Credentials (ADC)."},
		{Name: "location", Type: filament.FieldString, Scope: filament.ScopeConnection, Help: "Optional BigQuery dataset and job location, such as us-central1."},
	}
}

// Resolve translates connector configuration into BigQuery client options.
func Resolve(cfg filament.Config) (Resolved, error) {
	projectID := strings.TrimSpace(cfg.String("project_id"))
	if projectID == "" {
		return Resolved{}, fmt.Errorf("project_id is required")
	}
	return resolved(projectID, cfg.String("location"))
}

func resolved(projectID, location string) (Resolved, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return Resolved{}, fmt.Errorf("project_id is required")
	}
	location = strings.TrimSpace(location)
	if location != "" && (strings.ContainsAny(location, "/\\") || strings.ContainsAny(location, " \t\r\n")) {
		return Resolved{}, fmt.Errorf("location must be a location ID")
	}
	return Resolved{ProjectID: projectID, Location: location}, nil
}
