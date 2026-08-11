import { useMemo, useState } from "react";

import { create } from "@bufbuild/protobuf";
import { styled } from "@linaria/react";
import { ArrowsOutSimpleIcon, PulseIcon } from "@phosphor-icons/react";
import { useParams } from "@tanstack/react-router";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";
import Flash from "@galaxy-io/dls/transform/Flash";

import { ListRunsRequestSchema, type RunInfo } from "@/gen/ingestion/v1/runs_pb";

import BaseHeader, { BaseHeaderSize } from "@/layouts/components/BaseHeader";
import EmptyLayout from "@/layouts/EmptyLayout";

import { PIPELINE_CANVAS_OVERLAY_Z_INDEX } from "@/pages/pipelines/canvas/constants";
import {
  usePipelineCanvasRunActions,
  usePipelineCanvasRunState,
} from "@/pages/pipelines/canvas/providers/run/PipelineCanvasRunProvider";
import {
  PIPELINE_CANVAS_TERMINAL_HEIGHT,
  PIPELINE_CANVAS_TERMINAL_MAX_RUNS,
  PIPELINE_CANVAS_TERMINAL_NOTCH_WIDTH,
  PIPELINE_CANVAS_TERMINAL_RIGHT_OFFSET,
  PIPELINE_CANVAS_TERMINAL_WIDTH,
} from "@/pages/pipelines/canvas/terminal/constants";
import PipelineCanvasTerminalLine from "@/pages/pipelines/canvas/terminal/PipelineCanvasTerminalLine";

import { ACTIVE_RUN_STATUSES } from "@/api/queries/constants";
import { useListRunsQuery, useTailRunsStream } from "@/api/queries/runs";

const TerminalWrapper = styled.div`
  position: absolute;
  bottom: 0;
  right: ${PIPELINE_CANVAS_TERMINAL_RIGHT_OFFSET}px;
  z-index: ${PIPELINE_CANVAS_OVERLAY_Z_INDEX};

  display: flex;
  flex-direction: column;
  align-items: flex-end;
`;

const HeaderBar = withTheme(styled.div<PropsWithTheme<{ $isOpen: boolean }>>`
  width: ${({ $isOpen }) => ($isOpen ? "100%" : `${PIPELINE_CANVAS_TERMINAL_NOTCH_WIDTH}px`)};
  padding: 8px 8px 8px 12px;

  background-color: ${({ theme }) => theme.color.background.primary};
  border: 0.5px solid ${({ theme }) => theme.color.border.primary};
  border-bottom: none;
  border-radius: 6px 6px 0 0;
  cursor: ${({ $isOpen }) => ($isOpen ? "default" : "pointer")};
`);

const PanelClip = styled.div<{ $isOpen: boolean }>`
  width: ${PIPELINE_CANVAS_TERMINAL_WIDTH}px;
  max-width: 40vw;
  height: ${({ $isOpen }) => ($isOpen ? `${PIPELINE_CANVAS_TERMINAL_HEIGHT}px` : "0px")};
  overflow: hidden;

  transition: height 150ms ease;
`;

const Panel = withTheme(styled.div<PropsWithTheme>`
  width: 100%;
  height: ${PIPELINE_CANVAS_TERMINAL_HEIGHT}px;

  display: flex;
  flex-direction: column;

  background-color: ${({ theme }) => theme.color.background.primary};
  border: 0.5px solid ${({ theme }) => theme.color.border.primary};
  border-bottom: none;
`);

const TerminalBody = styled.div`
  flex: 1;
  min-height: 0;
  overflow-y: auto;

  display: flex;
  flex-direction: column-reverse;
  gap: 2px;
  padding: 8px 10px;
`;

const PipelineCanvasTerminal = () => {
  const { id } = useParams({ from: "/pipelines/$id" });
  const { isActivityOpen: isOpen } = usePipelineCanvasRunState();
  const { setActivityOpen } = usePipelineCanvasRunActions();

  const { data: activeRunsData } = useListRunsQuery({
    input: create(ListRunsRequestSchema, {
      pipelineId: id,
      status: [...ACTIVE_RUN_STATUSES],
    }),
  });

  const [runIds, setRunIds] = useState<RunInfo["runId"][]>([]);
  const mergedRunIds = [
    ...new Set([...runIds, ...(activeRunsData?.runs ?? []).map((run) => run.runId)]),
  ].slice(-PIPELINE_CANVAS_TERMINAL_MAX_RUNS);
  if (mergedRunIds.join("|") !== runIds.join("|")) {
    setRunIds(mergedRunIds);
  }

  const { events, isStreaming } = useTailRunsStream(runIds);
  const reversedEvents = useMemo(() => [...events].reverse(), [events]);

  const renderBody = () => {
    if (runIds.length === 0) {
      return (
        <EmptyLayout
          icon={<Icon component={PulseIcon} size={16} variant={IconVariant.SECONDARY} />}
          message="Run the pipeline to see activity"
        />
      );
    }

    return (
      <>
        {isStreaming && (
          <Text size={TextSize.CAPTION} variant={TextVariant.TERTIARY} isMonospace>
            <Flash>Listening...</Flash>
          </Text>
        )}
        {reversedEvents.map((event, index) => (
          // biome-ignore lint/suspicious/noArrayIndexKey: append-only feed without stable identity
          <PipelineCanvasTerminalLine key={index} event={event} />
        ))}
      </>
    );
  };

  return (
    <TerminalWrapper>
      <HeaderBar $isOpen={isOpen} onClick={isOpen ? undefined : () => setActivityOpen(true)}>
        <BaseHeader
          size={BaseHeaderSize.SMALL}
          title="Activity"
          icon={PulseIcon}
          actions={
            isOpen
              ? undefined
              : [
                  <Button
                    key="expand"
                    icon={ArrowsOutSimpleIcon}
                    variant={ButtonVariant.SECONDARY}
                    size={ButtonSize.SMALL}
                    onClick={() => setActivityOpen(true)}
                  />,
                ]
          }
          onClose={isOpen ? () => setActivityOpen(false) : undefined}
        />
      </HeaderBar>

      <PanelClip $isOpen={isOpen}>
        <Panel>
          <TerminalBody>{renderBody()}</TerminalBody>
        </Panel>
      </PanelClip>
    </TerminalWrapper>
  );
};

export default PipelineCanvasTerminal;
