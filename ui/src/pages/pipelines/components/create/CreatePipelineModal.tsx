import { styled } from "@linaria/react";
import { match } from "ts-pattern";

import Divider from "@galaxy-io/dls/layout/Divider";
import Flex, { AlignItems } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import { t } from "@galaxy-io/dls/theme/tokens/t";

import BaseHeader from "@/layouts/components/BaseHeader";

import CreatePipelineModalProvider, {
  useCreatePipelineModalState,
} from "@/pages/pipelines/components/create/CreatePipelineModalProvider";
import CreatePipelineModalFooter from "@/pages/pipelines/components/create/components/CreatePipelineModalFooter";
import CreatePipelineModalSidebar from "@/pages/pipelines/components/create/components/CreatePipelineModalSidebar";
import {
  CREATE_PIPELINE_MODAL_HEIGHT,
  CREATE_PIPELINE_MODAL_STEP_TO_IS_PADDED_MAP,
  CREATE_PIPELINE_MODAL_STEP_TO_TITLE_MAP,
  CREATE_PIPELINE_MODAL_WIDTH,
} from "@/pages/pipelines/components/create/constants";
import CreatePipelineModalConnections from "@/pages/pipelines/components/create/steps/CreatePipelineModalConnections";
import CreatePipelineModalDelivery from "@/pages/pipelines/components/create/steps/CreatePipelineModalDelivery";
import CreatePipelineModalDetails from "@/pages/pipelines/components/create/steps/CreatePipelineModalDetails";
import CreatePipelineModalResources from "@/pages/pipelines/components/create/steps/CreatePipelineModalResources";
import { CreatePipelineModalStep } from "@/pages/pipelines/components/create/types";

const ModalWrapper = styled.div`
  display: flex;

  height: ${CREATE_PIPELINE_MODAL_HEIGHT}px;
  width: ${CREATE_PIPELINE_MODAL_WIDTH}px;

  background-color: ${t.color.background.primary};
  border: 0.5px solid ${t.color.border.primary};
  border-radius: 8px;
  overflow: hidden;
`;

const MainWrapper = styled.div`
  display: flex;
  flex-direction: column;
  flex: 1;
  min-width: 0;
`;

const BodyWrapper = styled.div<{ $isPadded: boolean }>`
  display: flex;
  flex-direction: column;

  flex: 1;
  min-height: 0;
  width: 100%;
  overflow-y: auto;
  background-color: ${t.color.background.base};
  padding: ${({ $isPadded }) => ($isPadded ? "16px" : "0")};
`;

interface CreatePipelineModalProps {
  onClose: () => void;
}

const CreatePipelineModalContent = ({ onClose }: CreatePipelineModalProps) => {
  const { step } = useCreatePipelineModalState();

  const renderBody = () => {
    return match(step)
      .with(CreatePipelineModalStep.CONNECTIONS, () => <CreatePipelineModalConnections />)
      .with(CreatePipelineModalStep.RESOURCES, () => <CreatePipelineModalResources />)
      .with(CreatePipelineModalStep.DELIVERY, () => <CreatePipelineModalDelivery />)
      .with(CreatePipelineModalStep.DETAILS, () => <CreatePipelineModalDetails />)
      .exhaustive();
  };

  return (
    <ModalWrapper>
      <CreatePipelineModalSidebar />
      <MainWrapper>
        <FlexItem grow={0} shrink={0}>
          <Flex alignItems={AlignItems.START} padding={[12, 16]} fillWidth>
            <BaseHeader title={CREATE_PIPELINE_MODAL_STEP_TO_TITLE_MAP[step]} onClose={onClose} />
          </Flex>
        </FlexItem>
        <FlexItem grow={0} shrink={0}>
          <Divider />
        </FlexItem>

        <BodyWrapper $isPadded={CREATE_PIPELINE_MODAL_STEP_TO_IS_PADDED_MAP[step]}>
          {renderBody()}
        </BodyWrapper>

        <FlexItem grow={0} shrink={0}>
          <Divider />
        </FlexItem>
        <CreatePipelineModalFooter />
      </MainWrapper>
    </ModalWrapper>
  );
};

const CreatePipelineModal = ({ onClose }: CreatePipelineModalProps) => {
  return (
    <CreatePipelineModalProvider>
      <CreatePipelineModalContent onClose={onClose} />
    </CreatePipelineModalProvider>
  );
};

export default CreatePipelineModal;
