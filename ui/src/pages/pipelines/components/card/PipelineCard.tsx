import { create } from "@bufbuild/protobuf";
import { styled } from "@linaria/react";
import { Link } from "@tanstack/react-router";

import Beacon, { BeaconVariant } from "@galaxy-io/dls/beacons/Beacon";
import FlexWrapper, {
  AlignItems,
  FlexGap,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import Text, { TextSize, TextWeight } from "@galaxy-io/dls/text/Text";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import { ListConnectionsRequestSchema } from "@/gen/ingestion/v1/connections_pb";
import { GetPipelineVersionRequestSchema, type Pipeline } from "@/gen/ingestion/v1/pipelines_pb";

import {
  PIPELINE_CARD_HEIGHT,
  PIPELINE_INDICATOR_WIDTH,
} from "@/pages/pipelines/components/card/constants";
import PipelineFlow from "@/pages/pipelines/components/flow/PipelineFlow";
import { mapVersionNodesToFlowEndpoints } from "@/pages/pipelines/components/flow/utils";
import { formatPipelineName } from "@/pages/pipelines/utils";

import { useListConnectionsQuery } from "@/api/queries/connections";
import { useGetPipelineVersionQuery } from "@/api/queries/pipeline_versions";

const CardLinkWrapper = styled(Link)`
  display: block;
  width: 100%;
  text-decoration: none;
  color: inherit;
`;

const CardWrapper = withTheme(styled.div<PropsWithTheme>`
  width: 100%;
  height: ${PIPELINE_CARD_HEIGHT}px;

  padding: 0 12px;

  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;

  border-bottom: 0.5px solid ${({ theme }) => theme.color.border.primary};

  cursor: pointer;

  transition: background-color 100ms ease;

  &:hover {
    background-color: ${({ theme }) => theme.color.background.tertiary};
  }

  ${CardLinkWrapper}:last-child > & {
    border-bottom: none;
  }
`);

interface PipelineCardProps {
  pipeline: Pipeline;
}

const PipelineCard = ({ pipeline }: PipelineCardProps) => {
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

  return (
    <CardLinkWrapper to="/pipelines/$id" params={{ id: pipeline.id }}>
      <CardWrapper>
        <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.SMALL}>
          <FlexWrapper
            alignItems={AlignItems.CENTER}
            justifyContent={JustifyContent.CENTER}
            width={PIPELINE_INDICATOR_WIDTH}
          >
            <Beacon variant={BeaconVariant.SUCCESS} />
          </FlexWrapper>
          <Text size={TextSize.BODY_SM} weight={TextWeight.MEDIUM}>
            {formatPipelineName(pipeline)}
          </Text>
        </FlexWrapper>

        <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.MEDIUM}>
          <PipelineFlow source={source} sinks={sinks} hasEdges={hasEdges} />
        </FlexWrapper>
      </CardWrapper>
    </CardLinkWrapper>
  );
};

export default PipelineCard;
