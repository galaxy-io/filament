import FlexWrapper, {
  AlignItems,
  FlexGap,
} from "@galaxy-io/dls/containers/FlexWrapper";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import { FlowArrowIcon } from "@phosphor-icons/react";

import { PIPELINE_MAX_VISIBLE_SINKS } from "@/pages/pipelines/constants";
import { PipelineFlowSize } from "@/pages/pipelines/types";
import ConnectorTile, {
  ConnectorOverflowTile,
  ConnectorTileSize,
} from "@/pages/connectors/components/ConnectorTile";

const PIPELINE_FLOW_SIZE_TO_CONNECTOR_TILE_SIZE_MAP: Record<
  PipelineFlowSize,
  ConnectorTileSize
> = {
  [PipelineFlowSize.SMALL]: ConnectorTileSize.SMALL,
  [PipelineFlowSize.MEDIUM]: ConnectorTileSize.MEDIUM,
};

const PIPELINE_FLOW_SIZE_TO_ICON_SIZE_MAP: Record<PipelineFlowSize, number> = {
  [PipelineFlowSize.SMALL]: 12,
  [PipelineFlowSize.MEDIUM]: 16,
};

interface PipelineFlowProps {
  source: string;
  sinks: string[];
  size?: PipelineFlowSize;
  maxSinks?: number;
  onConnectionClick?: (connectionId: string, e: React.MouseEvent) => void;
}

/**
 * Source tile → sink tiles, optionally truncated with a "+N" overflow tile.
 */
const PipelineFlow = ({
  source,
  sinks,
  size = PipelineFlowSize.SMALL,
  maxSinks,
  onConnectionClick,
}: PipelineFlowProps) => {
  const limit = maxSinks ?? PIPELINE_MAX_VISIBLE_SINKS;
  const visibleSinks = maxSinks === undefined ? sinks : sinks.slice(0, limit);
  const overflowCount = sinks.length - visibleSinks.length;

  const tileSize = PIPELINE_FLOW_SIZE_TO_CONNECTOR_TILE_SIZE_MAP[size];
  const iconSize = PIPELINE_FLOW_SIZE_TO_ICON_SIZE_MAP[size];

  return (
    <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.SMALL}>
      <ConnectorTile
        connector={source}
        size={tileSize}
        onClick={onConnectionClick ? (e) => onConnectionClick(source, e) : undefined}
      />
      <Icon
        component={FlowArrowIcon}
        size={iconSize}
        variant={IconVariant.TERTIARY}
      />
      <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.XSMALL}>
        {visibleSinks.map((sink, index) => (
          <ConnectorTile
            key={`${sink}-${index}`}
            connector={sink}
            size={tileSize}
            onClick={onConnectionClick ? (e) => onConnectionClick(sink, e) : undefined}
          />
        ))}
        {overflowCount > 0 && <ConnectorOverflowTile count={overflowCount} />}
      </FlexWrapper>
    </FlexWrapper>
  );
};

export { PipelineFlowSize } from "@/pages/pipelines/types";
export default PipelineFlow;
