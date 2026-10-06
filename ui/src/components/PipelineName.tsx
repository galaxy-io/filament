import { create } from "@bufbuild/protobuf";
import { TrashIcon } from "@phosphor-icons/react";

import Chip, { ChipSize, ChipVariant } from "@galaxy-io/dls/chips/Chip";
import Skeleton from "@galaxy-io/dls/feedback/Skeleton";
import Box from "@galaxy-io/dls/layout/Box";
import Flex, { AlignItems } from "@galaxy-io/dls/layout/Flex";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
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
    return (
      <Box width={160}>
        <Skeleton />
      </Box>
    );
  }

  if (!pipeline?.name) {
    return (
      <Flex alignItems={AlignItems.CENTER} gap={8}>
        <Text size={size} variant={TextVariant.TERTIARY} lineClamp={1}>
          Untitled pipeline
        </Text>
        {renderDeletedChip()}
      </Flex>
    );
  }

  return (
    <Flex alignItems={AlignItems.CENTER} gap={8}>
      <Text size={size} lineClamp={1}>
        {formatPipelineName(pipeline)}
      </Text>
      {renderDeletedChip()}
    </Flex>
  );
};

export default PipelineName;
