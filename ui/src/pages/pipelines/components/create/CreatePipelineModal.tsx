import { match } from "ts-pattern";

import Box, { BoxVariant } from "@galaxy-io/dls/layout/Box";
import Divider from "@galaxy-io/dls/layout/Divider";
import Flex, { FlexDirection, FlexVariant } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import ScrollArea from "@galaxy-io/dls/layout/ScrollArea";
import Modal, { ModalSize } from "@galaxy-io/dls/modal/Modal";
import { Orientation, Radius } from "@galaxy-io/dls/theme/enums";

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

interface CreatePipelineModalProps {
  onClose: () => void;
}

const CreatePipelineModalContent = ({ onClose }: CreatePipelineModalProps) => {
  const { step } = useCreatePipelineModalState();

  const renderBody = () => {
    const body = match(step)
      .with(CreatePipelineModalStep.CONNECTIONS, () => <CreatePipelineModalConnections />)
      .with(CreatePipelineModalStep.RESOURCES, () => <CreatePipelineModalResources />)
      .with(CreatePipelineModalStep.DELIVERY, () => <CreatePipelineModalDelivery />)
      .with(CreatePipelineModalStep.DETAILS, () => <CreatePipelineModalDetails />)
      .exhaustive();

    if (CREATE_PIPELINE_MODAL_STEP_TO_IS_PADDED_MAP[step]) {
      return (
        <ScrollArea>
          <Box padding={16}>{body}</Box>
        </ScrollArea>
      );
    }

    return (
      <Flex direction={FlexDirection.COLUMN} height="100%">
        {body}
      </Flex>
    );
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
      <Flex
        height="100%"
        variant={FlexVariant.PRIMARY}
        hasBorder
        radius={Radius.LG}
        overflow="hidden"
      >
        <CreatePipelineModalSidebar />
        <Divider orientation={Orientation.VERTICAL} />
        <FlexItem grow={1} basis={0} minWidth={0}>
          <Box variant={BoxVariant.BASE} height="100%">
            {renderBody()}
          </Box>
        </FlexItem>
      </Flex>
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
