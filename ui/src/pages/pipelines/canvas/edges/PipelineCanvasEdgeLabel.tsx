import { styled } from "@linaria/react";

import FlexWrapper, {
  AlignItems,
  FlexDirection,
  FlexGap,
} from "@galaxy-io/dls/containers/FlexWrapper";
import { InputSize } from "@galaxy-io/dls/inputs/Input";
import SelectInput from "@galaxy-io/dls/inputs/SelectInput";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import type { IngestionType } from "@/gen/ingestion/v1/common_pb";

import { PIPELINE_CANVAS_OVERLAY_Z_INDEX } from "@/pages/pipelines/canvas/constants";
import { PIPELINE_CANVAS_EDGE_SELECT_WIDTH } from "@/pages/pipelines/canvas/edges/constants";
import PipelineCanvasEdgeCursor from "@/pages/pipelines/canvas/edges/PipelineCanvasEdgeCursor";
import {
  getBlockingMessages,
  getCursorRequirement,
  getEdgeCursor,
  getIngestionTypeOptions,
} from "@/pages/pipelines/canvas/edges/utils";
import {
  usePipelineCanvasActions,
  usePipelineCanvasReadOnly,
  usePipelineCanvasState,
} from "@/pages/pipelines/canvas/providers/canvas/PipelineCanvasProvider";
import { usePipelineCanvasValidation } from "@/pages/pipelines/canvas/providers/validation/PipelineCanvasValidationProvider";
import {
  getCanvasEdgeResource,
  getPipelineCanvasEdgeData,
  isIncrementalIngestionType,
} from "@/pages/pipelines/canvas/utils";

const LabelWrapper = styled.div<{ $labelX: number; $labelY: number }>`
  position: absolute;
  transform: translate(-50%, -50%)
    translate(${({ $labelX }) => $labelX}px, ${({ $labelY }) => $labelY}px);
  pointer-events: all;
  z-index: ${PIPELINE_CANVAS_OVERLAY_Z_INDEX};
`;

const SelectWrapper = withTheme(styled.div<PropsWithTheme<{ $isSelected: boolean }>>`
  background-color: ${({ theme }) => theme.color.background.base};
  border: ${({ theme, $isSelected }) =>
    $isSelected
      ? `2px solid ${theme.color.background.galaxy}`
      : `0.5px solid ${theme.color.border.primary}`};

  border-radius: 5px;
`);

interface PipelineCanvasEdgeLabelProps {
  edgeId: string;
  isSelected: boolean;
  labelX: number;
  labelY: number;
}

const PipelineCanvasEdgeLabel = ({
  edgeId,
  isSelected,
  labelX,
  labelY,
}: PipelineCanvasEdgeLabelProps) => {
  const isReadOnly = usePipelineCanvasReadOnly();
  const state = usePipelineCanvasState();
  const { setEdgeIngestionType, setEdgeCursor } = usePipelineCanvasActions();

  const { getEdgeValidation, isStale } = usePipelineCanvasValidation();

  const edge = state.edges.find((candidate) => candidate.id === edgeId);
  if (!edge) return null;

  const data = getPipelineCanvasEdgeData(edge);
  const resource = getCanvasEdgeResource(edge);
  const isIncremental = isIncrementalIngestionType(data.ingestionType);

  const validation = getEdgeValidation(edge);
  const options = getIngestionTypeOptions(validation, resource, data.ingestionType);
  const selectedOption = options.find((option) => option.value === data.ingestionType) ?? null;
  const cursorRequirement = getCursorRequirement(validation, resource);
  const blockingMessages = isStale ? [] : getBlockingMessages(validation);

  return (
    <LabelWrapper className="nodrag nopan" $labelX={labelX} $labelY={labelY}>
      <FlexWrapper
        direction={FlexDirection.COLUMN}
        alignItems={AlignItems.END}
        gap={FlexGap.XSMALL}
      >
        <SelectWrapper $isSelected={isSelected}>
          <SelectInput
            options={options}
            value={selectedOption}
            onChange={(option) => setEdgeIngestionType(edgeId, option.value as IngestionType)}
            size={InputSize.SMALL}
            width={PIPELINE_CANVAS_EDGE_SELECT_WIDTH}
            isDisabled={isReadOnly}
          />
        </SelectWrapper>

        {isIncremental && !!resource && (
          <PipelineCanvasEdgeCursor
            requirement={cursorRequirement}
            cursor={getEdgeCursor(edge, resource)}
            isDisabled={isReadOnly}
            onChange={(field, lookbackSeconds) =>
              setEdgeCursor(edgeId, resource, field, lookbackSeconds)
            }
          />
        )}

        {blockingMessages.map((message) => (
          <Text key={message} size={TextSize.CAPTION} variant={TextVariant.ERROR}>
            {message}
          </Text>
        ))}
      </FlexWrapper>
    </LabelWrapper>
  );
};

export default PipelineCanvasEdgeLabel;
