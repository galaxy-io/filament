import { create } from "@bufbuild/protobuf";
import { styled } from "@linaria/react";
import { ArrowUpRightIcon } from "@phosphor-icons/react";
import { Link } from "@tanstack/react-router";

import Beacon, { BeaconVariant } from "@galaxy-io/dls/beacons/Beacon";
import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import FlexWrapper, {
  AlignItems,
  FlexGap,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import Text, { TextWeight } from "@galaxy-io/dls/text/Text";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import { ListConnectionsRequestSchema } from "@/gen/ingestion/v1/connections_pb";
import { GetPipelineVersionRequestSchema, type Pipeline } from "@/gen/ingestion/v1/pipelines_pb";

import {
  PIPELINE_CARD_HEIGHT,
  PIPELINE_CARD_HEIGHT_COMPACT,
  PIPELINE_INDICATOR_WIDTH,
  PIPELINE_METRIC_COLUMN_WIDTH_CONNECTORS,
  PIPELINE_METRIC_COLUMN_WIDTH_LAST_RUN,
  PIPELINE_METRIC_COLUMN_WIDTH_LAST_VOLUME,
  PIPELINE_METRIC_COLUMN_WIDTH_VERSION,
} from "@/pages/pipelines/components/card/constants";
import PipelineCardMetric from "@/pages/pipelines/components/card/PipelineCardMetric";
import PipelineFlow from "@/pages/pipelines/components/flow/PipelineFlow";
import { mapVersionNodesToFlowEndpoints } from "@/pages/pipelines/components/flow/utils";
import { formatPipelineName } from "@/pages/pipelines/utils";

import { useListConnectionsQuery } from "@/api/queries/connections";
import { useGetPipelineVersionQuery } from "@/api/queries/pipeline_versions";

import { NOOP } from "@/constants";

import { formatBytes, formatTimeAgo } from "@/utils/format";

const CardLinkWrapper = styled(Link)`
  display: block;
  width: 100%;
  text-decoration: none;
  color: inherit;
`;

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

  ${CardLinkWrapper}:last-child > & {
    border-bottom: ${({ $isCompact, theme }) =>
      $isCompact ? "none" : `0.5px solid ${theme.color.border.primary}`};
  }
`);

interface PipelineCardProps {
  pipeline: Pipeline;
  isCompact?: boolean;
}

const PipelineCard = ({ pipeline, isCompact = false }: PipelineCardProps) => {
  const { data: versionData } = useGetPipelineVersionQuery({
    input: create(GetPipelineVersionRequestSchema, { pipelineId: pipeline.id }),
    options: { retry: false },
  });
  const nodes = versionData?.version?.nodes ?? [];
  const hasEdges = (versionData?.version?.edges ?? []).length > 0;

  const { data: connectionsData } = useListConnectionsQuery({
    input: create(ListConnectionsRequestSchema, {}),
  });
  const { source, sinks } = mapVersionNodesToFlowEndpoints(
    nodes,
    connectionsData?.connections ?? [],
  );

  const hasRun = pipeline.lastRunAt > 0n;
  const lastRunLabel = hasRun ? formatTimeAgo(pipeline.lastRunAt) : "—";
  const volumeLabel = hasRun ? formatBytes(pipeline.lastRunBytes) : "—";
  const versionLabel = pipeline.currentVersionId > 0n ? pipeline.currentVersionId.toString() : "—";

  return (
    <CardLinkWrapper to="/pipelines/$id" params={{ id: pipeline.id }}>
      <CardWrapper $isCompact={isCompact}>
        <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.MEDIUM}>
          <FlexWrapper
            alignItems={AlignItems.CENTER}
            justifyContent={JustifyContent.CENTER}
            width={PIPELINE_INDICATOR_WIDTH}
          >
            <Beacon variant={BeaconVariant.SUCCESS} />
          </FlexWrapper>
          <Text weight={TextWeight.MEDIUM}>{formatPipelineName(pipeline)}</Text>
        </FlexWrapper>

        <FlexWrapper
          alignItems={AlignItems.CENTER}
          gap={isCompact ? FlexGap.MEDIUM : FlexGap.XLARGE}
        >
          {isCompact ? (
            <PipelineFlow source={source} sinks={sinks} hasEdges={hasEdges} />
          ) : (
            <>
              <PipelineCardMetric width={PIPELINE_METRIC_COLUMN_WIDTH_CONNECTORS}>
                <PipelineFlow source={source} sinks={sinks} hasEdges={hasEdges} />
              </PipelineCardMetric>
              <PipelineCardMetric
                width={PIPELINE_METRIC_COLUMN_WIDTH_LAST_RUN}
                label="Last run"
                value={lastRunLabel}
              />
              <PipelineCardMetric
                width={PIPELINE_METRIC_COLUMN_WIDTH_LAST_VOLUME}
                label="Last volume"
                value={volumeLabel}
              />
              <PipelineCardMetric
                width={PIPELINE_METRIC_COLUMN_WIDTH_VERSION}
                label="Version"
                value={versionLabel}
              />
            </>
          )}
          <Button
            variant={ButtonVariant.TERTIARY}
            size={ButtonSize.SMALL}
            icon={ArrowUpRightIcon}
            onClick={NOOP}
          />
        </FlexWrapper>
      </CardWrapper>
    </CardLinkWrapper>
  );
};

export default PipelineCard;
