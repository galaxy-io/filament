import { useState } from "react";

import { ArrowUpRightIcon } from "@phosphor-icons/react";
import { styled } from "@linaria/react";
import { Link } from "@tanstack/react-router";

import Beacon from "@galaxy-io/dls/beacons/Beacon";
import FlexWrapper, {
  AlignItems,
  FlexGap,
} from "@galaxy-io/dls/containers/FlexWrapper";
import ToggleInput from "@galaxy-io/dls/inputs/ToggleInput";
import Text, {
  TextSize,
  TextVariant,
  TextWeight,
} from "@galaxy-io/dls/text/Text";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import PipelineFlow from "@/pages/pipelines/components/PipelineFlow";
import {
  PIPELINE_CARD_HEIGHT,
  PIPELINE_INDICATOR_WIDTH,
  PIPELINE_METRIC_COLUMN_WIDTH_MAP,
} from "@/pages/pipelines/constants";
import { PipelineListItem } from "@/pages/pipelines/types";
import { getHealthBeaconVariant } from "@/pages/pipelines/utils";
import { useOpenConnectorDrawer } from "@/pages/connectors/hooks";
import Button, {
  ButtonSize,
  ButtonVariant,
} from "@galaxy-io/dls/buttons/Button";

const IndicatorWrapper = styled.div`
  width: ${PIPELINE_INDICATOR_WIDTH}px;

  display: flex;
  align-items: center;
  justify-content: center;
`;

const CardLink = styled(Link)`
  text-decoration: none;
  color: inherit;
  width: 100%;
`;

const CardWrapper = withTheme(styled.div<PropsWithTheme>`
  width: 100%;
  height: ${PIPELINE_CARD_HEIGHT}px;

  padding: 0 16px;

  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 24px;

  border-bottom: 0.5px solid ${({ theme }) => theme.color.border.primary};

  cursor: pointer;

  transition: background-color 100ms ease;

  &:hover {
    background-color: ${({ theme }) => theme.color.background.tertiary};
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
}

const MetricColumn = ({ width, label, value }: MetricColumnProps) => {
  return (
    <MetricColumnWrapper $width={width}>
      {label && (
        <Text size={TextSize.CAPTION} variant={TextVariant.SECONDARY}>
          {label}
        </Text>
      )}
      <Text size={TextSize.BODY_SM} isMonospace>
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
  const openConnectorDrawer = useOpenConnectorDrawer();

  const handleToggleClick = (e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
  };

  const handleConnectorClick = (connector: string, e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    openConnectorDrawer(connector);
  };

  return (
    <CardLink to="/pipelines/$id" params={{ id: pipeline.id }}>
      <CardWrapper>
        <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.MEDIUM}>
          <IndicatorWrapper>
            <Beacon variant={getHealthBeaconVariant(pipeline.health)} />
          </IndicatorWrapper>
          <Text weight={TextWeight.MEDIUM}>{pipeline.name}</Text>
        </FlexWrapper>
        <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.XLARGE}>
          <MetricColumnWrapper $width={PIPELINE_METRIC_COLUMN_WIDTH_MAP.connectors}>
            <PipelineFlow
              source={pipeline.source}
              sinks={pipeline.sinks}
              onConnectorClick={handleConnectorClick}
            />
          </MetricColumnWrapper>
          <MetricColumn
            width={PIPELINE_METRIC_COLUMN_WIDTH_MAP.lastRun}
            label="Last run"
            value={pipeline.lastRunLabel}
          />
          <MetricColumn
            width={PIPELINE_METRIC_COLUMN_WIDTH_MAP.volume}
            label="Volume"
            value={pipeline.volumeLabel}
          />
          <MetricColumn
            width={PIPELINE_METRIC_COLUMN_WIDTH_MAP.schedule}
            value={pipeline.scheduleLabel}
          />
          <div onClick={handleToggleClick}>
            <ToggleInput value={isEnabled} onChange={setIsEnabled} />
          </div>
          <Button
            variant={ButtonVariant.TERTIARY}
            size={ButtonSize.SMALL}
            icon={ArrowUpRightIcon}
            onClick={handleToggleClick}
          />
        </FlexWrapper>
      </CardWrapper>
    </CardLink>
  );
};

export default PipelineCard;
