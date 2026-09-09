<div align="center">

<picture>
  <source media="(prefers-color-scheme: dark)" srcset=".github/assets/filament-dark.svg">
  <source media="(prefers-color-scheme: light)" srcset=".github/assets/filament-light.svg">
  <img alt="Filament — pluggable data replication with checkpointing, batching, and integrity events" src=".github/assets/filament-light.svg" width="520">
</picture>

[![Release](https://img.shields.io/github/v/release/galaxy-io/filament?filter=v*)](https://github.com/galaxy-io/filament/releases)
[![Tests](https://github.com/galaxy-io/filament/actions/workflows/test.yaml/badge.svg)](https://github.com/galaxy-io/filament/actions/workflows/test.yaml)
[![License: Apache 2.0](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)
[![DeepWiki](https://img.shields.io/badge/DeepWiki-galaxy--io%2Ffilament-blue.svg?logo=data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAACwAAAAyCAYAAAAnWDnqAAAAAXNSR0IArs4c6QAAA05JREFUaEPtmUtyEzEQhtWTQyQLHNak2AB7ZnyXZMEjXMGeK/AIi+QuHrMnbChYY7MIh8g01fJoopFb0uhhEqqcbWTp06/uv1saEDv4O3n3dV60RfP947Mm9/SQc0ICFQgzfc4CYZoTPAswgSJCCUJUnAAoRHOAUOcATwbmVLWdGoH//PB8mnKqScAhsD0kYP3j/Yt5LPQe2KvcXmGvRHcDnpxfL2zOYJ1mFwrryWTz0advv1Ut4CJgf5uhDuDj5eUcAUoahrdY/56ebRWeraTjMt/00Sh3UDtjgHtQNHwcRGOC98BJEAEymycmYcWwOprTgcB6VZ5JK5TAJ+fXGLBm3FDAmn6oPPjR4rKCAoJCal2eAiQp2x0vxTPB3ALO2CRkwmDy5WohzBDwSEFKRwPbknEggCPB/imwrycgxX2NzoMCHhPkDwqYMr9tRcP5qNrMZHkVnOjRMWwLCcr8ohBVb1OMjxLwGCvjTikrsBOiA6fNyCrm8V1rP93iVPpwaE+gO0SsWmPiXB+jikdf6SizrT5qKasx5j8ABbHpFTx+vFXp9EnYQmLx02h1QTTrl6eDqxLnGjporxl3NL3agEvXdT0WmEost648sQOYAeJS9Q7bfUVoMGnjo4AZdUMQku50McDcMWcBPvr0SzbTAFDfvJqwLzgxwATnCgnp4wDl6Aa+Ax283gghmj+vj7feE2KBBRMW3FzOpLOADl0Isb5587h/U4gGvkt5v60Z1VLG8BhYjbzRwyQZemwAd6cCR5/XFWLYZRIMpX39AR0tjaGGiGzLVyhse5C9RKC6ai42ppWPKiBagOvaYk8lO7DajerabOZP46Lby5wKjw1HCRx7p9sVMOWGzb/vA1hwiWc6jm3MvQDTogQkiqIhJV0nBQBTU+3okKCFDy9WwferkHjtxib7t3xIUQtHxnIwtx4mpg26/HfwVNVDb4oI9RHmx5WGelRVlrtiw43zboCLaxv46AZeB3IlTkwouebTr1y2NjSpHz68WNFjHvupy3q8TFn3Hos2IAk4Ju5dCo8B3wP7VPr/FGaKiG+T+v+TQqIrOqMTL1VdWV1DdmcbO8KXBz6esmYWYKPwDL5b5FA1a0hwapHiom0r/cKaoqr+27/XcrS5UwSMbQAAAABJRU5ErkJggg==)](https://deepwiki.com/galaxy-io/filament)
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
brew install galaxy-io/tap/filament
```

Prebuilt binaries for Linux and macOS are on the [releases page](https://github.com/galaxy-io/filament/releases).

## Getting started

To run Filament locally, install Go, Docker, `just`, Node.js, and pnpm. On macOS, the repository's `Brewfile` installs the toolchain:

```sh
brew bundle
```

For Podman users, set `CONTAINER_ENGINE=podman` to use Podman with the commands below. See [engine configuration](docs/pages/guides/contributing/local-development.mdx).

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

## Deploying Filament

Kubernetes with Helm is the recommended way to run Filament in production. See the [Kubernetes deployment guide](https://filament.getgalaxy.io/pages/guides/deployment/kubernetes) for installation instructions and the [Helm chart README](charts/filament/README.md) for all supported values.

For a simpler hosted deployment without Kubernetes, deploy with 1 click:

<div>

[![Deploy on Railway](https://railway.com/button.svg)](https://railway.com/deploy/filament?referralCode=H5DcHR&utm_medium=integration&utm_source=template&utm_campaign=generic)
[![Deploy to Render](https://render.com/images/deploy-to-render-button.svg)](https://render.com/deploy?repo=https%3A%2F%2Fgithub.com%2Fgalaxy-io%2Ffilament&branch=main&path=deploy%2Frender.yaml)
[![Deploy to DO](https://www.deploytodo.com/do-btn-blue.svg)](https://cloud.digitalocean.com/apps/new?repo=https://github.com/galaxy-io/filament/tree/feat/digital-ocean-deploy)

</div>

Read the [Railway](https://filament.getgalaxy.io/pages/guides/deployment/railway), [Render](https://filament.getgalaxy.io/pages/guides/deployment/render), and [DigitalOcean](https://filament.getgalaxy.io/pages/guides/deployment/digitalocean) guides for the deployed resources, runtime behavior, and production checklist.

> [!WARNING]
> The DigitalOcean deployment uses [filament/standalone](https://github.com/galaxy-io/filament/blob/main/cmd/standalone/main.go), an all-in-one image that includes the control plane, worker, and SQLite datastore.


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
