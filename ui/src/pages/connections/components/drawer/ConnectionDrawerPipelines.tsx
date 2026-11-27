import { type FC, useMemo } from "react";

import { FlowArrowIcon, PlusIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Skeleton from "@galaxy-io/dls/feedback/Skeleton";
import Box from "@galaxy-io/dls/layout/Box";
import ErrorLayout, { ErrorLayoutSize } from "@galaxy-io/dls/layout/ErrorLayout";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";

import type { Connection } from "@/gen/ingestion/v1/connections_pb";

import PipelineCard from "@/components/pipelines/PipelineCard";

import ConnectionDrawerSection from "@/pages/connections/components/drawer/ConnectionDrawerSection";
import { CONNECTOR_KIND_TO_PIPELINE_SEED_MAP } from "@/pages/connections/constants";
import { useConnectionPipelines } from "@/pages/connections/hooks/useConnectionPipelines";

import { useFilamentFlowOpen } from "@/module/hooks";
import { Flow } from "@/module/types";

interface ConnectionDrawerPipelinesProps {
  connection: Connection;
}

const ConnectionDrawerPipelines: FC<ConnectionDrawerPipelinesProps> = ({ connection }) => {
  const openFlow = useFilamentFlowOpen();
  const { pipelines, connectionIdsByPipelineId, isError, isLoading } = useConnectionPipelines();

  const connectedPipelines = useMemo(
    () =>
      pipelines.filter((pipeline) =>
        connectionIdsByPipelineId.get(pipeline.id)?.has(connection.id),
      ),
    [pipelines, connectionIdsByPipelineId, connection.id],
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
      actions={
        <Button
          icon={PlusIcon}
          ariaLabel="Create pipeline"
          tooltip="Create pipeline"
          variant={ButtonVariant.TERTIARY}
          size={ButtonSize.SMALL}
          isDisabled={!!connection.deletedAt}
          onClick={() =>
            openFlow(
              Flow.CREATE_PIPELINE,
              CONNECTOR_KIND_TO_PIPELINE_SEED_MAP[connection.kind](connection.id),
            )
          }
        />
      }
      isOpenInitial
      isFlush
    >
      {renderContent()}
    </ConnectionDrawerSection>
  );
};

export default ConnectionDrawerPipelines;
