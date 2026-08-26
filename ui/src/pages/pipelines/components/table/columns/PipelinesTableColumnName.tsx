import FlexWrapper, { AlignItems, FlexGap } from "@galaxy-io/dls/containers/FlexWrapper";

import type { Pipeline } from "@/gen/ingestion/v1/pipelines_pb";

import PipelineName from "@/components/PipelineName";

interface PipelinesTableColumnNameProps {
  pipeline: Pipeline;
}

const PipelinesTableColumnName = ({ pipeline }: PipelinesTableColumnNameProps) => {
  return (
    <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.SMALL}>
      <PipelineName pipelineId={pipeline.id} pipeline={pipeline} />
    </FlexWrapper>
  );
};

export default PipelinesTableColumnName;
