import { FlowArrowIcon } from "@phosphor-icons/react";
import { styled } from "@linaria/react";

import Chip, { ChipVariant } from "@galaxy-io/dls/chips/Chip";
import FlexWrapper, {
  AlignItems,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import Text, { TextSize } from "@galaxy-io/dls/text/Text";
import Widget, { WidgetVariant } from "@galaxy-io/dls/widget/Widget";

import { ProviderKind } from "@/gen/ingestion/v1/common_pb";
import type { ProviderSpec } from "@/gen/ingestion/v1/providers_pb";

const MetricItem = styled.div`
  display: flex;
  align-items: center;
  gap: 6px;
`;

interface ProviderDrawerMetricsProps {
  provider: ProviderSpec;
  pipelineCount?: number;
}

const ProviderDrawerMetrics = ({
  provider,
  pipelineCount = 0,
}: ProviderDrawerMetricsProps) => {
  const isSource = provider.kind === ProviderKind.SOURCE;
  const kindLabel = isSource ? "Source" : "Sink";

  return (
    <Widget variant={WidgetVariant.BASE} fillWidth noHover padding="12px">
      <FlexWrapper
        fillWidth
        alignItems={AlignItems.CENTER}
        justifyContent={JustifyContent.SPACE_BETWEEN}
      >
        <MetricItem>
          <Icon
            component={FlowArrowIcon}
            size={16}
            variant={IconVariant.TERTIARY}
          />
          <Text size={TextSize.BODY_SM}>
            {pipelineCount} {pipelineCount === 1 ? "pipeline" : "pipelines"}
          </Text>
        </MetricItem>
        <Chip
          label={kindLabel}
          variant={isSource ? ChipVariant.LIME : ChipVariant.PINK}
        />
      </FlexWrapper>
    </Widget>
  );
};

export default ProviderDrawerMetrics;
