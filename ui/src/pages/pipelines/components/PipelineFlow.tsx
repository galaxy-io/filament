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

interface PipelineFlowProps {
  source?: string;
  sinks?: string[];
  size?: PipelineFlowSize;
  maxSinks?: number;
}

/**
 * Source tile → sink tiles, optionally truncated with a "+N" overflow tile.
 */
const PipelineFlow = ({
  source,
  sinks = [],
  size = PipelineFlowSize.SMALL,
  maxSinks,
}: PipelineFlowProps) => {
  const navigate = useNavigate();

  const limit = maxSinks ?? PIPELINE_MAX_VISIBLE_SINKS;
  const visibleSinks = maxSinks === undefined ? sinks : sinks.slice(0, limit);
  const overflowCount = sinks.length - visibleSinks.length;

  const tileSize = PIPELINE_FLOW_SIZE_TO_CONNECTOR_TILE_SIZE_MAP[size];
  const iconSize = PIPELINE_FLOW_SIZE_TO_ICON_SIZE_MAP[size];

  const hasSource = !!source && source.length > 0;
  const hasSinks = sinks.length > 0;

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
          connector={source}
          size={tileSize}
          onClick={(e) => handleConnectionClick(source, e)}
        />
      ) : (
        <ConnectorTileEmpty size={tileSize} />
      )}
      <Icon
        component={hasSource && hasSinks ? FlowArrowIcon : LinkBreakIcon}
        size={iconSize}
        variant={IconVariant.TERTIARY}
      />
      {hasSinks ? (
        <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.XSMALL}>
          {visibleSinks.map((sink, index) => (
            <ConnectorTile
              // biome-ignore lint/suspicious/noArrayIndexKey: sinks are connector ids and a pipeline can have two sink nodes on the same connector, so id alone isn't guaranteed unique
              key={`${sink}-${index}`}
              connector={sink}
              size={tileSize}
              onClick={(e) => handleConnectionClick(sink, e)}
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
