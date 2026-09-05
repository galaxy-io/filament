<div align="center">

<picture>
  <source media="(prefers-color-scheme: dark)" srcset=".github/assets/filament-dark.svg">
  <source media="(prefers-color-scheme: light)" srcset=".github/assets/filament-light.svg">
  <img alt="Filament — pluggable data replication with checkpointing, batching, and integrity events" src=".github/assets/filament-light.svg" width="520">
</picture>

[![Release](https://img.shields.io/github/v/release/galaxy-io/filament?filter=v*)](https://github.com/galaxy-io/filament/releases)
[![Tests](https://github.com/galaxy-io/filament/actions/workflows/test.yaml/badge.svg)](https://github.com/galaxy-io/filament/actions/workflows/test.yaml)
[![License: Apache 2.0](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)
[![Slack](https://img.shields.io/badge/slack-join-4A154B?logo=slack&logoColor=white)](https://join.slack.com/t/galaxy-filament/shared_invite/zt-486oaagls-WCKm605mP6NCmoB3E7frpQ)

</div>

# Filament

Filament is pluggable data replication with checkpointing, batching, and integrity events.

It moves data from sources to sinks using full, incremental, or change data capture replication. Filament keeps progress durable as data moves, verifies batches on both sides of a write, and safely pauses or resumes runs from their checkpoints.

Sources, sinks, state storage, and event transport are replaceable interfaces. Run Filament as a service with its API and web UI, deploy it to your own infrastructure, or embed the engine in a Go application.

## Install

```sh
curl -fsSL https://getgalaxy.io/filament/install | sh
```

On macOS via Homebrew

```sh
brew install --cask galaxy-io/tap/filament
```

Prebuilt binaries for Linux and macOS are on the [releases page](https://github.com/galaxy-io/filament/releases).

## Getting started

One-click deployments for Railway and DigitalOcean are coming soon. Until then, you can run Filament locally or follow the [documentation](https://filament.getgalaxy.io) for more detailed guidance.

The local environment requires Go, Docker, `just`, Node.js, and pnpm. On macOS, the repository's `Brewfile` installs the toolchain:

```sh
brew bundle
```

Start PostgreSQL and NATS, then run Filament:

```sh
just infra
just dev
```

Open <http://localhost:5173> to use the web UI.

When you are finished, stop the local infrastructure with:

```sh
just infra down
```

## How it works

A Filament pipeline connects a source to a sink and defines how selected resources should be replicated. During a run, Filament extracts records into bounded batches, writes and verifies each batch, and advances durable checkpoints only after successful work.

The runtime is event-driven: the server accepts pipeline and run requests, the control plane schedules and dispatches work, and workers execute individual runs. PostgreSQL stores durable state and NATS JetStream carries lifecycle events between components.

## Developing Filament

Filament is written in Go, with a React and TypeScript web UI. Contributions to the engine, connectors, UI, deployment tooling, tests, and documentation are welcome.

See [CONTRIBUTING.md](.github/CONTRIBUTING.md) for repository setup, code generation, tests, and pull request guidance.

## Community

- Read the [Filament documentation](https://filament.getgalaxy.io).
- Ask questions and meet other users in the [Filament Slack community](https://join.slack.com/t/galaxy-filament/shared_invite/zt-486oaagls-WCKm605mP6NCmoB3E7frpQ).
- Report bugs or propose features through [GitHub Issues](https://github.com/galaxy-io/filament/issues).
