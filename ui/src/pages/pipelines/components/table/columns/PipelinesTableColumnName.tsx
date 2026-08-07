import { create } from "@bufbuild/protobuf";

import FlexWrapper, { AlignItems, FlexGap } from "@galaxy-io/dls/containers/FlexWrapper";

import { GetPipelineRequestSchema, type Pipeline } from "@/gen/ingestion/v1/pipelines_pb";

import PipelineName from "@/components/PipelineName";

import PipelineScheduleChip from "@/pages/pipelines/components/schedule/PipelineScheduleChip";

import { useGetPipelineQuery } from "@/api/queries/pipelines";

interface PipelinesTableColumnNameProps {
  pipeline: Pipeline;
}

const PipelinesTableColumnName = ({ pipeline }: PipelinesTableColumnNameProps) => {
  const { data: pipelineData } = useGetPipelineQuery({
    input: create(GetPipelineRequestSchema, { id: pipeline.id }),
  });

  return (
    <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.SMALL}>
      <PipelineName pipelineId={pipeline.id} pipeline={pipeline} />
      <PipelineScheduleChip schedule={pipelineData?.schedule} />
    </FlexWrapper>
  );
};

export default PipelinesTableColumnName;
