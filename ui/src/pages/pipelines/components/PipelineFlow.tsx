import FlexWrapper, {
  AlignItems,
  FlexGap,
} from "@galaxy-io/dls/containers/FlexWrapper";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import { FlowArrowIcon } from "@phosphor-icons/react";

import { PIPELINE_MAX_VISIBLE_SINKS } from "@/pages/pipelines/constants";
import ProviderTile, {
  ProviderOverflowTile,
  ProviderTileSize,
} from "@/pages/providers/components/ProviderTile";

interface PipelineFlowProps {
  source: string;
  sinks: string[];
}

/**
 * Source tile → sink tiles, truncated with a "+N" overflow tile past two.
 */
const PipelineFlow = ({ source, sinks }: PipelineFlowProps) => {
  const visibleSinks = sinks.slice(0, PIPELINE_MAX_VISIBLE_SINKS);
  const overflowCount = sinks.length - visibleSinks.length;

  return (
    <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.SMALL}>
      <ProviderTile provider={source} size={ProviderTileSize.SMALL} />
      <Icon
        component={FlowArrowIcon}
        size={12}
        variant={IconVariant.TERTIARY}
      />
      <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.XSMALL}>
        {visibleSinks.map((sink, index) => (
          <ProviderTile
            key={`${sink}-${index}`}
            provider={sink}
            size={ProviderTileSize.SMALL}
          />
        ))}
        {overflowCount > 0 && <ProviderOverflowTile count={overflowCount} />}
      </FlexWrapper>
    </FlexWrapper>
  );
};

export default PipelineFlow;
