# Filament TypeScript SDK

Create connections, discover resources, build pipelines, and run them from TypeScript or JavaScript.

- **Package:** `@galaxy-io/filament-ts`
- **Import:** `import { FilamentClient } from "@galaxy-io/filament-ts"`
- **Node:** 18+, or any runtime with `fetch`
- **Module format:** ES modules with bundled type declarations

## Installation

~~~sh
npm install @galaxy-io/filament-ts
~~~

## Connect to Filament

~~~ts
import { FilamentClient } from "@galaxy-io/filament-ts";

const filament = new FilamentClient({
  environment: process.env.FILAMENT_URL ?? "http://localhost:8080",
  token: process.env.FILAMENT_TOKEN,
});

const { connectors } = await filament.connector.list({});
for (const connector of connectors ?? []) {
  console.log(connector.name);
}
~~~

Set `FILAMENT_TOKEN` to a bearer token accepted by your server's identity provider.
For a local server with authentication disabled, omit `token`. The SDK also accepts
an async function as `token` for credentials that need refreshing.

Request timeouts are set with `timeoutInSeconds`, and retries with `maxRetries`,
either on the client or per call. The client retries twice by default; pass
`maxRetries: 0` on calls that must not repeat.

## Create and run a pipeline

A **connector** is an available integration, such as `sample` or `stdout`.
A **connection** is a configured instance you create using that connector.

The workflow is:

1. Create source and sink connections.
2. Discover the source's resources and select which ones to include.
3. Create a pipeline and save its graph.
4. Submit a run.

This complete example routes five rows from each selected sample resource to
stdout. It requires a running Filament deployment with a working execution backend.

~~~ts
import { randomUUID } from "node:crypto";
import { FilamentClient } from "@galaxy-io/filament-ts";

const filament = new FilamentClient({
  environment: process.env.FILAMENT_URL ?? "http://localhost:8080",
});
const name = `sample-to-stdout-${randomUUID().slice(0, 8)}`;

const { connection: source } = await filament.connection.create({
  name: `${name}-source`,
  kind: "CONNECTOR_KIND_SOURCE",
  connector: "sample",
});

const { connection: sink } = await filament.connection.create({
  name: `${name}-sink`,
  kind: "CONNECTOR_KIND_SINK",
  connector: "stdout",
});

const { resources } = await filament.connector.discoverResources({
  connectionId: source!.id,
});

// Select every available resource, or set this to ["users"].
const selectedResources = (resources ?? [])
  .filter((resource) => resource.isSelectable)
  .map((resource) => resource.name!);

const { pipeline } = await filament.pipeline.create({ name });

await filament.pipeline.version.create({
  pipelineId: pipeline!.id,
  graph: {
    nodes: [
      {
        id: "source",
        kind: "CONNECTOR_KIND_SOURCE",
        connectionId: source!.id,
        config: { rows: 5 },
      },
      {
        id: "sink",
        kind: "CONNECTOR_KIND_SINK",
        connectionId: sink!.id,
      },
    ],
    edges: selectedResources.map((resource) => ({
      fromNode: "source",
      toNode: "sink",
      resource,
    })),
  },
});

const submitted = await filament.pipeline.run({ pipelineId: pipeline!.id });

console.log("Pipeline:", pipeline!.id);
for (const edgeRun of submitted.edgeRuns ?? []) {
  console.log("Run:", edgeRun.run?.id, edgeRun.run?.status);
}
~~~

`pipeline.create(...)` creates the pipeline's metadata.
`pipeline.version.create(...)` saves its graph; the backend assigns the version
automatically. Both steps are required before the first run.

The sample source discovers `users` and `orders`. To route all resources without
enumerating them, save one edge with `resource` omitted. You still supply the source
and sink nodes.

Submitting a run returns its ID and initial status. It does not wait for completion.
Inspect a run with `filament.run.get({ runId })`, or open the pipeline in the UI.
The stdout sink writes rows to the worker's logs.

## API groups

| Group | Examples |
| --- | --- |
| `connector` | `list`, `get`, `discoverResources`, `validateConfig` |
| `connection` | `create`, `get`, `list`, `update`, `delete` |
| `pipeline` | `create`, `get`, `list`, `update`, `validate`, `run` |
| `pipeline.version` | `create`, `get`, `list` |
| `pipeline.schedule` | `create`, `update` |
| `pipeline.notifier` | `create`, `list`, `update`, `delete` |
| `run` | `get`, `list`, `signal` |
| `metrics` | `queryTimeseries`, `queryAggregate` |
| `auth` | `getConfig`, `getSession`, `login`, `logout` |
| `member` | `list`, `invite`, `setRole`, `remove` |
| `serviceAccount` | `create`, `list`, `rotateSecret`, `remove` |

The SDK covers unary API methods. Streaming `TailRun` is not included.

## Responses

Methods return typed responses. For example, `pipeline.create(...)` resolves to an
object whose `pipeline` property holds the created pipeline.

Fields omitted by the server are `undefined`, including lists. Use
`response.pipelines ?? []` when iterating. Protobuf 64-bit integer fields, such as
timestamps and record counts, can be decimal strings; use `Number(value)` or
`BigInt(value)` when needed.

Errors are thrown as `FilamentError`, with `statusCode` and the server's message.
Timeouts throw `FilamentTimeoutError`.

## Regenerating

The client under `src/` is generated by Fern from the server's OpenAPI specification.
From the repository root:

~~~sh
just sdks
~~~
