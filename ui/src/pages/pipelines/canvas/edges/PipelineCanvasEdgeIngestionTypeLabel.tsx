import { useMemo } from "react";

import { styled } from "@linaria/react";
import { InfoIcon } from "@phosphor-icons/react";

import Wrapper from "@galaxy-io/dls/containers/Wrapper";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import SelectInput, {
  type SelectInputOption,
  SelectInputSize,
} from "@galaxy-io/dls/inputs/SelectInput";
import Text, { TextSize } from "@galaxy-io/dls/text/Text";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";
import Tooltip, { TooltipPosition } from "@galaxy-io/dls/tooltip/Tooltip";

import { ConnectorKind, type IngestionType } from "@/gen/ingestion/v1/common_pb";

import { useConnectorSpec } from "@/pages/connectors/hooks/useConnectorSpec";
import { PIPELINE_CANVAS_OVERLAY_Z_INDEX } from "@/pages/pipelines/canvas/constants";
import {
  INGESTION_TYPE_TO_DESCRIPTION_MAP,
  INGESTION_TYPE_TO_LABEL_MAP,
  PIPELINE_CANVAS_EDGE_MODE_SELECT_WIDTH,
  PIPELINE_CANVAS_EDGE_TOOLTIP_MAX_WIDTH,
} from "@/pages/pipelines/canvas/edges/constants";
import {
  getSupportedIngestionTypes,
  PIPELINE_CANVAS_INGESTION_TYPES,
} from "@/pages/pipelines/canvas/graph/capabilities";
import { PIPELINE_CANVAS_NODE_GAP } from "@/pages/pipelines/canvas/nodes/constants";
import PipelineCanvasActionButton from "@/pages/pipelines/canvas/PipelineCanvasActionButton";
import {
  usePipelineCanvasActions,
  usePipelineCanvasReadOnly,
  usePipelineCanvasState,
} from "@/pages/pipelines/canvas/providers/canvas/PipelineCanvasProvider";
import { isConnectionNode } from "@/pages/pipelines/canvas/types";

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

const SelectWrapper = withTheme(styled.div<PropsWithTheme<{ $isSelected?: boolean }>>`
  background-color: ${({ theme }) => theme.color.background.base};
  border: ${({ theme, $isSelected }) =>
    $isSelected
      ? `2px solid ${theme.color.background.galaxy}`
      : `0.5px solid ${theme.color.border.primary}`};

  border-radius: 6px;
`);

interface PipelineCanvasEdgeIngestionTypeLabelProps {
  edgeId: string;
  sourceNodeId: string;
  targetNodeId: string;
  ingestionType: IngestionType;
  isSelected: boolean;
  labelX: number;
  labelY: number;
}

const PipelineCanvasEdgeIngestionTypeLabel = ({
  edgeId,
  sourceNodeId,
  targetNodeId,
  ingestionType,
  isSelected,
  labelX,
  labelY,
}: PipelineCanvasEdgeIngestionTypeLabelProps) => {
  const isReadOnly = usePipelineCanvasReadOnly();
  const state = usePipelineCanvasState();
  const { setEdgeIngestionType } = usePipelineCanvasActions();

  const getNodeConnector = (nodeId: string) => {
    const node = state.nodes.find((candidate) => candidate.id === nodeId);
    return node && isConnectionNode(node) ? node.data.connector : "";
  };

  const sourceSpec = useConnectorSpec(getNodeConnector(sourceNodeId), ConnectorKind.SOURCE);
  const sinkSpec = useConnectorSpec(getNodeConnector(targetNodeId), ConnectorKind.SINK);

  const options: SelectInputOption[] = useMemo(() => {
    const supported = getSupportedIngestionTypes(sourceSpec, sinkSpec);
    return PIPELINE_CANVAS_INGESTION_TYPES.filter(
      (type) => supported.includes(type) || type === ingestionType,
    ).map((type) => ({
      id: String(type),
      label: INGESTION_TYPE_TO_LABEL_MAP[type],
      value: type,
    }));
  }, [sourceSpec, sinkSpec, ingestionType]);

  const selectedOption = options.find((option) => option.value === ingestionType) ?? null;

  return (
    <LabelWrapper className="nodrag nopan" $labelX={labelX} $labelY={labelY}>
      <ActionsWrapper>
        <Tooltip
          body={
            <Wrapper maxWidth={PIPELINE_CANVAS_EDGE_TOOLTIP_MAX_WIDTH}>
              <Text size={TextSize.CAPTION}>
                {INGESTION_TYPE_TO_DESCRIPTION_MAP[ingestionType]}
              </Text>
            </Wrapper>
          }
          position={TooltipPosition.TOP}
        >
          <PipelineCanvasActionButton className="nodrag">
            <Icon component={InfoIcon} size={14} variant={IconVariant.TERTIARY} />
          </PipelineCanvasActionButton>
        </Tooltip>
      </ActionsWrapper>
      <SelectWrapper $isSelected={isSelected}>
        <SelectInput
          options={options}
          value={selectedOption}
          onChange={(option) => setEdgeIngestionType(edgeId, option.value as IngestionType)}
          size={SelectInputSize.SMALL}
          width={PIPELINE_CANVAS_EDGE_MODE_SELECT_WIDTH}
          isDisabled={isReadOnly}
        />
      </SelectWrapper>
    </LabelWrapper>
  );
};

export default PipelineCanvasEdgeIngestionTypeLabel;
