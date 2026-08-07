import { create } from "@bufbuild/protobuf";
import { ArrowLeftIcon, LinkBreakIcon } from "@phosphor-icons/react";
import { notFound, Outlet, useNavigate, useParams, useSearch } from "@tanstack/react-router";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";

import { GetPipelineRequestSchema } from "@/gen/ingestion/v1/pipelines_pb";

import ErrorLayout from "@/layouts/ErrorLayout";

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

  const handleGoToPipelines = () => {
    void navigate({ to: "/pipelines" });
  };

  if (!pipeline) {
    throw notFound();
  }

  if (pipeline.deletedAt) {
    return (
      <ErrorLayout
        icon={<Icon component={LinkBreakIcon} size={24} variant={IconVariant.ERROR} />}
        header="Deleted pipeline"
        message="The pipeline you are looking for has been deleted"
        actions={
          <Button
            label="Go back to pipelines"
            icon={ArrowLeftIcon}
            variant={ButtonVariant.SECONDARY}
            onClick={handleGoToPipelines}
          />
        }
      />
    );
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
          schedule={pipelineData.schedule}
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
