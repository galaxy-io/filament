import { styled } from "@linaria/react";
import { Link } from "@tanstack/react-router";

import Flex, { AlignItems } from "@galaxy-io/dls/layout/Flex";
import { t } from "@galaxy-io/dls/theme/tokens/t";

import type { Pipeline } from "@/gen/ingestion/v1/pipelines_pb";

import PipelineName from "@/components/PipelineName";

import { PIPELINE_CARD_HEIGHT } from "@/pages/pipelines/components/card/constants";
import PipelineFlow, { PipelineFlowSize } from "@/pages/pipelines/components/flow/PipelineFlow";
import { usePipelineFlowEndpoints } from "@/pages/pipelines/hooks/usePipelineFlowEndpoints";

import PipelineScheduleChip from "../schedule/PipelineScheduleChip";

const CardLinkWrapper = styled(Link)`
  display: block;
  width: 100%;
  text-decoration: none;
  color: inherit;
`;

const CardWrapper = styled.div`
  width: 100%;
  height: ${PIPELINE_CARD_HEIGHT}px;

  padding: 0 12px;

  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;

  border-bottom: 0.5px solid ${t.color.border.primary};

  cursor: pointer;

  transition: background-color 100ms ease;

  &:hover {
    background-color: ${t.color.background.tertiary};
  }

  ${CardLinkWrapper}:last-child > & {
    border-bottom: none;
  }
`;

interface PipelineCardProps {
  pipeline: Pipeline;
}

const PipelineCard = ({ pipeline }: PipelineCardProps) => {
  const { source, sinks, hasEdges, isLoading } = usePipelineFlowEndpoints(pipeline.id);

  return (
    <CardLinkWrapper to="/pipelines/$id" params={{ id: pipeline.id }}>
      <CardWrapper>
        <Flex alignItems={AlignItems.CENTER} gap={8}>
          <PipelineName pipelineId={pipeline.id} pipeline={pipeline} />
          <PipelineScheduleChip pipelineId={pipeline.id} />
        </Flex>
        <Flex alignItems={AlignItems.CENTER} gap={12}>
          <PipelineFlow
            source={source}
            sinks={sinks}
            hasEdges={hasEdges}
            size={PipelineFlowSize.SMALL}
            isLoading={isLoading}
          />
        </Flex>
      </CardWrapper>
    </CardLinkWrapper>
  );
};

export default PipelineCard;
