import { create } from "@bufbuild/protobuf";
import { Outlet, useNavigate, useParams, useSearch } from "@tanstack/react-router";

import {
  GetPipelineRequestSchema,
  GetPipelineVersionRequestSchema,
  ListPipelineVersionsRequestSchema,
} from "@/gen/ingestion/v1/pipelines_pb";

import PendingLayout from "@/layouts/PendingLayout";

import { mapPipelineVersionToCanvasState } from "@/pages/pipelines/canvas/graph";
import PipelineCanvasProvider from "@/pages/pipelines/canvas/PipelineCanvasProvider";
import PipelineLayout from "@/pages/pipelines/layout/PipelineLayout";

import { useSuspenseListConnectionsQuery } from "@/api/queries/connections";
import {
  useGetPipelineVersionQuery,
  useSuspenseListPipelineVersionsQuery,
} from "@/api/queries/pipeline_versions";
import { useSuspenseGetPipelineQuery } from "@/api/queries/pipelines";

const PipelinePage = () => {
  const { id } = useParams({ from: "/pipelines/$id" });
  const navigate = useNavigate();

  const { data: pipelineData } = useSuspenseGetPipelineQuery({
    input: create(GetPipelineRequestSchema, { id }),
  });
  const { data: versionsData } = useSuspenseListPipelineVersionsQuery({
    input: create(ListPipelineVersionsRequestSchema, { pipelineId: id }),
  });
  const { data: connectionsData } = useSuspenseListConnectionsQuery();

  const { data: versionData, isPending: isVersionPending } = useGetPipelineVersionQuery({
    input: create(GetPipelineVersionRequestSchema, { pipelineId: id }),
    options: { retry: false },
  });

  const pipeline = pipelineData.pipeline;
  const version = versionData?.version;
  const versions = versionsData.versions;

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

  if (isVersionPending) {
    return <PendingLayout />;
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
