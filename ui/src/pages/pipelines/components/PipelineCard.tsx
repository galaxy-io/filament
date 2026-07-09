import { useState } from "react";

import { ArrowUpRightIcon } from "@phosphor-icons/react";
import { styled } from "@linaria/react";

import Beacon, { BeaconSize } from "@galaxy-io/dls/beacons/Beacon";
import FlexWrapper, { AlignItems, FlexGap } from "@galaxy-io/dls/containers/FlexWrapper";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import ToggleInput from "@galaxy-io/dls/inputs/ToggleInput";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import ProviderFlow from "@/pages/pipelines/components/ProviderFlow";
import { PipelineListItem } from "@/pages/pipelines/types";
import { getHealthBeaconVariant } from "@/pages/pipelines/utils";

const CARD_HEIGHT = 48;

const METRIC_COLUMN_WIDTHS = {
  providers: 140,
  lastRun: 110,
  volume: 120,
  schedule: 60,
} as const;

const CardWrapper = withTheme(styled.div<PropsWithTheme>`
  width: 100%;
  height: ${CARD_HEIGHT}px;

  padding: 0 16px;

  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 24px;

  border-bottom: 0.5px solid ${({ theme }) => theme.color.border.primary};

  transition: background-color 100ms ease;

  &:last-of-type {
    border-bottom: 0;
  }

  &:hover {
    background-color: ${({ theme }) => theme.color.background.primaryAlt};
  }
`);

const MetricColumnWrapper = styled.div<{ $width: number }>`
  width: ${({ $width }) => $width}px;

  display: flex;
  align-items: center;
  gap: 8px;
`;

interface MetricColumnProps {
  width: number;
  label?: string;
  value: string;
  valueVariant?: TextVariant;
}

const MetricColumn = ({
  width,
  label,
  value,
  valueVariant = TextVariant.PRIMARY,
}: MetricColumnProps) => {
  return (
    <MetricColumnWrapper $width={width}>
      {label && (
        <Text size={TextSize.CAPTION} variant={TextVariant.SECONDARY}>
          {label}
        </Text>
      )}
      <Text size={TextSize.BODY_SM} variant={valueVariant} isMonospace>
        {value}
      </Text>
    </MetricColumnWrapper>
  );
};

interface PipelineCardProps {
  pipeline: PipelineListItem;
}

const PipelineCard = ({ pipeline }: PipelineCardProps) => {
  // Local-only until SignalRun (pause/resume) is wired to the toggle.
  const [isEnabled, setIsEnabled] = useState(pipeline.isEnabled);

  return (
    <CardWrapper>
      <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.LARGE}>
        <Beacon size={BeaconSize.SMALL} variant={getHealthBeaconVariant(pipeline.health)} />
        <Text size={TextSize.BODY_MD} weight={TextWeight.MEDIUM}>
          {pipeline.name}
        </Text>
      </FlexWrapper>
      <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.XLARGE}>
        <MetricColumnWrapper $width={METRIC_COLUMN_WIDTHS.providers}>
          <ProviderFlow source={pipeline.source} sinks={pipeline.sinks} />
        </MetricColumnWrapper>
        <MetricColumn
          width={METRIC_COLUMN_WIDTHS.lastRun}
          label="Last run"
          value={pipeline.lastRunLabel}
        />
        <MetricColumn
          width={METRIC_COLUMN_WIDTHS.volume}
          label="Volume"
          value={pipeline.volumeLabel}
        />
        <MetricColumn
          width={METRIC_COLUMN_WIDTHS.schedule}
          value={pipeline.scheduleLabel}
          valueVariant={TextVariant.SECONDARY}
        />
        <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.XLARGE}>
          <ToggleInput value={isEnabled} onChange={setIsEnabled} />
          <Icon component={ArrowUpRightIcon} size={12} variant={IconVariant.SECONDARY} />
        </FlexWrapper>
      </FlexWrapper>
    </CardWrapper>
  );
};

export default PipelineCard;
