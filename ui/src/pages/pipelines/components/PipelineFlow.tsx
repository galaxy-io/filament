import { FlowArrowIcon, LinkBreakIcon } from "@phosphor-icons/react";
import { useNavigate } from "@tanstack/react-router";

import FlexWrapper, { AlignItems, FlexGap } from "@galaxy-io/dls/containers/FlexWrapper";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";

import ConnectorTile, {
  ConnectorOverflowTile,
  ConnectorTileEmpty,
  ConnectorTileSize,
} from "@/pages/connectors/components/ConnectorTile";
import { PIPELINE_MAX_VISIBLE_SINKS } from "@/pages/pipelines/constants";
import { PipelineFlowSize } from "@/pages/pipelines/types";

const PIPELINE_FLOW_SIZE_TO_CONNECTOR_TILE_SIZE_MAP: Record<PipelineFlowSize, ConnectorTileSize> = {
  [PipelineFlowSize.SMALL]: ConnectorTileSize.SMALL,
  [PipelineFlowSize.MEDIUM]: ConnectorTileSize.MEDIUM,
};

const PIPELINE_FLOW_SIZE_TO_ICON_SIZE_MAP: Record<PipelineFlowSize, number> = {
  [PipelineFlowSize.SMALL]: 12,
  [PipelineFlowSize.MEDIUM]: 16,
};

// The tile logo resolves from the connector name, while click-through
// navigation needs the connection id - so flow entries carry both
export interface PipelineFlowConnection {
  connectionId: string;
  connector: string;
}

interface PipelineFlowProps {
  source?: PipelineFlowConnection;
  sinks?: PipelineFlowConnection[];
  // Whether the graph routes at least one source→sink edge; nodes without a
  // route render the broken link even when both ends exist
  hasEdges?: boolean;
  size?: PipelineFlowSize;
  maxSinks?: number;
}

/**
 * Source tile → sink tiles, optionally truncated with a "+N" overflow tile.
 */
const PipelineFlow = ({
  source,
  sinks = [],
  hasEdges = true,
  size = PipelineFlowSize.SMALL,
  maxSinks,
}: PipelineFlowProps) => {
  const navigate = useNavigate();

  const limit = maxSinks ?? PIPELINE_MAX_VISIBLE_SINKS;
  const visibleSinks = maxSinks === undefined ? sinks : sinks.slice(0, limit);
  const overflowCount = sinks.length - visibleSinks.length;

  const tileSize = PIPELINE_FLOW_SIZE_TO_CONNECTOR_TILE_SIZE_MAP[size];
  const iconSize = PIPELINE_FLOW_SIZE_TO_ICON_SIZE_MAP[size];

  const hasSource = !!source && source.connectionId.length > 0;
  const hasSinks = sinks.length > 0;
  const isLinked = hasSource && hasSinks && hasEdges;

  const handleConnectionClick = (connectionId: string, e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    navigate({
      to: ".",
      search: (prev) => ({ ...prev, connectionId }),
    });
  };

  return (
    <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.SMALL}>
      {hasSource ? (
        <ConnectorTile
          connector={source.connector}
          size={tileSize}
          onClick={(e) => handleConnectionClick(source.connectionId, e)}
        />
      ) : (
        <ConnectorTileEmpty size={tileSize} />
      )}
      <Icon
        component={isLinked ? FlowArrowIcon : LinkBreakIcon}
        size={iconSize}
        variant={isLinked ? IconVariant.TERTIARY : IconVariant.ERROR}
      />
      {hasSinks ? (
        <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.XSMALL}>
          {visibleSinks.map((sink, index) => (
            <ConnectorTile
              // biome-ignore lint/suspicious/noArrayIndexKey: a pipeline can have two sink nodes on the same connection, so id alone isn't guaranteed unique
              key={`${sink.connectionId}-${index}`}
              connector={sink.connector}
              size={tileSize}
              onClick={(e) => handleConnectionClick(sink.connectionId, e)}
            />
          ))}
          {overflowCount > 0 && <ConnectorOverflowTile count={overflowCount} />}
        </FlexWrapper>
      ) : (
        <ConnectorTileEmpty size={tileSize} />
      )}
    </FlexWrapper>
  );
};

export { PipelineFlowSize } from "@/pages/pipelines/types";
export default PipelineFlow;
