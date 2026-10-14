import { type FC, useMemo } from "react";

import { FlowArrowIcon } from "@phosphor-icons/react";

import Skeleton from "@galaxy-io/dls/feedback/Skeleton";
import Box from "@galaxy-io/dls/layout/Box";
import ErrorLayout, { ErrorLayoutSize } from "@galaxy-io/dls/layout/ErrorLayout";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";

import type { Connection } from "@/gen/ingestion/v1/connections_pb";

import PipelineCard from "@/components/pipelines/PipelineCard";

import ConnectionDrawerSection from "@/pages/connections/components/drawer/ConnectionDrawerSection";
import { useConnectionPipelines } from "@/pages/connections/hooks/useConnectionPipelines";

interface ConnectionDrawerPipelinesProps {
  connectionId: Connection["id"];
}

const ConnectionDrawerPipelines: FC<ConnectionDrawerPipelinesProps> = ({ connectionId }) => {
  const { pipelines, connectionIdsByPipelineId, isError, isLoading } = useConnectionPipelines();

  const connectedPipelines = useMemo(
    () =>
      pipelines.filter((pipeline) => connectionIdsByPipelineId.get(pipeline.id)?.has(connectionId)),
    [pipelines, connectionIdsByPipelineId, connectionId],
  );

  const renderContent = () => {
    if (isLoading) {
      return (
        <Box padding={12}>
          <Skeleton lines={2} />
        </Box>
      );
    }

    if (isError) {
      return (
        <Box padding={12}>
          <ErrorLayout
            size={ErrorLayoutSize.SMALL}
            header="Failed to load pipelines"
            description="The pipelines using this connection could not be loaded."
          />
        </Box>
      );
    }

    return (
      <Flex alignItems={AlignItems.STRETCH} fillWidth direction={FlexDirection.COLUMN}>
        {connectedPipelines.map((pipeline) => (
          <PipelineCard key={pipeline.id} pipeline={pipeline} />
        ))}
      </Flex>
    );
  };

  return (
    <ConnectionDrawerSection
      header="Pipelines"
      icon={FlowArrowIcon}
      count={isLoading || isError ? undefined : connectedPipelines.length}
      emptyHeader="No pipelines"
      emptyMessage="This connection is not used in any pipelines."
      isOpenInitial
      isFlush
    >
      {renderContent()}
    </ConnectionDrawerSection>
  );
};

export default ConnectionDrawerPipelines;
