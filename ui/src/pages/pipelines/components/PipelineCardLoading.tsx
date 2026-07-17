import { styled } from "@linaria/react";

import FlexWrapper, { AlignItems, FlexGap } from "@galaxy-io/dls/containers/FlexWrapper";
import TextShimmer from "@galaxy-io/dls/text/TextShimmer";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import {
  PIPELINE_CARD_HEIGHT,
  PIPELINE_INDICATOR_WIDTH,
  PIPELINE_METRIC_COLUMN_WIDTH_CONNECTORS,
  PIPELINE_METRIC_COLUMN_WIDTH_LAST_RUN,
  PIPELINE_METRIC_COLUMN_WIDTH_SCHEDULE,
  PIPELINE_METRIC_COLUMN_WIDTH_VOLUME,
} from "@/pages/pipelines/constants";

const CardWrapper = withTheme(styled.div<PropsWithTheme>`
  width: 100%;
  height: ${PIPELINE_CARD_HEIGHT}px;

  padding: 0 16px;

  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 24px;

  border-bottom: 0.5px solid ${({ theme }) => theme.color.border.primary};
`);

const IndicatorWrapper = styled.div`
  width: ${PIPELINE_INDICATOR_WIDTH}px;

  display: flex;
  align-items: center;
  justify-content: center;
`;

const MetricColumnWrapper = styled.div<{ $width: number }>`
  width: ${({ $width }) => $width}px;

  display: flex;
  align-items: center;
  gap: 8px;
`;

const PipelineCardLoading = () => {
  return (
    <CardWrapper>
      <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.MEDIUM}>
        <IndicatorWrapper>
          <TextShimmer width={8} height={8} />
        </IndicatorWrapper>
        <TextShimmer width={160} height={18} />
      </FlexWrapper>

      <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.XLARGE}>
        <MetricColumnWrapper $width={PIPELINE_METRIC_COLUMN_WIDTH_CONNECTORS}>
          <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.SMALL}>
            <TextShimmer width={24} height={24} />
            <TextShimmer width={12} height={12} />
            <TextShimmer width={24} height={24} />
            <TextShimmer width={24} height={24} />
          </FlexWrapper>
        </MetricColumnWrapper>

        <MetricColumnWrapper $width={PIPELINE_METRIC_COLUMN_WIDTH_LAST_RUN}>
          <TextShimmer width={50} height={12} />
          <TextShimmer width={56} height={14} />
        </MetricColumnWrapper>

        <MetricColumnWrapper $width={PIPELINE_METRIC_COLUMN_WIDTH_VOLUME}>
          <TextShimmer width={46} height={12} />
          <TextShimmer width={60} height={14} />
        </MetricColumnWrapper>

        <MetricColumnWrapper $width={PIPELINE_METRIC_COLUMN_WIDTH_SCHEDULE}>
          <TextShimmer width={72} height={14} />
        </MetricColumnWrapper>

        <TextShimmer width={36} height={20} />

        <TextShimmer width={28} height={28} />
      </FlexWrapper>
    </CardWrapper>
  );
};

export default PipelineCardLoading;
