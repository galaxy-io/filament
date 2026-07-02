module github.com/galaxy-io/filament/datastore/postgres

go 1.26.4

require (
	github.com/galaxy-io/filament v0.0.0
	github.com/jackc/pgx/v5 v5.10.0
	github.com/pressly/goose/v3 v3.24.1
	google.golang.org/protobuf v1.36.11
)

replace github.com/galaxy-io/filament => ../..
