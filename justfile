# generate all checked-in generated code
gen: proto sqlc

# generate protobuf and ConnectRPC Go stubs
proto:
    buf generate

# generate sqlc Go code
sqlc:
    sqlc generate -f datastore/postgres/sqlc.yaml

# lint protobuf definitions
proto-lint:
    buf lint

# verify generated protobuf files are up to date
proto-check:
    buf generate
    git diff --exit-code -- api

# build both linux binaries into bin/
binaries:
    GOWORK=off CGO_ENABLED=0 GOOS=linux go build -C cmd/ingestion-control -trimpath -ldflags="-s -w" -o ../../bin/filament-control .
    GOWORK=off CGO_ENABLED=0 GOOS=linux go build -C cmd/ingestion-worker -trimpath -ldflags="-s -w" -o ../../bin/filament-worker .

# build docker images
images: binaries
    docker build -f cmd/ingestion-control/Dockerfile -t galaxy-io/filament:latest .
    docker build -f cmd/ingestion-worker/Dockerfile -t galaxy-io/filament-worker:latest .

# run a command in every Go module (tests/ needs docker; excluded where noted)
_each cmd:
    for dir in $(find . -name go.mod -exec dirname {} \;); do (cd "$dir" && {{cmd}}); done

# apply gofumpt + goimports to every Go module (settings in .golangci.yaml)
format: (_each "GOWORK=off golangci-lint fmt ./...")

# check formatting without writing
format-check: (_each "GOWORK=off golangci-lint fmt --diff ./...")

# run linters and apply auto-fixes where possible
lint: (_each "GOWORK=off golangci-lint run --fix ./...")

# run linters without fixing (what CI runs)
lint-check: (_each "GOWORK=off golangci-lint run ./...")

# run unit tests in every Go module except tests/ (integration; needs docker)
test:
    for dir in $(find . -name go.mod -not -path "./tests/*" -exec dirname {} \;); do (cd "$dir" && GOWORK=off go test ./...); done

# run the integration/e2e suite (requires docker)
test-integration:
    cd tests && GOWORK=off go test ./...
