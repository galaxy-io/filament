import { type FC, useMemo } from "react";

import { create } from "@bufbuild/protobuf";
import { ArrowLeftIcon, LinkBreakIcon } from "@phosphor-icons/react";
import { notFound, Outlet } from "@tanstack/react-router";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import ErrorLayout from "@galaxy-io/dls/layout/ErrorLayout";

import { GetPipelineRequestSchema } from "@/gen/ingestion/v1/pipelines_pb";

import PipelineLayout from "@/layouts/pipeline/PipelineLayout";

import { mapPipelineVersionToCanvasState } from "@/pages/pipelines/canvas/graph/serialize";
import PipelineCanvasProvider from "@/pages/pipelines/canvas/providers/canvas/PipelineCanvasProvider";
import { usePipelinePreviewVersion } from "@/pages/pipelines/hooks/usePipelinePreviewVersion";

import { useFilamentNavigate, usePipelineParams } from "@/module/hooks";
import { FilamentPath } from "@/module/paths";

import { useSuspenseGetPipelineQuery } from "@/api/queries/pipelines";

const PipelinePage: FC = () => {
  const { id } = usePipelineParams();
  const navigate = useFilamentNavigate();

  const { data: pipelineData } = useSuspenseGetPipelineQuery({
    input: create(GetPipelineRequestSchema, { id }),
  });

  const pipeline = pipelineData.pipeline;
  const version = pipelineData.pipeline?.currentVersion;

  const previewed = usePipelinePreviewVersion();

  const graph = useMemo(
    () => mapPipelineVersionToCanvasState(previewed ?? version),
    [previewed, version],
  );

  const handleGoToPipelines = () => {
    void navigate({ to: FilamentPath.PIPELINES });
  };

  if (!pipeline) {
    throw notFound();
  }

  if (pipeline.deletedAt) {
    return (
      <ErrorLayout
        icon={LinkBreakIcon}
        header="Deleted pipeline"
        description="The pipeline you are looking for has been deleted"
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
