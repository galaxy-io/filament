import type { FC } from "react";

import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";

import { ConnectorKind, ExecutionMode } from "@/gen/ingestion/v1/common_pb";

import { useCreatePipelineModalState } from "@/pages/pipelines/components/create/CreatePipelineModalProvider";
import CreatePipelineModalDeliveryConnection from "@/pages/pipelines/components/create/steps/components/CreatePipelineModalDeliveryConnection";
import CreatePipelineModalDeliveryNotifications from "@/pages/pipelines/components/create/steps/components/CreatePipelineModalDeliveryNotifications";
import CreatePipelineModalDeliverySchedule from "@/pages/pipelines/components/create/steps/components/CreatePipelineModalDeliverySchedule";
import CreatePipelineModalDeliveryWorker from "@/pages/pipelines/components/create/steps/components/CreatePipelineModalDeliveryWorker";

const CreatePipelineModalDelivery: FC = () => {
  const { sourceConnection, sinks, executionMode } = useCreatePipelineModalState();

  return (
    <Flex alignItems={AlignItems.STRETCH} direction={FlexDirection.COLUMN} gap={12} fillWidth>
      {sourceConnection && (
        <CreatePipelineModalDeliveryConnection
          connection={sourceConnection}
          kind={ConnectorKind.SOURCE}
        />
      )}
      {sinks.map((sink) => (
        <CreatePipelineModalDeliveryConnection
          key={sink.connection.id}
          connection={sink.connection}
          kind={ConnectorKind.SINK}
          sink={sink}
        />
      ))}
      {executionMode !== ExecutionMode.CONTINUOUS && <CreatePipelineModalDeliverySchedule />}
      <CreatePipelineModalDeliveryNotifications />
      <CreatePipelineModalDeliveryWorker />
    </Flex>
  );
};

export default CreatePipelineModalDelivery;
