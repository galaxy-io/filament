import type { FC } from "react";

import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";

import { ExecutionMode } from "@/gen/ingestion/v1/common_pb";

import { useCreatePipelineModalState } from "@/pages/pipelines/components/create/CreatePipelineModalProvider";
import CreatePipelineModalDeliveryAdvanced from "@/pages/pipelines/components/create/steps/components/CreatePipelineModalDeliveryAdvanced";
import CreatePipelineModalDeliveryDestinations from "@/pages/pipelines/components/create/steps/components/CreatePipelineModalDeliveryDestinations";
import CreatePipelineModalDeliveryNotifications from "@/pages/pipelines/components/create/steps/components/CreatePipelineModalDeliveryNotifications";
import CreatePipelineModalDeliverySchedule from "@/pages/pipelines/components/create/steps/components/CreatePipelineModalDeliverySchedule";

const CreatePipelineModalDelivery: FC = () => {
  const { sinks, executionMode } = useCreatePipelineModalState();

  return (
    <Flex alignItems={AlignItems.STRETCH} direction={FlexDirection.COLUMN} gap={12} fillWidth>
      {sinks.length > 0 && <CreatePipelineModalDeliveryDestinations />}
      {executionMode !== ExecutionMode.CONTINUOUS && <CreatePipelineModalDeliverySchedule />}
      <CreatePipelineModalDeliveryNotifications />
      <CreatePipelineModalDeliveryAdvanced />
    </Flex>
  );
};

export default CreatePipelineModalDelivery;
