package catalog

import (
	"context"
	"fmt"

	icecatalog "github.com/apache/iceberg-go/catalog"

	// Blank imports register each catalog backend's factory in the iceberg-go
	// catalog registry. Load dispatches on the configured type or URI scheme.
	_ "github.com/apache/iceberg-go/catalog/glue"
	_ "github.com/apache/iceberg-go/catalog/hive"
	_ "github.com/apache/iceberg-go/catalog/rest"
	_ "github.com/apache/iceberg-go/catalog/sql"

	// Registers the FileIO backends used for table data and metadata files.
	_ "github.com/apache/iceberg-go/io/gocloud"
)

// Open loads the configured Iceberg catalog.
func Open(ctx context.Context, name string, setup Setup) (icecatalog.Catalog, error) {
	return icecatalog.Load(ctx, name, setup.Properties)
}

// Test loads the catalog and performs a read-only namespace listing.
func Test(ctx context.Context, setup Setup) error {
	cat, err := Open(ctx, "iceberg-validation", setup)
	if err != nil {
		return fmt.Errorf("open catalog: %w", err)
	}
	if _, err := cat.ListNamespaces(ctx, nil); err != nil {
		return fmt.Errorf("list namespaces: %w", err)
	}
	return nil
}
