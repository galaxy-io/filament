import { styled } from "@linaria/react";
import { Link } from "@tanstack/react-router";

import Beacon, { BeaconVariant } from "@galaxy-io/dls/beacons/Beacon";
import FlexWrapper, {
  AlignItems,
  FlexGap,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import type { Pipeline } from "@/gen/ingestion/v1/pipelines_pb";

import PipelineName from "@/components/PipelineName";

import {
  PIPELINE_CARD_HEIGHT,
  PIPELINE_INDICATOR_WIDTH,
} from "@/pages/pipelines/components/card/constants";
import PipelineFlow, { PipelineFlowSize } from "@/pages/pipelines/components/flow/PipelineFlow";
import { usePipelineFlowEndpoints } from "@/pages/pipelines/hooks/usePipelineFlowEndpoints";
import PipelineScheduleChip from "../schedule/PipelineScheduleChip";

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
  const { source, sinks, hasEdges } = usePipelineFlowEndpoints(pipeline.id);

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
          <PipelineName pipelineId={pipeline.id} pipeline={pipeline} />
          <PipelineScheduleChip pipelineId={pipeline.id} />
        </FlexWrapper>
        <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.MEDIUM}>
          <PipelineFlow
            source={source}
            sinks={sinks}
            hasEdges={hasEdges}
            size={PipelineFlowSize.SMALL}
          />
        </FlexWrapper>
      </CardWrapper>
    </CardLinkWrapper>
  );
};

export default PipelineCard;
