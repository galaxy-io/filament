import FlexWrapper, { AlignItems, FlexGap } from "@galaxy-io/dls/containers/FlexWrapper";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";

import ProviderTile, { ProviderOverflowTile } from "@/pages/pipelines/components/ProviderTile";

const MAX_VISIBLE_SINKS = 2;

interface ProviderFlowProps {
  source: string;
  sinks: string[];
}

/**
 * Source tile → sink tiles, truncated with a "+N" overflow tile past two.
 */
const ProviderFlow = ({ source, sinks }: ProviderFlowProps) => {
  const visibleSinks = sinks.slice(0, MAX_VISIBLE_SINKS);
  const overflowCount = sinks.length - visibleSinks.length;

  return (
    <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.SMALL}>
      <ProviderTile provider={source} />
      <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY_ALT} isMonospace>
        →
      </Text>
      <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.XSMALL}>
        {visibleSinks.map((sink, index) => (
          <ProviderTile key={`${sink}-${index}`} provider={sink} />
        ))}
        {overflowCount > 0 && <ProviderOverflowTile count={overflowCount} />}
      </FlexWrapper>
    </FlexWrapper>
  );
};

export default ProviderFlow;
