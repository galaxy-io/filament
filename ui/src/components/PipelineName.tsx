import { create } from "@bufbuild/protobuf";
import { TrashIcon } from "@phosphor-icons/react";

import Chip, { ChipSize, ChipVariant } from "@galaxy-io/dls/chips/Chip";
import FlexWrapper, { AlignItems, FlexGap } from "@galaxy-io/dls/containers/FlexWrapper";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import TextShimmer from "@galaxy-io/dls/text/TextShimmer";
import Tooltip from "@galaxy-io/dls/tooltip/Tooltip";

import { GetPipelineRequestSchema, type Pipeline } from "@/gen/ingestion/v1/pipelines_pb";

import { formatPipelineName } from "@/pages/pipelines/utils";

import { useGetPipelineQuery } from "@/api/queries/pipelines";

import { formatTimestamp } from "@/utils/format";

interface PipelineNameProps {
  pipelineId: Pipeline["id"];
  pipeline?: Pipeline;
  size?: TextSize;
}

const PipelineName = ({
  size = TextSize.BODY_MD,
  pipelineId,
  pipeline: initialPipeline,
}: PipelineNameProps) => {
  const { data, isLoading } = useGetPipelineQuery({
    input: create(GetPipelineRequestSchema, {
      id: pipelineId,
    }),
    options: {
      enabled: !initialPipeline,
    },
  });

  const pipeline = data?.pipeline || initialPipeline;

  const renderDeletedChip = () => {
    if (!pipeline?.deletedAt) {
      return null;
    }

    return (
      <Tooltip body={`Deleted on ${formatTimestamp(pipeline.deletedAt)}`}>
        <Chip icon={TrashIcon} label="Deleted" variant={ChipVariant.ERROR} size={ChipSize.SMALL} />
      </Tooltip>
    );
  };

  if (isLoading) {
    return <TextShimmer width={160} height={16} />;
  }

  if (!pipeline?.name) {
    return (
      <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.SMALL}>
        <Text size={size} variant={TextVariant.TERTIARY} isEllipsis>
          Untitled pipeline
        </Text>
        {renderDeletedChip()}
      </FlexWrapper>
    );
  }

  return (
    <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.SMALL}>
      <Text size={size} isEllipsis>
        {formatPipelineName(pipeline)}
      </Text>
      {renderDeletedChip()}
    </FlexWrapper>
  );
};

export default PipelineName;
