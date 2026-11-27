// Package sqlserver registers the Microsoft SQL Server and Azure SQL sources
// with the default registry. Both run the same reader and differ only in the
// connection settings they accept. Enable them with a blank import:
//
//	import _ "github.com/galaxy-io/filament/connectors/sqlserver"
package sqlserver

import (
	"github.com/galaxy-io/filament"
	source "github.com/galaxy-io/filament/connectors/sqlserver/source"
	"github.com/galaxy-io/filament/registry"
)

func init() {
	registry.RegisterSource("sqlserver", true, func() filament.Source { return source.New() })
	registry.RegisterSource("azuresql", true, func() filament.Source { return source.NewAzureSQL() })
}
