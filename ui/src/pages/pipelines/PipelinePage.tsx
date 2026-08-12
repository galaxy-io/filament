import { useMemo } from "react";

import { create } from "@bufbuild/protobuf";
import { ArrowLeftIcon, LinkBreakIcon } from "@phosphor-icons/react";
import { notFound, Outlet, useNavigate, useParams } from "@tanstack/react-router";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";

import { GetPipelineRequestSchema } from "@/gen/ingestion/v1/pipelines_pb";

import ErrorLayout from "@/layouts/ErrorLayout";

import { mapPipelineVersionToCanvasState } from "@/pages/pipelines/canvas/graph/serialize";
import PipelineCanvasProvider from "@/pages/pipelines/canvas/providers/canvas/PipelineCanvasProvider";
import { usePipelinePreviewVersion } from "@/pages/pipelines/hooks/usePipelinePreviewVersion";
import PipelineLayout from "@/pages/pipelines/layout/PipelineLayout";

import { useSuspenseGetPipelineQuery } from "@/api/queries/pipelines";

const PipelinePage = () => {
  const { id } = useParams({ from: "/pipelines/$id" });
  const navigate = useNavigate();

  const { data: pipelineData } = useSuspenseGetPipelineQuery({
    input: create(GetPipelineRequestSchema, { id }),
  });

  const pipeline = pipelineData.pipeline;
  const version = pipelineData.currentVersion;

  const previewed = usePipelinePreviewVersion();

  const graph = useMemo(
    () => mapPipelineVersionToCanvasState(previewed ?? version),
    [previewed, version],
  );

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
      graph={graph}
      isReadOnly={Boolean(previewed)}
    >
      <PipelineLayout>
        <Outlet />
      </PipelineLayout>
    </PipelineCanvasProvider>
  );
};

export default PipelinePage;
