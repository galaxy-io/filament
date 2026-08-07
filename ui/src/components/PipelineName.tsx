import { create } from "@bufbuild/protobuf";

import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import TextShimmer from "@galaxy-io/dls/text/TextShimmer";

import { GetPipelineRequestSchema } from "@/gen/ingestion/v1/pipelines_pb";

import { useGetPipelineQuery } from "@/api/queries/pipelines";

interface PipelineNameProps {
  size?: TextSize;
  pipelineId: string;
}

const PipelineName = ({ size = TextSize.BODY_MD, pipelineId }: PipelineNameProps) => {
  const { data, isLoading } = useGetPipelineQuery({
    input: create(GetPipelineRequestSchema, {
      id: pipelineId,
    }),
  });

  if (isLoading) {
    return <TextShimmer width={160} height={16} />;
  }

  if (!data?.pipeline?.name) {
    return (
      <Text size={size} variant={TextVariant.TERTIARY} isEllipsis>
        {pipelineId}
      </Text>
    );
  }

  return (
    <Text size={size} isEllipsis>
      {data?.pipeline?.name}
    </Text>
  );
};

export default PipelineName;
