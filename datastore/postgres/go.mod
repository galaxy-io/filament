module github.com/galaxy-io/filament/datastore/postgres

go 1.26.4

require (
	github.com/galaxy-io/filament v0.0.0
	github.com/jackc/pgx/v5 v5.10.0
	github.com/pressly/goose/v3 v3.24.1
	google.golang.org/protobuf v1.36.12-0.20260120151049-f2248ac996af
)

require (
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/mfridman/interpolate v0.0.2 // indirect
	github.com/sethvargo/go-retry v0.3.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/text v0.37.0 // indirect
)

replace github.com/galaxy-io/filament => ../..
