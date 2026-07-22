import { create } from "@bufbuild/protobuf";
import { createFileRoute, Outlet, useNavigate, useParams, useSearch } from "@tanstack/react-router";

import PipelineLayout from "@/layouts/pipeline/PipelineLayout";
import { PipelineStatus } from "@/layouts/pipeline/types";

import PipelineCanvasProvider from "@/pages/pipelines/canvas/PipelineCanvasProvider";
import { mapPipelineVersionToCanvasState } from "@/pages/pipelines/canvas/utils";

import { useListConnectionsQuery } from "@/api/queries/connectors";
import {
  useGetPipelineQuery,
  useGetPipelineVersionQuery,
  useListPipelineVersionsQuery,
} from "@/api/queries/pipelines";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import { ListConnectionsRequestSchema } from "@/gen/ingestion/v1/connections_pb";
import {
  GetPipelineRequestSchema,
  GetPipelineVersionRequestSchema,
  ListPipelineVersionsRequestSchema,
} from "@/gen/ingestion/v1/pipelines_pb";

const PipelineLayoutRoute = () => {
  const { id } = useParams({ from: "/pipelines/$id" });
  const navigate = useNavigate();

  // The preview version lives in the canvas route's ?version search param, so
  // navigating to history/settings always drops back to the latest version.
  // null = latest (editable); a version number = read-only preview
  const search = useSearch({ strict: false }) as { version?: number };
  const previewVersion = search.version != null ? BigInt(search.version) : null;

  const setPreviewVersion = (version: bigint | null) => {
    void navigate({
      to: "/pipelines/$id/canvas",
      params: { id },
      search: version === null ? {} : { version: Number(version) },
    });
  };

  const { data: pipelineData, isLoading: isLoadingPipeline } = useGetPipelineQuery({
    input: create(GetPipelineRequestSchema, { id }),
  });
  // version 0 = the pipeline's current version. A pipeline with no saved
  // versions yet returns NotFound, which renders as an empty canvas.
  const { data: versionData, isLoading: isLoadingVersion } = useGetPipelineVersionQuery({
    input: create(GetPipelineVersionRequestSchema, { pipelineId: id }),
    options: { retry: false },
  });
  const { data: versionsData } = useListPipelineVersionsQuery({
    input: create(ListPipelineVersionsRequestSchema, { pipelineId: id }),
  });
  const { data: connectionsData, isLoading: isLoadingConnections } = useListConnectionsQuery({
    input: create(ListConnectionsRequestSchema, { kind: ConnectorKind.UNSPECIFIED }),
  });

  const pipeline = pipelineData?.pipeline;
  const version = versionData?.version;
  const versions = versionsData?.versions ?? [];

  // The preview graph comes straight from the already-loaded list; a selection
  // that no longer exists in the list falls back to the editable latest view,
  // and ?version pointing at the latest renders as latest (editable) too
  const latestVersion = versions[0]?.version;
  const previewed =
    previewVersion !== null && previewVersion !== latestVersion
      ? versions.find((v) => v.version === previewVersion)
      : undefined;
  const renderedVersion = previewed ?? version;

  if (isLoadingPipeline || isLoadingVersion || isLoadingConnections || !pipeline) {
    return null;
  }

  // The stable "latest" key preserves canvas state across saves; a version key
  // remounts the provider so the preview graph re-seeds the reducer
  const canvasKey = `${id}:${previewed ? previewed.version : "latest"}`;

  return (
    <PipelineCanvasProvider
      key={canvasKey}
      initialState={{
        ...mapPipelineVersionToCanvasState(renderedVersion, connectionsData?.connections ?? []),
        isReadOnly: Boolean(previewed),
      }}
    >
      <PipelineLayout
        pipeline={pipeline}
        currentVersion={version}
        status={PipelineStatus.DRAFT}
        versions={versions}
        previewVersion={previewed?.version ?? null}
        onPreviewVersionChange={setPreviewVersion}
      >
        <Outlet />
      </PipelineLayout>
    </PipelineCanvasProvider>
  );
};

export const Route = createFileRoute("/pipelines/$id")({
  component: PipelineLayoutRoute,
});
