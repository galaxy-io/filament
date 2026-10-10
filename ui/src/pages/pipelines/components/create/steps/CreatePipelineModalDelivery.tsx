import type { FC } from "react";

import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";

import { ExecutionMode } from "@/gen/ingestion/v1/common_pb";

import { useCreatePipelineModalState } from "@/pages/pipelines/components/create/CreatePipelineModalProvider";
import CreatePipelineModalDeliveryConnections from "@/pages/pipelines/components/create/steps/components/CreatePipelineModalDeliveryConnections";
import CreatePipelineModalDeliveryNotifications from "@/pages/pipelines/components/create/steps/components/CreatePipelineModalDeliveryNotifications";
import CreatePipelineModalDeliverySchedule from "@/pages/pipelines/components/create/steps/components/CreatePipelineModalDeliverySchedule";
import CreatePipelineModalDeliveryWorker from "@/pages/pipelines/components/create/steps/components/CreatePipelineModalDeliveryWorker";

const CreatePipelineModalDelivery: FC = () => {
  const { executionMode } = useCreatePipelineModalState();

  return (
    <Flex alignItems={AlignItems.STRETCH} direction={FlexDirection.COLUMN} gap={12} fillWidth>
      <CreatePipelineModalDeliveryConnections />
      {executionMode !== ExecutionMode.CONTINUOUS && <CreatePipelineModalDeliverySchedule />}
      <CreatePipelineModalDeliveryNotifications />
      <CreatePipelineModalDeliveryWorker />
    </Flex>
  );
};

export default CreatePipelineModalDelivery;
