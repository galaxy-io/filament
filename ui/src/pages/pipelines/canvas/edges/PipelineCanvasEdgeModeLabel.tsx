import { useMemo } from "react";

import { styled } from "@linaria/react";

import SelectInput, {
  type SelectInputOption,
  SelectInputSize,
} from "@galaxy-io/dls/inputs/SelectInput";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import { ConnectorKind, type IngestionType } from "@/gen/ingestion/v1/common_pb";

import { useConnectorSpec } from "@/pages/connectors/hooks/useConnectorSpec";
import { PIPELINE_CANVAS_OVERLAY_Z_INDEX } from "@/pages/pipelines/canvas/constants";
import {
  INGESTION_TYPE_TO_LABEL_MAP,
  PIPELINE_CANVAS_EDGE_MODE_SELECT_WIDTH,
} from "@/pages/pipelines/canvas/edges/constants";
import {
  getIngestionTypeSupport,
  PIPELINE_CANVAS_INGESTION_TYPES,
} from "@/pages/pipelines/canvas/graph/capabilities";
import {
  usePipelineCanvasActions,
  usePipelineCanvasReadOnly,
  usePipelineCanvasState,
} from "@/pages/pipelines/canvas/providers/canvas/PipelineCanvasProvider";
import { isConnectionNode } from "@/pages/pipelines/canvas/types";

const LabelWrapper = styled.div<{ $labelX: number; $labelY: number }>`
  position: absolute;
  transform: translate(-50%, -50%) translate(${({ $labelX }) => $labelX}px, ${({ $labelY }) => $labelY}px);
  pointer-events: all;
  z-index: ${PIPELINE_CANVAS_OVERLAY_Z_INDEX};
`;

const SelectWrapper = withTheme(styled.div<PropsWithTheme<{ $isSelected?: boolean }>>`
  background-color: ${({ theme }) => theme.color.background.base};
  border: ${({ theme, $isSelected }) =>
    $isSelected
      ? `2px solid ${theme.color.background.galaxy}`
      : `0.5px solid ${theme.color.border.primary}`};
 
  border-radius: 6px;
`);

interface PipelineCanvasEdgeModeLabelProps {
  edgeId: string;
  sourceNodeId: string;
  targetNodeId: string;
  ingestionType: IngestionType;
  isSelected: boolean;
  labelX: number;
  labelY: number;
}

const PipelineCanvasEdgeModeLabel = ({
  edgeId,
  sourceNodeId,
  targetNodeId,
  ingestionType,
  isSelected,
  labelX,
  labelY,
}: PipelineCanvasEdgeModeLabelProps) => {
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
    const support = getIngestionTypeSupport(sourceSpec, sinkSpec);
    return PIPELINE_CANVAS_INGESTION_TYPES.filter(
      (type) => support[type].isSupported || type === ingestionType,
    ).map((type) => ({
      id: String(type),
      label: INGESTION_TYPE_TO_LABEL_MAP[type],
      value: type,
    }));
  }, [sourceSpec, sinkSpec, ingestionType]);

  const selectedOption = options.find((option) => option.value === ingestionType) ?? null;

  return (
    <LabelWrapper className="nodrag nopan" $labelX={labelX} $labelY={labelY}>
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

export default PipelineCanvasEdgeModeLabel;
