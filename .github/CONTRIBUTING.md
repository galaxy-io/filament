# Contributing to Filament

Thanks for helping improve Filament. This guide explains how to set up the project, make a change, and open a pull request.

## Before you start

For a bug fix, open or link an issue that explains what happened and what you expected. For a large feature or architectural change, open a feature request before you start. This gives us time to agree on the approach.

By participating, you agree to follow our [Code of Conduct](../CODE_OF_CONDUCT.md).

Do not include credentials, customer data, production connection strings, or other secrets in issues, tests, fixtures, logs, or commits.

## Development setup

You will need:

- Go 1.26.4 or newer
- Docker with Compose
- `just`
- Node.js 22 and pnpm 10 for UI changes
- `golangci-lint`, Buf, and sqlc for formatting, linting, and code generation

On macOS, the repository's [`Brewfile`](../Brewfile) installs the toolchain:

```sh
brew bundle
```

On other platforms, install the same tools with your package manager. Then list the available tasks:

```sh
just --list
```

Start the local dependencies and application with:

```sh
just infra
just dev
```

Open <http://localhost:5173> to use the UI. When you finish, run `just infra down`. Run `just infra volumes` if you also want to delete local PostgreSQL and NATS data.

## Making a change

1. Fork the repository and clone your fork.
2. Create a focused branch from the latest `main`.
3. Add or update tests for behavior changes.
4. Regenerate checked-in artifacts when their sources change.
5. Run the relevant formatting, lint, build, and test commands.
6. Open a pull request to the upstream repository. Explain the problem, your approach, and how you tested the change.

Keep each pull request focused on one concern. Clearly note changes to public interfaces, connector configuration, protobuf definitions, datastore migrations, or Helm values.

## Code generation

We commit generated API and datastore code. Do not edit it by hand.

| Changed source | Command | Generated output |
| --- | --- | --- |
| `api/**/*.proto` | `just proto` | Go and TypeScript API clients |
| `datastore/postgres/queries/*.sql` or `sqlc.yaml` | `just sqlc` | PostgreSQL query code |
| Either of the above | `just gen` | All generated Go/API code |

Use `just proto-check` to confirm generated protobuf files are current.

## Tests and checks

Run the checks that apply to your change. Start with:

```sh
just format check
just lint check
just test
```

Useful targeted commands include:

```sh
just ui-dist          # Type-check and build the UI bundle
just build            # Generate and compile the full workspace
just proto-lint       # Lint protobuf definitions
just test-integration # Run Docker-backed integration and e2e tests
```

Integration tests use testcontainers and require Docker. You can pin container image versions in `docker/.env` as described in [`tests/testcontainers/README.md`](../tests/testcontainers/README.md). Some TPC-H tests also require `duckdb` on `PATH`.

For a small change, you may run the narrowest relevant package or test. List the exact commands in your pull request.

## Connector changes

Connector changes should include:

- A connector specification with accurate maturity and capability metadata
- Configuration validation and, where supported, a live connection test
- Unit tests for configuration, type conversion, and errors
- Integration tests for behavior that depends on an external system
- Registration in the correct `register.go` file
- User documentation for required permissions and configuration

New sources implement [`source.go`](../source.go). New sinks implement [`sink.go`](../sink.go). Advertise optional interfaces only when they are implemented and tested.

## Pull requests

A pull request should include:

- A short description of the problem and why the change is needed
- The approach and any important tradeoffs
- Links to related issues
- Screenshots or recordings for UI changes
- Migration or compatibility notes for API, configuration, or schema changes
- The commands used to test the change

Before requesting review, confirm that:

- Generated files are current
- Formatting, linting, and relevant tests pass
- Tests cover new behavior
- Documentation and examples are current
- The change contains no secrets or unrelated generated files

Rules in [`.github/CODEOWNERS`](CODEOWNERS) request the right reviewers automatically.

## Reporting bugs and proposing features

Use GitHub Issues for bugs and feature proposals. Search first to avoid duplicates. Include the use case, affected component, environment, and constraints. Remove all sensitive information.

Do not open a public issue for a security report. Use the repository owner's private security contact or GitHub's private vulnerability reporting feature when available.
