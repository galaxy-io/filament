import FlexWrapper, { FlexDirection, FlexGap } from "@galaxy-io/dls/containers/FlexWrapper";

import { useCreatePipelineModalState } from "@/pages/pipelines/components/create/CreatePipelineModalProvider";
import CreatePipelineModalDeliveryAdvanced from "@/pages/pipelines/components/create/steps/components/CreatePipelineModalDeliveryAdvanced";
import CreatePipelineModalDeliveryDestinations from "@/pages/pipelines/components/create/steps/components/CreatePipelineModalDeliveryDestinations";
import CreatePipelineModalDeliveryNotifications from "@/pages/pipelines/components/create/steps/components/CreatePipelineModalDeliveryNotifications";
import CreatePipelineModalDeliverySchedule from "@/pages/pipelines/components/create/steps/components/CreatePipelineModalDeliverySchedule";
import CreatePipelineModalDeliverySection from "@/pages/pipelines/components/create/steps/components/CreatePipelineModalDeliverySection";

const CreatePipelineModalDelivery = () => {
  const { sinks } = useCreatePipelineModalState();

  return (
    <FlexWrapper direction={FlexDirection.COLUMN} gap={FlexGap.LARGE} fillWidth>
      {sinks.length > 0 && (
        <CreatePipelineModalDeliverySection header="Destinations">
          <CreatePipelineModalDeliveryDestinations />
        </CreatePipelineModalDeliverySection>
      )}
      <CreatePipelineModalDeliverySection header="Schedule">
        <CreatePipelineModalDeliverySchedule />
      </CreatePipelineModalDeliverySection>
      <CreatePipelineModalDeliverySection header="Notifications">
        <CreatePipelineModalDeliveryNotifications />
      </CreatePipelineModalDeliverySection>
      <CreatePipelineModalDeliverySection header="Advanced">
        <CreatePipelineModalDeliveryAdvanced />
      </CreatePipelineModalDeliverySection>
    </FlexWrapper>
  );
};

export default CreatePipelineModalDelivery;
