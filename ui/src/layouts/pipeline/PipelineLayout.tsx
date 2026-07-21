import type { PropsWithChildren } from "react";
import { useState } from "react";

import { create } from "@bufbuild/protobuf";
import { styled } from "@linaria/react";
import { useNavigate } from "@tanstack/react-router";

import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";
import { ToastVariant } from "@galaxy-io/dls/toast/Toast";
import { useToast } from "@galaxy-io/dls/toast/useToast";

import { PIPELINE_SIDEBAR_WIDTH } from "@/layouts/pipeline/constants";
import PipelineLayoutBackButton from "@/layouts/pipeline/PipelineLayoutBackButton";
import PipelineLayoutNavbar from "@/layouts/pipeline/PipelineLayoutNavbar";
import PipelineLayoutSidebar from "@/layouts/pipeline/PipelineLayoutSidebar";
import { PipelineSidebarItem, type PipelineStatus } from "@/layouts/pipeline/types";

import { PipelineCanvasActionType } from "@/pages/pipelines/canvas/actions";
import { usePipelineCanvas, usePipelineCanvasSave } from "@/pages/pipelines/canvas/hooks";
import { isPipelineRunnable } from "@/pages/pipelines/canvas/utils";

import { useRunPipelineMutation } from "@/api/queries/runs";

import type { Pipeline } from "@/gen/ingestion/v1/pipelines_pb";
import { RunPipelineRequestSchema } from "@/gen/ingestion/v1/runs_pb";

import { useRouteMatch } from "@/hooks/useRouteMatch";

const LayoutWrapper = withTheme(styled.div<PropsWithTheme>`
  width: 100%;
  height: 100%;

  display: flex;

  background-color: ${({ theme }) => theme.color.background.base};
`);

const LeftColumn = withTheme(styled.div<PropsWithTheme>`
  width: ${PIPELINE_SIDEBAR_WIDTH}px;
  height: 100%;

  flex-shrink: 0;

  display: flex;
  flex-direction: column;

  background-color: ${({ theme }) => theme.color.background.base};
`);

const RightColumn = styled.div`
  flex: 1;
  height: 100%;
  min-width: 0;

  display: flex;
  flex-direction: column;
`;

const ContentWrapper = withTheme(styled.div<PropsWithTheme>`
  flex: 1;
  width: 100%;
  min-height: 0;

  padding: 0 12px 12px 0;

  display: flex;

  background-color: ${({ theme }) => theme.color.background.base};
`);

const ContentIsland = withTheme(styled.div<PropsWithTheme>`
  flex: 1;
  width: 100%;
  min-height: 0;

  background-color: ${({ theme }) => theme.color.background.primary};

  border: 0.5px solid ${({ theme }) => theme.color.border.primary};
  border-radius: 6px;

  overflow: hidden;
`);

interface PipelineLayoutProps {
  pipeline: Pipeline;
  status: PipelineStatus;
}

interface PipelineLayoutState {
  isEnabled: boolean;
}

const DEFAULT_STATE: PipelineLayoutState = {
  isEnabled: false,
};

const PipelineLayout = ({ pipeline, status, children }: PropsWithChildren<PipelineLayoutProps>) => {
  const navigate = useNavigate();
  const { showToast } = useToast();
  const [state, setState] = useState<PipelineLayoutState>(DEFAULT_STATE);

  const { dispatch } = usePipelineCanvas();
  const { hasChanges, isSaving, save } = usePipelineCanvasSave(pipeline);
  const { mutate: runPipeline, isPending: isRunning } = useRunPipelineMutation();

  const { isRouteMatch: isHistoryActive } = useRouteMatch({
    route: "/pipelines/$id/history",
    fuzzy: false,
  });
  const { isRouteMatch: isSettingsActive } = useRouteMatch({
    route: "/pipelines/$id/settings",
    fuzzy: false,
  });

  const getActiveItem = (): PipelineSidebarItem => {
    if (isHistoryActive) return PipelineSidebarItem.HISTORY;
    if (isSettingsActive) return PipelineSidebarItem.SETTINGS;
    return PipelineSidebarItem.CANVAS;
  };

  const handleToggleEnabled = () => {
    setState((prev) => ({ ...prev, isEnabled: !prev.isEnabled }));
  };

  const handleItemClick = (item: PipelineSidebarItem) => {
    navigate({
      to: `/pipelines/$id/${item}`,
      params: { id: pipeline.id },
    });
  };

  const handleRun = () => {
    runPipeline(create(RunPipelineRequestSchema, { pipelineId: pipeline.id }), {
      onSuccess: (response) => {
        dispatch({
          type: PipelineCanvasActionType.SET_RUN_BINDINGS,
          payload: response.runs,
        });
        // Pop the activity terminal so the new run's facts are visible immediately
        dispatch({
          type: PipelineCanvasActionType.SET_ACTIVITY_OPEN,
          payload: true,
        });
        showToast({
          header: "Run started",
          subheader: `${pipeline.name} is now running.`,
          variant: ToastVariant.SUCCESS,
        });
      },
      onError: (error) => {
        showToast({
          header: "Run failed",
          subheader: error instanceof Error ? error.message : "Failed to run pipeline",
          variant: ToastVariant.ERROR,
        });
      },
    });
  };

  return (
    <LayoutWrapper>
      <LeftColumn>
        <PipelineLayoutBackButton />
        <PipelineLayoutSidebar activeItem={getActiveItem()} onItemClick={handleItemClick} />
      </LeftColumn>
      <RightColumn>
        <PipelineLayoutNavbar
          name={pipeline.name}
          status={status}
          isEnabled={state.isEnabled}
          onToggleEnabled={handleToggleEnabled}
          hasChanges={hasChanges}
          isSaving={isSaving}
          onSave={save}
          isRunning={isRunning}
          isRunDisabled={!isPipelineRunnable(pipeline)}
          onRun={handleRun}
        />
        <ContentWrapper>
          <ContentIsland>{children}</ContentIsland>
        </ContentWrapper>
      </RightColumn>
    </LayoutWrapper>
  );
};

export default PipelineLayout;
