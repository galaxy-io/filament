import FlexWrapper, { FlexDirection, FlexGap } from "@galaxy-io/dls/containers/FlexWrapper";

import { ExecutionMode } from "@/gen/ingestion/v1/common_pb";

import { useCreatePipelineModalState } from "@/pages/pipelines/components/create/CreatePipelineModalProvider";
import CreatePipelineModalDeliveryAdvanced from "@/pages/pipelines/components/create/steps/components/CreatePipelineModalDeliveryAdvanced";
import CreatePipelineModalDeliveryDestinations from "@/pages/pipelines/components/create/steps/components/CreatePipelineModalDeliveryDestinations";
import CreatePipelineModalDeliveryExecutionMode from "@/pages/pipelines/components/create/steps/components/CreatePipelineModalDeliveryExecutionMode";
import CreatePipelineModalDeliveryNotifications from "@/pages/pipelines/components/create/steps/components/CreatePipelineModalDeliveryNotifications";
import CreatePipelineModalDeliverySchedule from "@/pages/pipelines/components/create/steps/components/CreatePipelineModalDeliverySchedule";
import CreatePipelineModalDeliverySection from "@/pages/pipelines/components/create/steps/components/CreatePipelineModalDeliverySection";

const CreatePipelineModalDelivery = () => {
  const { sinks, executionMode } = useCreatePipelineModalState();

  return (
    <FlexWrapper direction={FlexDirection.COLUMN} gap={FlexGap.LARGE} fillWidth>
      {sinks.length > 0 && (
        <CreatePipelineModalDeliverySection header="Destinations">
          <CreatePipelineModalDeliveryDestinations />
        </CreatePipelineModalDeliverySection>
      )}
      <CreatePipelineModalDeliverySection header="Execution type">
        <CreatePipelineModalDeliveryExecutionMode />
      </CreatePipelineModalDeliverySection>
      {executionMode !== ExecutionMode.CONTINUOUS && (
        <CreatePipelineModalDeliverySection header="Schedule">
          <CreatePipelineModalDeliverySchedule />
        </CreatePipelineModalDeliverySection>
      )}
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
