import { type FC, useMemo } from "react";

import { ArrowLeftIcon, LinkBreakIcon } from "@phosphor-icons/react";
import { notFound, Outlet } from "@tanstack/react-router";

import Button from "@galaxy-io/dls/buttons/Button";
import ErrorLayout from "@galaxy-io/dls/layout/ErrorLayout";
import PageLayout from "@galaxy-io/dls/layout/PageLayout";

import RouterLink from "@/components/RouterLink";

import { mapPipelineVersionToCanvasState } from "@/pages/pipelines/canvas/graph/serialize";
import PipelineCanvasProvider from "@/pages/pipelines/canvas/providers/canvas/PipelineCanvasProvider";
import PipelinePageActions from "@/pages/pipelines/components/header/PipelinePageActions";
import PipelinePageHeading from "@/pages/pipelines/components/header/PipelinePageHeading";
import PipelinePagePreviewBanner from "@/pages/pipelines/components/header/PipelinePagePreviewBanner";
import PipelinePageScheduleBanner from "@/pages/pipelines/components/header/PipelinePageScheduleBanner";
import PipelinePageTabs from "@/pages/pipelines/components/header/PipelinePageTabs";
import { usePipelinePreviewVersion } from "@/pages/pipelines/hooks/usePipelinePreviewVersion";

import { usePipelineParams } from "@/module/hooks";
import { createFilamentHref, FilamentPath } from "@/module/paths";

import { createGetPipelineInput, useSuspenseGetPipelineQuery } from "@/api/queries/pipelines";

const PipelinePage: FC = () => {
  const { id } = usePipelineParams();

  const { data: pipelineData } = useSuspenseGetPipelineQuery({
    input: createGetPipelineInput(id),
  });

  const pipeline = pipelineData.pipeline;
  const version = pipelineData.pipeline?.currentVersion;

  const previewed = usePipelinePreviewVersion();

  const graph = useMemo(
    () => mapPipelineVersionToCanvasState(previewed ?? version),
    [previewed, version],
  );

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
            href={createFilamentHref(FilamentPath.PIPELINES)}
            as={RouterLink}
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
      <PageLayout
        header={<PipelinePageHeading />}
        actions={<PipelinePageActions />}
        tabs={<PipelinePageTabs />}
        banner={previewed ? <PipelinePagePreviewBanner /> : <PipelinePageScheduleBanner />}
      >
        <Outlet />
      </PageLayout>
    </PipelineCanvasProvider>
  );
};

export default PipelinePage;
