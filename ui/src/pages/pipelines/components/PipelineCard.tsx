import { useState } from "react";

import { create } from "@bufbuild/protobuf";
import { styled } from "@linaria/react";
import { ArrowUpRightIcon, InfoIcon } from "@phosphor-icons/react";
import { useNavigate } from "@tanstack/react-router";

import Beacon from "@galaxy-io/dls/beacons/Beacon";
import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import FlexWrapper, {
  AlignItems,
  FlexGap,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import ToggleInput from "@galaxy-io/dls/inputs/ToggleInput";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";
import Tooltip, { TooltipPosition } from "@galaxy-io/dls/tooltip/Tooltip";

import PipelineFlow from "@/pages/pipelines/components/PipelineFlow";
import {
  PIPELINE_CARD_HEIGHT,
  PIPELINE_CARD_HEIGHT_COMPACT,
  PIPELINE_INDICATOR_WIDTH,
  PIPELINE_METRIC_COLUMN_WIDTH_CONNECTORS,
  PIPELINE_METRIC_COLUMN_WIDTH_LAST_RUN,
  PIPELINE_METRIC_COLUMN_WIDTH_SCHEDULE,
  PIPELINE_METRIC_COLUMN_WIDTH_VOLUME,
} from "@/pages/pipelines/constants";
import { PipelineHealth } from "@/pages/pipelines/types";
import { getHealthBeaconVariant } from "@/pages/pipelines/utils";

import { useGetPipelineVersionQuery } from "@/api/queries/pipelines";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import { GetPipelineVersionRequestSchema, type Pipeline } from "@/gen/ingestion/v1/pipelines_pb";

const CardWrapper = withTheme(styled.div<PropsWithTheme<{ $isCompact?: boolean }>>`
  width: 100%;
  height: ${({ $isCompact }) =>
    $isCompact ? PIPELINE_CARD_HEIGHT_COMPACT : PIPELINE_CARD_HEIGHT}px;

  padding: 0 ${({ $isCompact }) => ($isCompact ? "12px" : "16px")};

  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: ${({ $isCompact }) => ($isCompact ? "12px" : "24px")};

  border-bottom: 0.5px solid ${({ theme }) => theme.color.border.primary};

  cursor: pointer;

  transition: background-color 100ms ease;

  &:hover {
    background-color: ${({ theme }) => theme.color.background.tertiary};
  }

  &:last-child {
    border-bottom: ${({ $isCompact, theme }) =>
      $isCompact ? "none" : `0.5px solid ${theme.color.border.primary}`};
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
  isCompact?: boolean;
}

interface PipelineCardState {
  isEnabled: boolean;
}

const DEFAULT_STATE: PipelineCardState = {
  isEnabled: false,
};

const PipelineCard = ({ pipeline, isCompact = false }: PipelineCardProps) => {
  const navigate = useNavigate();

  const [state, setState] = useState<PipelineCardState>(DEFAULT_STATE);

  // The graph lives on the pipeline's current version; NotFound (no saved
  // version yet) renders as an empty flow
  const { data: versionData } = useGetPipelineVersionQuery({
    input: create(GetPipelineVersionRequestSchema, { pipelineId: pipeline.id }),
    options: { retry: false },
  });
  const nodes = versionData?.version?.nodes ?? [];

  const handleIsEnabledChange = (isEnabled: boolean) => {
    setState((prev) => ({ ...prev, isEnabled }));
  };

  const handlePipelineClick = () => {
    navigate({
      to: "/pipelines/$id",
      params: { id: pipeline.id },
    });
  };

  const source = nodes.find((n) => n.kind === ConnectorKind.SOURCE)?.connectionId ?? "";
  const sinks = nodes.filter((n) => n.kind === ConnectorKind.SINK).map((n) => n.connectionId);

  return (
    <CardWrapper $isCompact={isCompact} onClick={handlePipelineClick}>
      <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.MEDIUM}>
        <FlexWrapper
          alignItems={AlignItems.CENTER}
          justifyContent={JustifyContent.CENTER}
          width={PIPELINE_INDICATOR_WIDTH}
        >
          <Beacon variant={getHealthBeaconVariant(PipelineHealth.HEALTHY)} />
        </FlexWrapper>
        <Tooltip
          body={
            <Text size={TextSize.CAPTION} isMonospace isSelectable>
              {pipeline.id}
            </Text>
          }
          position={TooltipPosition.RIGHT}
          isInteractive
        >
          <Icon component={InfoIcon} variant={IconVariant.TERTIARY} size={14} />
        </Tooltip>
        <Text weight={TextWeight.MEDIUM}>{pipeline.name}</Text>
      </FlexWrapper>

      <FlexWrapper alignItems={AlignItems.CENTER} gap={isCompact ? FlexGap.MEDIUM : FlexGap.XLARGE}>
        {isCompact ? (
          <PipelineFlow source={source} sinks={sinks} />
        ) : (
          <MetricColumnWrapper $width={PIPELINE_METRIC_COLUMN_WIDTH_CONNECTORS}>
            <PipelineFlow source={source} sinks={sinks} />
          </MetricColumnWrapper>
        )}
        {!isCompact && (
          <>
            <MetricColumn
              width={PIPELINE_METRIC_COLUMN_WIDTH_LAST_RUN}
              label="Last run"
              value="—"
            />
            <MetricColumn width={PIPELINE_METRIC_COLUMN_WIDTH_VOLUME} label="Volume" value="—" />
            <MetricColumn width={PIPELINE_METRIC_COLUMN_WIDTH_SCHEDULE} value="—" />
            <ToggleInput value={state.isEnabled} onChange={handleIsEnabledChange} />
          </>
        )}
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
