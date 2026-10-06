import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";

import { ExecutionMode } from "@/gen/ingestion/v1/common_pb";

import { useCreatePipelineModalState } from "@/pages/pipelines/components/create/CreatePipelineModalProvider";
import CreatePipelineModalDeliveryAdvanced from "@/pages/pipelines/components/create/steps/components/CreatePipelineModalDeliveryAdvanced";
import CreatePipelineModalDeliveryDestinations from "@/pages/pipelines/components/create/steps/components/CreatePipelineModalDeliveryDestinations";
import CreatePipelineModalDeliveryNotifications from "@/pages/pipelines/components/create/steps/components/CreatePipelineModalDeliveryNotifications";
import CreatePipelineModalDeliverySchedule from "@/pages/pipelines/components/create/steps/components/CreatePipelineModalDeliverySchedule";
import CreatePipelineModalDeliverySection from "@/pages/pipelines/components/create/steps/components/CreatePipelineModalDeliverySection";

const CreatePipelineModalDelivery = () => {
  const { sinks, executionMode } = useCreatePipelineModalState();

  return (
    <Flex alignItems={AlignItems.STRETCH} direction={FlexDirection.COLUMN} gap={16} fillWidth>
      {sinks.length > 0 && (
        <CreatePipelineModalDeliverySection header="Destinations">
          <CreatePipelineModalDeliveryDestinations />
        </CreatePipelineModalDeliverySection>
      )}
      {executionMode !== ExecutionMode.CONTINUOUS && <CreatePipelineModalDeliverySchedule />}
      <CreatePipelineModalDeliveryNotifications />
      <CreatePipelineModalDeliverySection header="Advanced">
        <CreatePipelineModalDeliveryAdvanced />
      </CreatePipelineModalDeliverySection>
    </Flex>
  );
};

export default CreatePipelineModalDelivery;
