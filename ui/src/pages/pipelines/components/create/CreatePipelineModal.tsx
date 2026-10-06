import { styled } from "@linaria/react";
import { match } from "ts-pattern";

import Modal, { ModalSize } from "@galaxy-io/dls/modal/Modal";
import { t } from "@galaxy-io/dls/theme/tokens/t";

import CreatePipelineModalProvider, {
  useCreatePipelineModalState,
} from "@/pages/pipelines/components/create/CreatePipelineModalProvider";
import CreatePipelineModalFooter from "@/pages/pipelines/components/create/components/CreatePipelineModalFooter";
import CreatePipelineModalSidebar from "@/pages/pipelines/components/create/components/CreatePipelineModalSidebar";
import { CREATE_PIPELINE_MODAL_STEP_TO_IS_PADDED_MAP } from "@/pages/pipelines/components/create/constants";
import CreatePipelineModalConnections from "@/pages/pipelines/components/create/steps/CreatePipelineModalConnections";
import CreatePipelineModalDelivery from "@/pages/pipelines/components/create/steps/CreatePipelineModalDelivery";
import CreatePipelineModalDetails from "@/pages/pipelines/components/create/steps/CreatePipelineModalDetails";
import CreatePipelineModalResources from "@/pages/pipelines/components/create/steps/CreatePipelineModalResources";
import { CreatePipelineModalStep } from "@/pages/pipelines/components/create/types";

const FrameWrapper = styled.div`
  display: flex;
  height: 100%;

  background-color: ${t.color.background.primary};
  border: 0.5px solid ${t.color.border.primary};
  border-radius: ${t.radius.lg};
  overflow: hidden;
`;

const BodyWrapper = styled.div<{ $isPadded: boolean }>`
  display: flex;
  flex-direction: column;

  flex: 1;
  min-width: 0;
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
    <Modal
      isOpen
      size={ModalSize.X_LARGE}
      header="Create a new pipeline"
      footer={<CreatePipelineModalFooter />}
      onOpenChange={(isOpen) => {
        if (!isOpen) onClose();
      }}
    >
      <FrameWrapper>
        <CreatePipelineModalSidebar />
        <BodyWrapper $isPadded={CREATE_PIPELINE_MODAL_STEP_TO_IS_PADDED_MAP[step]}>
          {renderBody()}
        </BodyWrapper>
      </FrameWrapper>
    </Modal>
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
