set dotenv-load

default:
    @just --list

# start local infra (postgres + nats); `just infra auth` adds zitadel,
# `just infra down` stops everything, `just infra down volumes` also drops volumes
infra mode="up" scope="":
    docker compose {{ if mode == "down" { if scope == "volumes" { "--profile auth down -v" } else { "--profile auth down" } } else if mode == "auth" { "--profile auth up -d --wait" } else { "up -d --wait" } }}

# run datastore migrations against the local database
migrate:
    cd cmd/server && \
      PERSISTENCE_DSN="${PERSISTENCE_DSN:-postgresql://filament:filament@localhost:5432/filament?sslmode=disable}" \
      GOWORK=off go run . -migrate

# run the API server locally (defaults match docker-compose.yaml; env overrides)
server mode="": migrate
    cd cmd/server && \
      PERSISTENCE_DSN="${PERSISTENCE_DSN:-postgresql://filament:filament@localhost:5432/filament?sslmode=disable}" \
      NATS_URL="${NATS_URL:-nats://localhost:4222}" \
      NATS_STREAM="${NATS_STREAM:-EVENTBUS}" \
      NATS_SUBJECTS="${NATS_SUBJECTS:-ingestion.v1.>}" \
      ENCRYPTION_KEY="${ENCRYPTION_KEY:-2y4Ou1wAxZ3tReU064W61mal5sXl/2ymtS022pbizws=}" \
      AUTH_PROVIDER="${AUTH_PROVIDER:-{{ if mode == "auth" { "zitadel" } else { "" } }}}" \
      AUTH_ISSUER="${AUTH_ISSUER:-http://localhost:8300}" \
      AUTH_PAT="${AUTH_PAT:-$(cat {{ justfile_directory() }}/.zitadel/pat 2>/dev/null)}" \
      AUTH_UI_ORIGIN="${AUTH_UI_ORIGIN:-http://localhost:5173}" \
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

# run the full app: control plane, API server, UI
dev mode="": migrate
    #!/usr/bin/env bash
    set -euo pipefail
    trap 'kill $(jobs -p) 2>/dev/null' EXIT
    just control-plane &
    just server {{ mode }} &
    until curl -sf http://localhost:8080/startupz > /dev/null 2>&1; do sleep 0.2; done
    until curl -sf http://localhost:8081/startupz > /dev/null 2>&1; do sleep 0.2; done
    just ui &
    wait

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

# format protobuf definitions
proto-format mode="fix":
    buf format {{ if mode == "check" { "-d --exit-code" } else { "-w" } }}

# verify generated protobuf files are up to date
proto-check:
    buf generate
    git diff --exit-code -- api

# build the UI bundle the server embeds
ui-dist:
    cd ui && pnpm install && pnpm build

# regenerate code, build the UI, and compile every Go module
build: gen ui-dist (_each "GOWORK=off go build ./...")

# build linux release binaries into bin/ (server and standalone embed ui/dist)
binaries: ui-dist
    GOWORK=off CGO_ENABLED=0 GOOS=linux go build -C cmd/server -tags embedui -trimpath -ldflags="-s -w" -o ../../bin/filament/server .
    GOWORK=off CGO_ENABLED=0 GOOS=linux go build -C cmd/control-plane -trimpath -ldflags="-s -w" -o ../../bin/filament/control-plane .
    GOWORK=off CGO_ENABLED=0 GOOS=linux go build -C cmd/worker -trimpath -ldflags="-s -w" -o ../../bin/filament/worker .
    GOWORK=off CGO_ENABLED=0 GOOS=linux go build -C cmd/standalone -tags embedui -trimpath -ldflags="-s -w" -o ../../bin/filament/standalone .
    GOWORK=off CGO_ENABLED=0 GOOS=linux go build -C cmd/filament -trimpath -ldflags="-s -w" -o ../../bin/filament/filament .

# build docker images
images: binaries
    docker build -f cmd/server/Dockerfile -t galaxy-io/filament/server:latest .
    docker build -f cmd/control-plane/Dockerfile -t galaxy-io/filament/control-plane:latest .
    docker build -f cmd/worker/Dockerfile -t galaxy-io/filament/worker:latest .
    docker build -f cmd/standalone/Dockerfile -t galaxy-io/filament/standalone:latest .

# run a command in every Go module (tests/ needs docker; excluded where noted)
_each cmd:
    for dir in $(find . -name go.mod -exec dirname {} \;); do (cd "$dir" && {{cmd}}) || exit 1; done

# format Go, UI, and proto; `just format check` verifies without writing
format mode="fix": (go-format mode) (ui-format mode) (proto-format mode)

# run all linters; `just lint check` runs without fixing
lint mode="fix": (go-lint mode) (ui-lint mode)

# gofumpt + goimports across every Go module (settings in .golangci.yaml)
go-format mode="fix": (_each ("GOWORK=off golangci-lint fmt " + (if mode == "check" { "--diff " } else { "" }) + "./..."))

# Go linters across every Go module
go-lint mode="fix": (_each ("GOWORK=off golangci-lint run " + (if mode == "check" { "" } else { "--fix " }) + "./..."))

# Biome formatting for the UI package
ui-format mode="fix":
    cd ui && pnpm run {{ if mode == "check" { "format:check" } else { "format" } }}

# Biome lint for the UI package
ui-lint mode="fix":
    cd ui && pnpm run {{ if mode == "check" { "lint:check" } else { "lint" } }}

# run unit tests in every Go module except tests/ (integration; needs docker)
test:
    for dir in $(find . -name go.mod -not -path "./tests/*" -exec dirname {} \;); do (cd "$dir" && GOWORK=off go test ./...) || exit 1; done

# run the integration/e2e suite (requires docker + tests/docker/.env)
# Every suite file is //go:build integration, so without the tag this matches
# no packages and exits 0 — passing while testing nothing.
test-integration:
    cd tests && GOWORK=off go test -tags integration ./...

# tidy go.mod/go.sum in every Go module, then sync workspace versions
tidy: (_each "GOWORK=off go mod tidy")
    go work sync

# run the docs site locally (mintlify preview at :3000; installs deps on first run)
docs:
    cd docs && pnpm install && pnpm dev
