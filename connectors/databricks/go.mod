module github.com/galaxy-io/filament/connectors/databricks

go 1.26.4

require (
	github.com/apache/arrow-go/v18 v18.7.0
	github.com/databricks/zerobus-sdk/go v1.7.0
	github.com/galaxy-io/filament v0.0.0
	go.opentelemetry.io/otel v1.45.0
	go.opentelemetry.io/otel/metric v1.45.0
	golang.org/x/oauth2 v0.36.0
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/go-logr/logr v1.4.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/goccy/go-json v0.10.6 // indirect
	github.com/google/flatbuffers v25.12.19+incompatible // indirect
	github.com/klauspost/compress v1.20.0 // indirect
	github.com/klauspost/cpuid/v2 v2.4.0 // indirect
	github.com/pierrec/lz4/v4 v4.1.30 // indirect
	github.com/zeebo/xxh3 v1.1.0 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/otel/trace v1.45.0 // indirect
	golang.org/x/exp v0.0.0-20260718201538-764159d718ef // indirect
	golang.org/x/sys v0.48.0 // indirect
	google.golang.org/protobuf v1.36.12-0.20260120151049-f2248ac996af // indirect
)

replace github.com/galaxy-io/filament => ../..
