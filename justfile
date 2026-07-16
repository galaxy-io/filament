set dotenv-load

default:
    @just --list

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

# build the UI bundle the server embeds
ui-dist:
    cd ui && pnpm install && pnpm build

# build linux binaries into bin/ (server embeds ui/dist)
binaries: ui-dist
    GOWORK=off CGO_ENABLED=0 GOOS=linux go build -C cmd/server -tags embedui -trimpath -ldflags="-s -w" -o ../../bin/filament-server .
    GOWORK=off CGO_ENABLED=0 GOOS=linux go build -C cmd/control-plane -trimpath -ldflags="-s -w" -o ../../bin/filament-control-plane .
    GOWORK=off CGO_ENABLED=0 GOOS=linux go build -C cmd/worker -trimpath -ldflags="-s -w" -o ../../bin/filament-worker .

# build docker images
images: binaries
    docker build -f cmd/server/Dockerfile -t galaxy-io/filament-server:latest .
    docker build -f cmd/control-plane/Dockerfile -t galaxy-io/filament-control-plane:latest .
    docker build -f cmd/worker/Dockerfile -t galaxy-io/filament-worker:latest .

# run a command in every Go module (tests/ needs docker; excluded where noted)
_each cmd:
    for dir in $(find . -name go.mod -exec dirname {} \;); do (cd "$dir" && {{cmd}}) || exit 1; done

# tidy go.mod/go.sum in every Go module
tidy: (_each "GOWORK=off go mod tidy")

# apply gofumpt + goimports to every Go module (settings in .golangci.yaml), plus UI formatting
format: (_each "GOWORK=off golangci-lint fmt ./...") ui-format

# check Go formatting without writing (what CI runs)
go-format-check: (_each "GOWORK=off golangci-lint fmt --diff ./...")

# check formatting without writing
format-check: go-format-check ui-format-check

# run linters and apply auto-fixes where possible
lint: (_each "GOWORK=off golangci-lint run --fix ./...") ui-lint

# run Go linters without fixing (what CI runs)
go-lint-check: (_each "GOWORK=off golangci-lint run ./...")

# run all linters without fixing
lint-check: go-lint-check ui-lint-check

# run unit tests in every Go module except tests/ (integration; needs docker)
test:
    for dir in $(find . -name go.mod -not -path "./tests/*" -exec dirname {} \;); do (cd "$dir" && GOWORK=off go test ./...) || exit 1; done

# run the integration/e2e suite (requires docker)
test-integration:
    cd tests && GOWORK=off go test ./...

# start local infra (postgres + nats), gated on health
infra:
    docker compose up -d --wait

# run datastore migrations against the local database
migrate:
    cd cmd/server && \
      PERSISTENCE_DSN="${PERSISTENCE_DSN:-postgresql://filament:filament@localhost:5432/filament?sslmode=disable}" \
      GOWORK=off go run . -migrate

# run the API server locally (defaults match docker-compose.yaml; env overrides)
server: migrate
    cd cmd/server && \
      PERSISTENCE_DSN="${PERSISTENCE_DSN:-postgresql://filament:filament@localhost:5432/filament?sslmode=disable}" \
      NATS_URL="${NATS_URL:-nats://localhost:4222}" \
      NATS_STREAM="${NATS_STREAM:-EVENTBUS}" \
      NATS_SUBJECTS="${NATS_SUBJECTS:-ingestion.v1.>}" \
      ENCRYPTION_KEY="${ENCRYPTION_KEY:-2y4Ou1wAxZ3tReU064W61mal5sXl/2ymtS022pbizws=}" \
      GOWORK=off go run .

# run the control plane locally (defaults match docker-compose.yaml; env overrides)
control-plane:
    cd cmd/control-plane && \
      PERSISTENCE_DSN="${PERSISTENCE_DSN:-postgresql://filament:filament@localhost:5432/filament?sslmode=disable}" \
      NATS_URL="${NATS_URL:-nats://localhost:4222}" \
      NATS_STREAM="${NATS_STREAM:-EVENTBUS}" \
      NATS_SUBJECTS="${NATS_SUBJECTS:-ingestion.v1.>}" \
      DISPATCH_MODE="${DISPATCH_MODE:-inproc}" \
      ENCRYPTION_KEY="${ENCRYPTION_KEY:-2y4Ou1wAxZ3tReU064W61mal5sXl/2ymtS022pbizws=}" \
      GOWORK=off go run .

# run the web UI dev server (vite, proxies API to :8080)
ui:
    cd ui && pnpm install && pnpm dev

# apply Biome formatting to the UI package
ui-format:
    cd ui && pnpm run format

# check UI formatting without writing
ui-format-check:
    cd ui && pnpm run format:check

# run Biome lint and apply auto-fixes where possible
ui-lint:
    cd ui && pnpm run lint

# run Biome lint without fixing (what CI runs)
ui-lint-check:
    cd ui && pnpm run lint:check

# run the full app: control plane, API server, UI
dev:
    #!/usr/bin/env bash
    set -euo pipefail
    trap 'kill $(jobs -p) 2>/dev/null' EXIT
    just control-plane &
    just server &
    just ui &
    wait
