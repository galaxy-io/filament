import { useEffect, useMemo, useRef } from "react";

import { styled } from "@linaria/react";
import { PulseIcon, XIcon } from "@phosphor-icons/react";

import Flashing from "@galaxy-io/dls/animations/Flashing";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import { PipelineCanvasActionType } from "@/pages/pipelines/canvas/actions";
import { CANVAS_TERMINAL_WIDTH } from "@/pages/pipelines/canvas/constants";
import { usePipelineCanvas } from "@/pages/pipelines/canvas/hooks";
import PipelineCanvasTerminalLine from "@/pages/pipelines/canvas/terminal/PipelineCanvasTerminalLine";

import { useTailRunsStream } from "@/api/queries/runs";

// Docked panel: the canvas page splits, canvas left / activity right
const TerminalWrapper = withTheme(styled.div<PropsWithTheme>`
  width: ${CANVAS_TERMINAL_WIDTH}px;
  max-width: 50%;
  height: 100%;
  flex-shrink: 0;

  display: flex;
  flex-direction: column;

  background-color: ${({ theme }) => theme.color.background.base};
  border-left: 0.5px solid ${({ theme }) => theme.color.border.primary};
`);

const TerminalHeader = styled.div`
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 10px;
  flex-shrink: 0;
`;

const HeaderSpacer = styled.div`
  flex: 1;
`;

const HeaderButton = withTheme(styled.button<PropsWithTheme>`
  width: 20px;
  height: 20px;
  padding: 0;

  display: flex;
  align-items: center;
  justify-content: center;

  background-color: transparent;
  border: none;
  border-radius: 4px;
  cursor: pointer;

  transition: background-color 100ms ease;

  &:hover {
    background-color: ${({ theme }) => theme.color.background.tertiary};
  }
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

  const runIds = useMemo(
    () => [...new Set(state.runBindings.map((binding) => binding.runId))],
    [state.runBindings],
  );

  const { events, isStreaming } = useTailRunsStream(runIds);

  const scrollRef = useRef<HTMLDivElement>(null);

  // Stick to the bottom as new lines arrive
  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [events.length]);

  const handleClose = () => {
    dispatch({ type: PipelineCanvasActionType.SET_ACTIVE_MODE, payload: null });
  };

  const renderBody = () => {
    if (runIds.length === 0) {
      return (
        <Text size={TextSize.CAPTION} variant={TextVariant.TERTIARY} isMonospace>
          Run the pipeline to see activity
        </Text>
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
      <TerminalHeader>
        <Icon component={PulseIcon} size={14} variant={IconVariant.SECONDARY} />
        <Text size={TextSize.BODY_SM} weight={TextWeight.MEDIUM}>
          Activity
        </Text>
        <HeaderSpacer />
        <HeaderButton onClick={handleClose}>
          <Icon component={XIcon} size={14} variant={IconVariant.TERTIARY} />
        </HeaderButton>
      </TerminalHeader>

      <HorizontalDivider />

      <TerminalBody ref={scrollRef}>{renderBody()}</TerminalBody>
    </TerminalWrapper>
  );
};

export default PipelineCanvasTerminal;
