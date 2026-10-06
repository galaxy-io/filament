import Flex, { AlignItems } from "@galaxy-io/dls/layout/Flex";

import type { Pipeline } from "@/gen/ingestion/v1/pipelines_pb";

import PipelineName from "@/components/PipelineName";

interface PipelinesTableColumnNameProps {
  pipeline: Pipeline;
}

const PipelinesTableColumnName = ({ pipeline }: PipelinesTableColumnNameProps) => {
  return (
    <Flex alignItems={AlignItems.CENTER} gap={8}>
      <PipelineName pipelineId={pipeline.id} pipeline={pipeline} />
    </Flex>
  );
};

export default PipelinesTableColumnName;
