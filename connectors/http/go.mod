module github.com/galaxy-io/filament/connectors/http

go 1.26.4

require (
	github.com/galaxy-io/filament v0.0.0
	github.com/santhosh-tekuri/jsonschema/v5 v5.3.1
	github.com/tidwall/gjson v1.18.0
	golang.org/x/sync v0.20.0
	golang.org/x/time v0.15.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/kr/pretty v0.3.1 // indirect
	github.com/rogpeppe/go-internal v1.14.1 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.0 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
)

replace github.com/galaxy-io/filament => ../..
