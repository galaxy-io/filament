import { useState } from "react";

import { styled } from "@linaria/react";
import { ArrowUpRightIcon } from "@phosphor-icons/react";
import { useNavigate } from "@tanstack/react-router";

import Beacon from "@galaxy-io/dls/beacons/Beacon";
import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import FlexWrapper, {
  AlignItems,
  FlexGap,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import ToggleInput from "@galaxy-io/dls/inputs/ToggleInput";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import PipelineFlow from "@/pages/pipelines/components/PipelineFlow";
import {
  PIPELINE_CARD_HEIGHT,
  PIPELINE_INDICATOR_WIDTH,
  PIPELINE_METRIC_COLUMN_WIDTH_CONNECTORS,
  PIPELINE_METRIC_COLUMN_WIDTH_LAST_RUN,
  PIPELINE_METRIC_COLUMN_WIDTH_SCHEDULE,
  PIPELINE_METRIC_COLUMN_WIDTH_VOLUME,
} from "@/pages/pipelines/constants";
import { PipelineHealth } from "@/pages/pipelines/types";
import { getHealthBeaconVariant } from "@/pages/pipelines/utils";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import type { Pipeline } from "@/gen/ingestion/v1/pipelines_pb";

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
  pipeline: Pipeline;
}

interface PipelineCardState {
  isEnabled: boolean;
}

const DEFAULT_STATE: PipelineCardState = {
  isEnabled: false,
};

const PipelineCard = ({ pipeline }: PipelineCardProps) => {
  const navigate = useNavigate();

  const [state, setState] = useState<PipelineCardState>(DEFAULT_STATE);

  const handleIsEnabledChange = (isEnabled: boolean) => {
    setState((prev) => ({ ...prev, isEnabled }));
  };

  const handlePipelineClick = () => {
    navigate({
      to: "/pipelines/$id",
      params: { id: pipeline.id },
    });
  };

  return (
    <CardWrapper onClick={handlePipelineClick}>
      <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.MEDIUM}>
        <FlexWrapper
          alignItems={AlignItems.CENTER}
          justifyContent={JustifyContent.CENTER}
          width={PIPELINE_INDICATOR_WIDTH}
        >
          <Beacon variant={getHealthBeaconVariant(PipelineHealth.HEALTHY)} />
        </FlexWrapper>
        <Text weight={TextWeight.MEDIUM}>{pipeline.name}</Text>
      </FlexWrapper>

      <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.XLARGE}>
        <MetricColumnWrapper $width={PIPELINE_METRIC_COLUMN_WIDTH_CONNECTORS}>
          <PipelineFlow
            source={pipeline.nodes.find((n) => n.kind === ConnectorKind.SOURCE)?.connectionId ?? ""}
            sinks={pipeline.nodes
              .filter((n) => n.kind === ConnectorKind.SINK)
              .map((n) => n.connectionId)}
          />
        </MetricColumnWrapper>
        <MetricColumn width={PIPELINE_METRIC_COLUMN_WIDTH_LAST_RUN} label="Last run" value="—" />
        <MetricColumn width={PIPELINE_METRIC_COLUMN_WIDTH_VOLUME} label="Volume" value="—" />
        <MetricColumn width={PIPELINE_METRIC_COLUMN_WIDTH_SCHEDULE} value="—" />
        <ToggleInput value={state.isEnabled} onChange={handleIsEnabledChange} />
        <Button
          variant={ButtonVariant.TERTIARY}
          size={ButtonSize.SMALL}
          icon={ArrowUpRightIcon}
          onClick={handlePipelineClick}
        />
      </FlexWrapper>
    </CardWrapper>
  );
};

export default PipelineCard;
