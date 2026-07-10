import FlexWrapper, {
  AlignItems,
  FlexGap,
} from "@galaxy-io/dls/containers/FlexWrapper";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import { FlowArrowIcon } from "@phosphor-icons/react";

import { PIPELINE_MAX_VISIBLE_SINKS } from "@/pages/pipelines/constants";
import { PipelineFlowSize } from "@/pages/pipelines/types";
import ProviderTile, {
  ProviderOverflowTile,
  ProviderTileSize,
} from "@/pages/providers/components/ProviderTile";

const PIPELINE_FLOW_SIZE_TO_PROVIDER_TILE_SIZE_MAP: Record<
  PipelineFlowSize,
  ProviderTileSize
> = {
  [PipelineFlowSize.SMALL]: ProviderTileSize.SMALL,
  [PipelineFlowSize.MEDIUM]: ProviderTileSize.MEDIUM,
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
}

/**
 * Source tile → sink tiles, optionally truncated with a "+N" overflow tile.
 */
const PipelineFlow = ({
  source,
  sinks,
  size = PipelineFlowSize.SMALL,
  maxSinks,
}: PipelineFlowProps) => {
  const limit = maxSinks ?? PIPELINE_MAX_VISIBLE_SINKS;
  const visibleSinks = maxSinks === undefined ? sinks : sinks.slice(0, limit);
  const overflowCount = sinks.length - visibleSinks.length;

  const tileSize = PIPELINE_FLOW_SIZE_TO_PROVIDER_TILE_SIZE_MAP[size];
  const iconSize = PIPELINE_FLOW_SIZE_TO_ICON_SIZE_MAP[size];

  return (
    <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.SMALL}>
      <ProviderTile provider={source} size={tileSize} />
      <Icon
        component={FlowArrowIcon}
        size={iconSize}
        variant={IconVariant.TERTIARY}
      />
      <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.XSMALL}>
        {visibleSinks.map((sink, index) => (
          <ProviderTile
            key={`${sink}-${index}`}
            provider={sink}
            size={tileSize}
          />
        ))}
        {overflowCount > 0 && <ProviderOverflowTile count={overflowCount} />}
      </FlexWrapper>
    </FlexWrapper>
  );
};

export { PipelineFlowSize } from "@/pages/pipelines/types";
export default PipelineFlow;
