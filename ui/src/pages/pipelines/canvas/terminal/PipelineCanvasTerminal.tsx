import { useEffect, useMemo, useRef } from "react";

import { styled } from "@linaria/react";
import { ArrowsOutSimpleIcon, ListIcon, PulseIcon, TerminalIcon } from "@phosphor-icons/react";

import Flashing from "@galaxy-io/dls/animations/Flashing";
import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import BaseHeader from "@/layouts/components/BaseHeader";
import { BaseHeaderSize } from "@/layouts/components/types";
import EmptyLayout from "@/layouts/EmptyLayout";

import { PipelineCanvasActionType } from "@/pages/pipelines/canvas/actions";
import {
  CANVAS_TERMINAL_HEIGHT,
  CANVAS_TERMINAL_NOTCH_WIDTH,
  CANVAS_TERMINAL_RIGHT_OFFSET,
  CANVAS_TERMINAL_WIDTH,
} from "@/pages/pipelines/canvas/constants";
import { usePipelineCanvas } from "@/pages/pipelines/canvas/hooks";
import PipelineCanvasTerminalLine from "@/pages/pipelines/canvas/terminal/PipelineCanvasTerminalLine";

import { useTailRunsStream } from "@/api/queries/runs";

const TerminalWrapper = styled.div`
  position: absolute;
  bottom: 0;
  right: ${CANVAS_TERMINAL_RIGHT_OFFSET}px;
  z-index: 1001;

  display: flex;
  flex-direction: column;
  align-items: flex-end;
`;

const HeaderBar = withTheme(styled.div<PropsWithTheme<{ $isOpen: boolean }>>`
  width: ${({ $isOpen }) => ($isOpen ? "100%" : `${CANVAS_TERMINAL_NOTCH_WIDTH}px`)};
  padding: 8px 12px;

  background-color: ${({ theme }) => theme.color.background.primary};
  border: 0.5px solid ${({ theme }) => theme.color.border.primary};
  border-bottom: none;
  border-radius: 6px 6px 0 0;
  cursor: ${({ $isOpen }) => ($isOpen ? "default" : "pointer")};
`);

const PanelClip = styled.div<{ $isOpen: boolean }>`
  width: ${CANVAS_TERMINAL_WIDTH}px;
  max-width: 40vw;
  height: ${({ $isOpen }) => ($isOpen ? `${CANVAS_TERMINAL_HEIGHT}px` : "0px")};
  overflow: hidden;

  transition: height 150ms ease;
`;

const Panel = withTheme(styled.div<PropsWithTheme>`
  width: 100%;
  height: ${CANVAS_TERMINAL_HEIGHT}px;

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
  flex-direction: column;
  gap: 2px;
  padding: 8px 10px;
`;

const PipelineCanvasTerminal = () => {
  const { state, dispatch } = usePipelineCanvas();

  const isOpen = state.isActivityOpen;

  const runIds = useMemo(
    () => [...new Set(state.runBindings.map((binding) => binding.runId))],
    [state.runBindings],
  );

  const { events, isStreaming } = useTailRunsStream(runIds);

  const scrollRef = useRef<HTMLDivElement>(null);

  // biome-ignore lint/correctness/useExhaustiveDependencies: re-scroll when lines arrive or the panel opens
  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [events.length, isOpen]);

  const setIsOpen = (payload: boolean) => {
    dispatch({ type: PipelineCanvasActionType.SET_ACTIVITY_OPEN, payload });
  };

  const renderBody = () => {
    if (runIds.length === 0) {
      return (
        <EmptyLayout
          icon={<Icon component={ListIcon} size={16} variant={IconVariant.SECONDARY} />}
          message="Run the pipeline to see activity"
        />
      );
    }

    return (
      <>
        {events.map((event, index) => (
          // biome-ignore lint/suspicious/noArrayIndexKey: append-only feed without stable identity
          <PipelineCanvasTerminalLine key={index} event={event} />
        ))}
        {isStreaming && (
          <Text size={TextSize.CAPTION} variant={TextVariant.TERTIARY} isMonospace>
            <Flashing>Listening...</Flashing>
          </Text>
        )}
      </>
    );
  };

  return (
    <TerminalWrapper>
      <HeaderBar $isOpen={isOpen} onClick={isOpen ? undefined : () => setIsOpen(true)}>
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
                    variant={ButtonVariant.TERTIARY}
                    size={ButtonSize.SMALL}
                    onClick={() => setIsOpen(true)}
                  />,
                ]
          }
          onClose={isOpen ? () => setIsOpen(false) : undefined}
        />
      </HeaderBar>

      <PanelClip $isOpen={isOpen}>
        <Panel>
          <TerminalBody ref={scrollRef}>{renderBody()}</TerminalBody>
        </Panel>
      </PanelClip>
    </TerminalWrapper>
  );
};

export default PipelineCanvasTerminal;
