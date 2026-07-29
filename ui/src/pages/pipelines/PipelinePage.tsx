import { create } from "@bufbuild/protobuf";
import { Outlet, useNavigate, useParams, useSearch } from "@tanstack/react-router";

import { GetPipelineRequestSchema } from "@/gen/ingestion/v1/pipelines_pb";

import { mapPipelineVersionToCanvasState } from "@/pages/pipelines/canvas/graph/serialize";
import PipelineCanvasProvider from "@/pages/pipelines/canvas/providers/canvas/PipelineCanvasProvider";
import PipelineCanvasRunProvider from "@/pages/pipelines/canvas/providers/run/PipelineCanvasRunProvider";
import PipelineLayout from "@/pages/pipelines/layout/PipelineLayout";

import { useSuspenseListConnectionsQuery } from "@/api/queries/connections";
import { useSuspenseGetPipelineQuery } from "@/api/queries/pipelines";

const PipelinePage = () => {
  const { id } = useParams({ from: "/pipelines/$id" });
  const navigate = useNavigate();

  const { data: pipelineData } = useSuspenseGetPipelineQuery({
    input: create(GetPipelineRequestSchema, { id }),
  });
  const { data: connectionsData } = useSuspenseListConnectionsQuery();

  const pipeline = pipelineData.pipeline;
  const version = pipelineData.currentVersion;
  const versions = pipelineData.versions;

  const { version: searchVersion } = useSearch({ from: "/pipelines/$id" });
  const previewed = versions.find(
    (item) => item.version !== versions[0]?.version && Number(item.version) === searchVersion,
  );

  const handlePreviewVersionChange = (nextVersion: bigint | null) => {
    void navigate({
      to: "/pipelines/$id/canvas",
      params: { id },
      search: nextVersion === null ? {} : { version: Number(nextVersion) },
    });
  };

  if (!pipeline) {
    throw new Error(`Pipeline ${id} not found`);
  }

  return (
    <PipelineCanvasProvider
      graphKey={`${id}:${previewed?.version ?? "latest"}`}
      graph={mapPipelineVersionToCanvasState(previewed ?? version, connectionsData.connections)}
      isReadOnly={Boolean(previewed)}
    >
      <PipelineCanvasRunProvider>
        <PipelineLayout
          pipeline={pipeline}
          currentVersion={version}
          versions={versions}
          connections={connectionsData.connections}
          previewVersion={previewed?.version ?? null}
          onPreviewVersionChange={handlePreviewVersionChange}
        >
          <Outlet />
        </PipelineLayout>
      </PipelineCanvasRunProvider>
    </PipelineCanvasProvider>
  );
};

export default PipelinePage;
