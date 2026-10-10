# Filament: agent guide

Filament is pluggable data replication with checkpointing, batching and integrity events. It is a Go engine and server (`cmd/`, `server/`, `pipeline/`, `connectors/`, `datastore/`, the root package) with a protobuf API (`api/`, `protos/`) and a React web UI (`ui/`). The code is the source of truth. This file says where the house conventions live so you do not re-derive them.

## Skills

The repository ships its conventions as skills under `.agents/skills/` (`.claude` is a symlink to `.agents`, so Claude Code loads them automatically).

| Skill | Use it when |
|---|---|
| `filament-ui` | Anything under `ui/`. Screens, routes, search params, query hooks, forms, tables, dashboards, overlays, styling with `@galaxy-io/dls`, reviewing UI. |
| `filament-http-connector` | Building or changing an HTTP source connector, its manifest, catalog registration and docs. |

## UI

All work in `ui/` follows the `filament-ui` skill. Read its `SKILL.md` before writing or reviewing a component, and keep its app references (`architecture.md`, `routing.md`, `data.md`, `screens.md`, `forms.md`, `dashboards.md`, `workflow.md`) current when a convention changes. Do not restate UI conventions here. The short version, so a change in another part of the repo does not break them.

- The UI is React 18, Vite, TanStack Router, ConnectRPC through `@connectrpc/connect-query`, Linaria, and the Galaxy design system `@galaxy-io/dls`. Components come from the DLS by subpath, styling comes from DLS tokens, and nothing is imported through a barrel.
- One source tree builds the standalone app (`pnpm build` → `ui/build/`, embedded by the Go server) and the mountable `@galaxy-io/filament` module (`pnpm build:lib` → `ui/dist/`) that a Galaxy host mounts. Standalone-only code lives in `ui/src/host/`, the module's surface lives in `ui/src/module/`, and the package exports are the `exports` map in `ui/package.json`. Library code never imports `src/host` or names an absolute route. `ui/fixtures/host/` is the reference host. See "Module and host" in `ui/README.md`.
- Protos drive the UI's types. After a proto change run `cd ui && pnpm codegen` and commit `ui/src/gen/`. A new service needs a proxy entry in `ui/vite.config.ts`.
- Gates, run from `ui/` with pnpm (never npm): `pnpm check` (typecheck, typecheck:lib, lint, format, both builds, the package checks and the fixture host), then `git diff --exit-code src/host/routeTree.gen.ts`. `just ui-check` runs all of it from the root and is what CI runs. `just format check` and `just lint check` cover the UI with the rest of the repo.
- The four design-system references inside the skill (`components.md`, `tokens.md`, `patterns.md`, `recipes.md`) describe `@galaxy-io/dls`. Update them with a DLS version bump and keep app conventions out of them.
- Do not restart a dev server you did not start. `just dev` runs the control plane, the API on 8080 and the UI on 5173. Start your own Vite on a free port with its own `cacheDir` to verify, with `API_PROXY_TARGET` pointing the UI proxy at a second API when needed.

## Services

Four binaries under `cmd/` share one wiring layer in `cmd/internal/`. Each `main.go` opens with a package comment that states the process's job. Read it before changing a process.

- `cmd/server` serves the ConnectRPC API and the embedded UI, persists pipelines and runs, and publishes `run.requested` facts. It never executes a run. `-migrate` applies datastore migrations and exits.
- `cmd/control-plane` is the orchestration loop. It fires schedules, dispatches requested runs, reaps stale ones, and folds worker facts back into the datastore. Its modules live in `internal/modules/` (`tracker`, `dispatch`, `scheduler`, `reaper`, `notifier`, `streamsupervisor`). Run exactly one replica.
- `cmd/worker` executes one already persisted run and exits. It learns its assignment from `RUN_ID`, `TENANT_ID` and, for continuous runs, `EXECUTION_ID`. Everything else arrives through the worker Secret and ConfigMap. It gets no Kubernetes service-account token.
- `cmd/standalone` and the CLI's local mode run the whole stack in one process over SQLite, composed by `app/`. Standalone embeds NATS, the CLI uses the in-process bus. `cmd/server` does not use `app/`.

`DISPATCH_MODE` picks the dispatcher in `cmd/internal/dispatch`. `kubernetes` (the default) creates one batch Job per run through `internal/modules/dispatch/k8s`, configured by the `K8S_*` variables in its `config.go`. `inproc` runs the engine inside the control plane, which is what `just control-plane` and `just dev` use.

## Run lifecycle

- `RunStatus` and the `RunState` stamps are defined in `run.go`. Stamps are first-write-wins in the store. `RunScheduled` is declared last so persisted ordinals stay stable. The proto mirror is `protos/ingestion/v1/runs.proto`.
- The tracker is the only writer of run state. The runner (`runner/`) publishes `run.started` and exactly one terminal fact and never writes state itself. The reaper kills a run by publishing `run.failed`, not by updating a row. Keep it that way. A new transition is a new fact folded by the tracker.
- The scheduler owns `RunScheduled` rows and promotes them to Requested. The server and orchestrator persist Requested runs and publish `run.requested`, idempotent on the idempotency key.
- `runner.RunOne` is the single run sequence shared by the in-process engine and the worker. `pipeline/` is the builder, writer and integrity loop underneath it. Source and sink contracts are `source.go` and `sink.go`. Advertise an optional interface only when it is implemented and tested.
- Typed events live in `events/` with subjects of the form `ingestion.v1.<entity>.<tenant>.<run>.<event>`. The bus in `eventbus/` is transport only. Add a payload to the catalog in `events/`, never an untyped map.

## API and datastore

- Protos are `protos/{auth,ingestion,metrics}/v1`. `just proto` regenerates `api/` including `api/openapi.json`, and `just proto-check` verifies it. Fern SDKs under `sdks/` are generated from that spec with `just sdks`.
- Every RPC runs through the auth interceptor in `server/auth.go`. A new RPC is tenant-scoped and locked down until it is added to `publicProcedures`. Handlers pass the tenant to the store explicitly, and SQL filters on `tenant_id`.
- `filament.DataStore` in `infra.go` is the store contract. Optional capabilities such as schedules, metrics and continuous runs are separate interfaces found by type assertion. Postgres and SQLite each have `queries/*.sql` compiled by sqlc into `sqlcgen/`. Do not edit generated code.
- Migrations are goose files under `datastore/{postgres,sqlite}/migrations`. The Postgres schema is deployed, so never edit a shipped migration. Append a new numbered file.
- Secrets resolve through `secret/`. The datastore-backed provider is the default and needs `ENCRYPTION_KEY`. AWS and GCP providers are separate modules selected by `SECRET_PROVIDER`.
- Identity is optional. `AUTH_PROVIDER` unset means every request uses the default tenant. Zitadel and Keycloak adapters are separate modules under `identity/`.

## Runtime configuration

Configuration is explicit environment variables, one per value, read in `cmd/internal/`. Do not add a config file format or a packed string. `cmd/internal/boot` is the shared startup for every binary. The Helm chart in `charts/filament` repeats the env block in every ConfigMap on purpose, and its templates carry no explanatory comments. `docs/pages/guides/deployment/configuration.mdx` is the user-facing list and must change with the code.

- Local defaults match `docker-compose.yaml` and are set inside the `just server` and `just control-plane` recipes. `just infra` starts Postgres and NATS, and `just infra zitadel|keycloak` adds an identity provider.
- Health is `/livez`, `/startupz` and `/readyz` from `cmd/internal/health`. The server listens on `SERVER_ADDR` and the control plane's health server on `HEALTH_ADDR`.
- OTel export is off unless an `OTEL_EXPORTER_OTLP_*` endpoint is set. Each module registers its own instruments, and metric names start with `filament_`.
- Images are built from `cmd/*/Dockerfile` with `CGO_ENABLED=0` by `just binaries` and `just images`.

## Repository

- `just` is the task runner. `just --list` shows every recipe. `just gen` regenerates all checked-in generated code, `just test` runs Go unit tests, `just test-integration` and `just test-e2e` use Testcontainers.
- Go modules form a workspace (`go.work`). Build a single module with `GOWORK=off go build ./...` inside it.
- `just migrate` applies datastore migrations to the local database. Postgres tests expect their own scratch database, never the development one.
- Docs are a Mintlify site under `docs/` (`just docs`). Prose in docs avoids semicolons and colons and keeps sentences tight.
- Do not commit, push or open pull requests unless asked. Leave edits in the working tree and report what changed.
