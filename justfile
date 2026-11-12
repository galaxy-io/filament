set dotenv-load

# Local defaults, matching docker-compose.yaml
export PERSISTENCE_DSN := env('PERSISTENCE_DSN', 'postgresql://filament:filament@localhost:5432/filament?sslmode=disable')
export NATS_URL := env('NATS_URL', 'nats://localhost:4222')
export NATS_STREAM := env('NATS_STREAM', 'EVENTBUS')
export NATS_SUBJECTS := env('NATS_SUBJECTS', 'ingestion.v1.>')
# NOTE: This is a fake key for local development
export ENCRYPTION_KEY := env('ENCRYPTION_KEY','2y4Ou1wAxZ3tReU064W61mal5sXl/2ymtS022pbizws=')
export WORKER_URL := env('WORKER_URL', 'http://localhost:8082')

container := 'bash scripts/container.sh'

[private]
default:
    @just --list

# Start Postgres and NATS; `down [volumes]` stops
[group('dev')]
infra mode="up" scope="":
    #!/usr/bin/env bash
    set -euo pipefail
    case "{{ mode }}" in
      up) {{ container }} compose up -d --wait ;;
      down) {{ container }} compose --profile zitadel --profile keycloak down {{ if scope == "volumes" { "-v" } else { "" } }} ;;
      *) {{ container }} compose --profile {{ mode }} up -d --wait ;;
    esac
    # Keycloak's master realm refuses plain-http admin sign-ins from off the
    # container, which is where the server creates filament's realm from.
    if [ "{{ mode }}" = "keycloak" ]; then
      {{ container }} compose --profile keycloak exec -T keycloak sh -c \
        '/opt/keycloak/bin/kcadm.sh config credentials --server http://localhost:8080 --realm master --user admin --password admin >/dev/null && /opt/keycloak/bin/kcadm.sh update realms/master -s sslRequired=NONE'
    fi

# Migrate the local database
[group('dev')]
migrate:
    cd cmd/server && GOWORK=off go run . -migrate

# Run the API server
[group('dev')]
server mode="": migrate
    cd cmd/server && \
      AUTH_PROVIDER="${AUTH_PROVIDER:-{{ mode }}}" \
      AUTH_ISSUER="${AUTH_ISSUER:-{{ if mode == "keycloak" { "http://localhost:8400/realms/filament" } else { "http://localhost:8300" } }}}" \
      AUTH_PAT="${AUTH_PAT:-$(cat {{ justfile_directory() }}/.zitadel/pat 2>/dev/null)}" \
      AUTH_CLIENT_ID="${AUTH_CLIENT_ID:-filament}" \
      AUTH_CLIENT_SECRET="${AUTH_CLIENT_SECRET:-filament}" \
      AUTH_ADMIN_USERNAME="${AUTH_ADMIN_USERNAME:-admin}" \
      AUTH_ADMIN_PASSWORD="${AUTH_ADMIN_PASSWORD:-admin}" \
      AUTH_UI_ORIGIN="${AUTH_UI_ORIGIN:-http://localhost:5173}" \
      GOWORK=off go run .

# Run the worker
[group('dev')]
worker:
    cd cmd/worker && WORKER_ADDR="${WORKER_ADDR:-:8082}" GOWORK=off go run . -serve -execute

# Run the control plane
[group('dev')]
control-plane:
    cd cmd/control-plane && DISPATCH_MODE="${DISPATCH_MODE:-worker}" GOWORK=off go run .

# Run the UI dev server
[group('dev')]
ui:
    cd ui && pnpm install && pnpm dev

# Run the full stack
[group('dev')]
dev mode="": migrate
    #!/usr/bin/env bash
    set -euo pipefail
    trap 'kill $(jobs -p) 2>/dev/null' EXIT
    just worker &
    just control-plane &
    just server {{ mode }} &
    for port in 8082 8080 8081; do
      until curl -sf "http://localhost:$port/startupz" >/dev/null 2>&1; do sleep 0.2; done
    done
    just ui &
    wait

# Install the CLI and local engine
[group('dev')]
cli: ui-build
    #!/usr/bin/env bash
    set -euo pipefail
    bin="$(go env GOBIN)"
    bin="${bin:-$(go env GOPATH)/bin}"
    GOWORK=off CGO_ENABLED=0 go install -C cmd -trimpath -ldflags="-s -w" ./filament
    GOWORK=off CGO_ENABLED=0 go build -C cmd -tags embedui -trimpath -ldflags="-s -w" -o "$bin/filament-standalone" ./standalone

# Generate code
[group('code')]
gen: proto sqlc

# Generate protos and OpenAPI
[group('code')]
proto:
    buf generate

# Generate sqlc queries
[group('code')]
sqlc:
    sqlc generate -f datastore/postgres/sqlc.yaml
    sqlc generate -f datastore/sqlite/sqlc.yaml

# Generate SDKs
[group('code')]
sdks: proto
    fern check
    fern generate --group python
    fern generate --group typescript

# Tidy go.mod
[group('code')]
tidy: (_each "GOWORK=off go mod tidy")
    go work sync

# Build everything
[group('build')]
build: gen ui-build (_each "GOWORK=off go build ./...")

# Build the UI
[group('build')]
ui-build:
    cd ui && pnpm install && pnpm build

# Build release binaries
[group('build')]
binaries: ui-build (_release "server" "-tags embedui") (_release "control-plane") (_release "worker") (_release "standalone" "-tags embedui") (_release "filament")

# Build images
[group('build')]
images: binaries (_image "server") (_image "control-plane") (_image "worker") (_image "worker" "Dockerfile.libc" "latest-libc") (_image "standalone")

# Format everything
[group('check')]
format mode="fix": (go-format mode) (ui-format mode) (proto-format mode) (manifest-format mode)

# Lint everything
[group('check')]
lint mode="fix": (go-lint mode) (ui-lint mode) manifest-lint

# Format Go
[group('check')]
go-format mode="fix": (_each ("GOWORK=off golangci-lint fmt " + if mode == "check" { "--diff ./..." } else { "./..." }))

# Lint Go
[group('check')]
go-lint mode="fix": (_each ("GOWORK=off golangci-lint run " + if mode == "check" { "./..." } else { "--fix ./..." }))

# Format the UI
[group('check')]
ui-format mode="fix":
    cd ui && pnpm run {{ if mode == "check" { "format:check" } else { "format" } }}

# Lint the UI
[group('check')]
ui-lint mode="fix":
    cd ui && pnpm run {{ if mode == "check" { "lint:check" } else { "lint" } }}

# Run UI checks
[group('check')]
ui-check:
    cd ui && pnpm install --frozen-lockfile && pnpm check && git diff --exit-code src/host/routeTree.gen.ts

# Format protos
[group('check')]
proto-format mode="fix":
    buf format {{ if mode == "check" { "-d --exit-code" } else { "-w" } }}

# Lint protos
[group('check')]
proto-lint:
    buf lint

# Check generated protos
[group('check')]
proto-check:
    #!/usr/bin/env bash
    set -euo pipefail
    buf generate
    git diff --exit-code -- api
    untracked="$(git ls-files --others --exclude-standard -- api)"
    if [ -n "$untracked" ]; then echo "untracked generated files:"; echo "$untracked"; exit 1; fi

# Format HTTP manifests
[group('check')]
manifest-format mode="fix":
    go run github.com/google/yamlfmt/cmd/yamlfmt@v0.21.0 -conf connectors/http/.yamlfmt.yaml {{ if mode == "check" { "-lint" } else { "" } }} connectors/http/manifests

# Lint HTTP manifests
[group('check')]
manifest-lint: (manifest-format "check")

# Run Go tests
[group('test')]
test:
    for dir in $(find . -name go.mod -not -path "./tests/*" -exec dirname {} \;); do (cd "$dir" && GOWORK=off go test ./...) || exit 1; done

# Run integration tests
[group('test')]
test-integration:
    {{ container }} exec env GOWORK=off go -C tests test -count=1 -tags integration ./integration/...

# Run e2e tests
[group('test')]
test-e2e tags="e2e,privileged":
    {{ container }} exec env GOWORK=off go -C tests test -count=1 -tags {{ quote(tags) }} -p=1 ./e2e/...

# Check the container engine
[group('test')]
container-check:
    {{ container }} doctor

# Run the docs site
[group('docs')]
docs:
    cd docs && pnpm install && pnpm dev

# Build one release binary
[private]
_release name flags="":
    GOWORK=off CGO_ENABLED=0 GOOS=linux go build -C cmd/{{ name }} {{ flags }} -trimpath -ldflags="-s -w" -o ../../bin/filament/{{ name }} .

# Build one image
[private]
_image name dockerfile="Dockerfile" tag="latest":
    {{ container }} build --platform "linux/$(go env GOARCH)" -f cmd/{{ name }}/{{ dockerfile }} -t galaxy-io/filament/{{ name }}:{{ tag }} .

# Run a command in every Go module
[private]
_each cmd:
    for dir in $(find . -name go.mod -exec dirname {} \;); do (cd "$dir" && {{ cmd }}) || exit 1; done
