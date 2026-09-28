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

# 1. Create the sample source.
source = filament.connection.create(
    name=f"{name}-source", kind="CONNECTOR_KIND_SOURCE", connector="sample"
).connection

# 2. Create the stdout sink.
sink = filament.connection.create(
    name=f"{name}-sink", kind="CONNECTOR_KIND_SINK", connector="stdout"
).connection

# 3. Discover the source's resources.
resources = filament.connector.discover_resources(connection_id=source.id).resources or []
print("Discovered resources:", [resource.name for resource in resources])

# 4. Select all available resources, or use a subset such as ["users"].
selected_resources = [resource.name for resource in resources if resource.is_selectable]

# 5. Create the pipeline and save its source-to-sink graph.
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
                id="sink", kind="CONNECTOR_KIND_SINK", connection_id=sink.id,
            ),
        ],
        edges=[
            IngestionV1PipelineEdge(
                from_node="source", to_node="sink", resource=resource,
            )
            for resource in selected_resources
        ],
    ),
)

# 6. Run it. Sample rows appear in the worker's stdout logs.
print(f"Pipeline: {pipeline.id}")
print(filament.pipeline.run(pipeline_id=pipeline.id))
