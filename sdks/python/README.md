# Filament Python SDK

Create connections, discover resources, build pipelines, and run them from Python.

- **Package:** `filament-py`
- **Import:** `from filament import Filament`
- **Python:** 3.10+
- **Clients:** synchronous `Filament` and asynchronous `AsyncFilament`

## Installation

Install [filament-py from PyPI](https://pypi.org/project/filament-py/) with uv:

~~~sh
uv add filament-py
~~~

For an existing virtual environment, use `uv pip install filament-py`.

## Connect to Filament

~~~python
import os

from filament import Filament

filament = Filament(
    base_url=os.getenv("FILAMENT_URL", "http://localhost:8080"),
)

for connector in filament.connector.list().connectors or []:
    print(connector.name)
~~~

Set `FILAMENT_TOKEN` to a bearer token accepted by your server's identity provider.
For a local server with authentication disabled, omit `token`. The SDK also accepts
a callable token provider for credentials that need refreshing.

You can configure request timeouts with `Filament(..., timeout=30)`; the value is
in seconds. The SDK creates its HTTP client and supplies protocol headers.

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

~~~python
import os
from uuid import uuid4

from filament import (
    Filament,
    IngestionV1PipelineEdge,
    IngestionV1PipelineGraph,
    IngestionV1PipelineNode,
)

filament = Filament(
    base_url=os.getenv("FILAMENT_URL", "http://localhost:8080"),
)
name = f"sample-to-stdout-{uuid4().hex[:8]}"

source = filament.connection.create(
    name=f"{name}-source",
    kind="CONNECTOR_KIND_SOURCE",
    connector="sample",
).connection

sink = filament.connection.create(
    name=f"{name}-sink",
    kind="CONNECTOR_KIND_SINK",
    connector="stdout",
).connection

resources = filament.connector.discover_resources(
    connection_id=source.id,
).resources or []

# Select every available resource, or set this to ["users"].
selected_resources = [
    resource.name for resource in resources if resource.is_selectable
]

pipeline = filament.pipeline.create(name=name).pipeline

filament.pipeline.version.create(
    pipeline_id=pipeline.id,
    graph=IngestionV1PipelineGraph(
        nodes=[
            IngestionV1PipelineNode(
                id="source",
                kind="CONNECTOR_KIND_SOURCE",
                connection_id=source.id,
                config={"rows": 5},
            ),
            IngestionV1PipelineNode(
                id="sink",
                kind="CONNECTOR_KIND_SINK",
                connection_id=sink.id,
            ),
        ],
        edges=[
            IngestionV1PipelineEdge(
                from_node="source",
                to_node="sink",
                resource=resource,
            )
            for resource in selected_resources
        ],
    ),
)

submitted = filament.pipeline.run(pipeline_id=pipeline.id)

print("Pipeline:", pipeline.id)
for edge_run in submitted.edge_runs or []:
    print("Run:", edge_run.run.id, edge_run.run.status)
~~~

`pipeline.create(...)` creates the pipeline's metadata.
`pipeline.version.create(...)` saves its graph; the backend assigns the version
automatically. Both steps are required before the first run.

The sample source discovers `users` and `orders`. To route all resources without
enumerating them, save one edge with `resource` omitted. You still supply the source
and sink nodes.

Submitting a run returns its ID and initial status. It does not wait for completion.
Inspect a run with `filament.run.get(run_id=run_id)`, or open the pipeline in the UI.
The stdout sink writes rows to the worker's logs. The example leaves its connections,
pipeline, and run available for inspection.

The runnable version lives in
[examples/smoke.py](https://github.com/galaxy-io/filament/blob/main/sdks/python/examples/smoke.py).
From the repository root:

~~~sh
uv run --project sdks/python sdks/python/examples/smoke.py
~~~

## Async usage

The async client exposes the same resource groups and methods.

~~~python
import asyncio
import os

from filament import AsyncFilament

async def main():
    filament = AsyncFilament(
        base_url=os.getenv("FILAMENT_URL", "http://localhost:8080"),
        token=os.getenv("FILAMENT_TOKEN"),
    )
    response = await filament.pipeline.list()
    for pipeline in response.pipelines or []:
        print(pipeline.id, pipeline.name)

asyncio.run(main())
~~~

## API groups

| Group | Examples |
| --- | --- |
| `connector` | `list`, `get`, `discover_resources`, `validate_config` |
| `connection` | `create`, `get`, `list`, `update`, `delete` |
| `pipeline` | `create`, `get`, `list`, `update`, `validate`, `run` |
| `pipeline.version` | `create`, `get`, `list` |
| `pipeline.schedule` | `create`, `update` |
| `pipeline.notifier` | `create`, `list`, `update`, `delete` |
| `run` | `get`, `list`, `signal` |
| `metrics` | `query_timeseries`, `query_aggregate` |
| `auth` | `get_config`, `get_session`, `login`, `logout` |
| `member` | `list`, `invite`, `set_role`, `remove` |
| `service_account` | `create`, `list`, `rotate_secret`, `remove` |

The SDK covers unary API methods. Streaming `TailRun` is not included.

## Responses and pagination

Methods return generated models. For example, `pipeline.create(...)` returns a
response whose `pipeline` property holds the created pipeline.

Fields omitted by the server can be `None`, including lists. Use
`response.pipelines or []` when iterating. Protobuf 64-bit integer fields, such as
timestamps and record counts, can be decimal strings; use `int(value)` when needed.

List methods expose explicit pagination:

~~~python
from filament import IngestionV1PaginationRequest

page = filament.pipeline.list(
    pagination=IngestionV1PaginationRequest(page_size=25),
)
next_cursor = page.pagination.next_cursor if page.pagination else None
~~~

Pass `next_cursor` as `IngestionV1PaginationRequest(cursor=next_cursor, page_size=25)`
to fetch the next page when a cursor is present. Omitting pagination returns the
full result set.

## Errors and retries

~~~python
from filament.core.api_error import ApiError

try:
    filament.pipeline.get(id="missing-pipeline")
except ApiError as error:
    print(error.status_code)  # e.g. 404
    print(error.body)         # Connect error code, message, and optional details
~~~

Automatic retries are disabled because mutations are not generally idempotent.
For a read you want to retry, use
`filament.pipeline.list(request_options={"max_retries": 2})`.

## Development

The client and models are generated by Fern from the protobuf-derived OpenAPI
specification. Configure names and generation in `fern/`; do not edit
`sdks/python/src/filament/` directly. This README and the package metadata are
maintained separately and preserved across regeneration.

From the repository root:

~~~sh
# Regenerate the OpenAPI specification and SDK (requires Buf and Fern login).
just sdks

# Install the SDK and run interoperability tests against an ephemeral Go server.
uv sync --project sdks/python
FILAMENT_SDK_PYTHON="$PWD/sdks/python/.venv/bin/python" GOWORK=off \
  go test ./sdks/python/tests -run TestPythonSDK -count=1

# Build the wheel and source distribution.
uv build --no-sources --project sdks/python
~~~

## License

Apache-2.0.
