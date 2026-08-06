import { styled } from "@linaria/react";
import { InfoIcon } from "@phosphor-icons/react";

import FlexWrapper, {
  AlignItems,
  FlexDirection,
  FlexGap,
} from "@galaxy-io/dls/containers/FlexWrapper";
import Wrapper from "@galaxy-io/dls/containers/Wrapper";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import { InputSize } from "@galaxy-io/dls/inputs/Input";
import SelectInput from "@galaxy-io/dls/inputs/SelectInput";
import Text, { TextSize } from "@galaxy-io/dls/text/Text";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";
import Tooltip, { TooltipPosition } from "@galaxy-io/dls/tooltip/Tooltip";

import { ConnectorKind, type IngestionType } from "@/gen/ingestion/v1/common_pb";

import { useConnectorSpec } from "@/pages/connectors/hooks/useConnectorSpec";
import { PIPELINE_CANVAS_OVERLAY_Z_INDEX } from "@/pages/pipelines/canvas/constants";
import {
  INGESTION_TYPE_TO_DESCRIPTION_MAP,
  PIPELINE_CANVAS_EDGE_SELECT_WIDTH,
  PIPELINE_CANVAS_EDGE_TOOLTIP_MAX_WIDTH,
} from "@/pages/pipelines/canvas/edges/constants";
import PipelineCanvasEdgeCursor from "@/pages/pipelines/canvas/edges/PipelineCanvasEdgeCursor";
import { getEdgeCursor, getIngestionTypeOptions } from "@/pages/pipelines/canvas/edges/utils";
import { usePipelineCanvasResourceColumns } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasResourceColumns";
import { PIPELINE_CANVAS_NODE_GAP } from "@/pages/pipelines/canvas/nodes/constants";
import PipelineCanvasActionButton from "@/pages/pipelines/canvas/PipelineCanvasActionButton";
import {
  usePipelineCanvasActions,
  usePipelineCanvasReadOnly,
  usePipelineCanvasState,
} from "@/pages/pipelines/canvas/providers/canvas/PipelineCanvasProvider";
import { isConnectionNode } from "@/pages/pipelines/canvas/types";
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

const ActionsWrapper = styled.div`
  position: absolute;
  bottom: 100%;
  right: 0;
  padding-bottom: ${PIPELINE_CANVAS_NODE_GAP}px;

  display: flex;
  align-items: center;
  gap: 4px;
`;

const SelectWrapper = withTheme(styled.div<PropsWithTheme<{ $isSelected: boolean }>>`
  background-color: ${({ theme }) => theme.color.background.base};
  border: ${({ theme, $isSelected }) =>
    $isSelected
      ? `2px solid ${theme.color.background.galaxy}`
      : `0.5px solid ${theme.color.border.primary}`};

  border-radius: 6px;
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

  const edge = state.edges.find((candidate) => candidate.id === edgeId);
  const sourceNode = state.nodes.find((node) => node.id === edge?.source);
  const connector =
    sourceNode && isConnectionNode(sourceNode) ? sourceNode.data.connector : undefined;
  const connectionId =
    sourceNode && isConnectionNode(sourceNode) ? sourceNode.data.connectionId : "";
  const spec = useConnectorSpec(connector ?? "", ConnectorKind.SOURCE);

  const data = edge ? getPipelineCanvasEdgeData(edge) : null;
  const resource = edge ? getCanvasEdgeResource(edge) : "";
  const isIncremental = data ? isIncrementalIngestionType(data.ingestionType) : false;

  const { columns, error, isLoading } = usePipelineCanvasResourceColumns({
    connectionId,
    resource,
    isEnabled: isIncremental,
  });

  if (!edge || !data) return null;

  const options = getIngestionTypeOptions(spec, data.ingestionType);
  const selectedOption = options.find((option) => option.value === data.ingestionType) ?? null;

  return (
    <LabelWrapper className="nodrag nopan" $labelX={labelX} $labelY={labelY}>
      <ActionsWrapper>
        <Tooltip
          body={
            <Wrapper maxWidth={PIPELINE_CANVAS_EDGE_TOOLTIP_MAX_WIDTH}>
              <Text size={TextSize.BODY_SM}>
                {INGESTION_TYPE_TO_DESCRIPTION_MAP[data.ingestionType]}
              </Text>
            </Wrapper>
          }
          position={TooltipPosition.TOP}
        >
          <PipelineCanvasActionButton className="nodrag" type="button">
            <Icon component={InfoIcon} size={14} variant={IconVariant.TERTIARY} />
          </PipelineCanvasActionButton>
        </Tooltip>
      </ActionsWrapper>

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
            columns={columns}
            cursor={getEdgeCursor(edge, resource)}
            error={error}
            isLoading={isLoading}
            isDisabled={isReadOnly}
            onChange={(field, lookbackSeconds) =>
              setEdgeCursor(edgeId, resource, field, lookbackSeconds)
            }
          />
        )}
      </FlexWrapper>
    </LabelWrapper>
  );
};

export default PipelineCanvasEdgeLabel;
