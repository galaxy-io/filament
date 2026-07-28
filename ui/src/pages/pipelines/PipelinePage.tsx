import { create } from "@bufbuild/protobuf";
import { Outlet, useNavigate, useParams, useSearch } from "@tanstack/react-router";

import { GetPipelineRequestSchema } from "@/gen/ingestion/v1/pipelines_pb";

import { mapPipelineVersionToCanvasState } from "@/pages/pipelines/canvas/graph";
import PipelineCanvasProvider from "@/pages/pipelines/canvas/PipelineCanvasProvider";
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

  const { version: searchVersion } = useSearch({ from: "__root__" });
  const previewVersion = searchVersion != null ? BigInt(searchVersion) : null;
  const previewed =
    previewVersion !== null && previewVersion !== versions[0]?.version
      ? versions.find((item) => item.version === previewVersion)
      : undefined;

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
      key={`${id}:${previewed?.version ?? "latest"}`}
      initialState={{
        ...mapPipelineVersionToCanvasState(previewed ?? version, connectionsData.connections),
        isReadOnly: Boolean(previewed),
      }}
    >
      <PipelineLayout
        pipeline={pipeline}
        currentVersion={version}
        versions={versions}
        previewVersion={previewed?.version ?? null}
        onPreviewVersionChange={handlePreviewVersionChange}
      >
        <Outlet />
      </PipelineLayout>
    </PipelineCanvasProvider>
  );
};

export default PipelinePage;
