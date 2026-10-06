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
- Protos drive the UI's types. After a proto change run `cd ui && pnpm codegen` and commit `ui/src/gen/`. A new service needs a proxy entry in `ui/vite.config.ts`.
- Gates, run from `ui/` with pnpm (never npm): `pnpm typecheck`, `pnpm lint:check`, `pnpm format:check`, `pnpm build`, and `git diff --exit-code src/routeTree.gen.ts`. `just format check` and `just lint check` cover the UI with the rest of the repo.
- The four design-system references inside the skill (`components.md`, `tokens.md`, `patterns.md`, `recipes.md`) are generated in the DLS repository. After a DLS release copy them by hand from `~/git/dls/skills/galaxy-ui/references/`. Never copy the whole DLS skill folder over `.agents/skills/filament-ui`.
- Do not restart a dev server you did not start. `just dev` runs the control plane, the API on 8080 and the UI on 5173. Start your own server on a free port to verify.

## Repository

- `just` is the task runner. `just --list` shows every recipe. `just gen` regenerates all checked-in generated code, `just test` runs Go unit tests, `just test-integration` and `just test-e2e` use Testcontainers.
- Go modules form a workspace (`go.work`). Build a single module with `GOWORK=off go build ./...` inside it.
- Datastore migrations live in `datastore/`. `just migrate` applies them to the local database. Postgres tests expect their own scratch database, never the development one.
- Docs are a Mintlify site under `docs/` (`just docs`). Prose in docs avoids semicolons and colons and keeps sentences tight.
- Do not commit, push or open pull requests unless asked. Leave edits in the working tree and report what changed.
