import { type FC, useState } from "react";

import { match } from "ts-pattern";

import Box, { BoxVariant } from "@galaxy-io/dls/layout/Box";
import Flex, { FlexDirection } from "@galaxy-io/dls/layout/Flex";
import ScrollArea from "@galaxy-io/dls/layout/ScrollArea";
import Modal from "@galaxy-io/dls/modal/Modal";

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
import CreatePipelineModalResourcesColumnsPane from "@/pages/pipelines/components/create/steps/components/CreatePipelineModalResourcesColumnsPane";
import {
  type CreatePipelineModalResourceRow,
  CreatePipelineModalStep,
} from "@/pages/pipelines/components/create/types";

import { useOverlaySession } from "@/hooks/useOverlaySession";

interface CreatePipelineModalProps {
  isOpen: boolean;
  onClose: () => void;
}

interface CreatePipelineModalContentState {
  activeResource?: CreatePipelineModalResourceRow["name"];
}

const DEFAULT_CONTENT_STATE: CreatePipelineModalContentState = {};

const CreatePipelineModalContent: FC<CreatePipelineModalProps> = ({ isOpen, onClose }) => {
  const { step, rowsBySink, activeSinkId } = useCreatePipelineModalState();
  const [localState, setLocalState] =
    useState<CreatePipelineModalContentState>(DEFAULT_CONTENT_STATE);

  const activeRow =
    step === CreatePipelineModalStep.RESOURCES
      ? rowsBySink[activeSinkId]?.find(
          (row) => row.name === localState.activeResource && row.isColumnSelectable,
        )
      : undefined;

  const handleActiveResourceChange = (activeResource?: CreatePipelineModalResourceRow["name"]) => {
    setLocalState((prev) => ({ ...prev, activeResource }));
  };

  const renderBody = () => {
    const body = match(step)
      .with(CreatePipelineModalStep.CONNECTIONS, () => <CreatePipelineModalConnections />)
      .with(CreatePipelineModalStep.RESOURCES, () => (
        <CreatePipelineModalResources
          activeRow={activeRow}
          onActiveResourceChange={handleActiveResourceChange}
        />
      ))
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
      isOpen={isOpen}
      isFullscreen
      header="New pipeline"
      sidebar={activeRow ? null : <CreatePipelineModalSidebar />}
      footer={<CreatePipelineModalFooter />}
      aside={
        activeRow ? (
          <CreatePipelineModalResourcesColumnsPane
            key={activeRow.name}
            row={activeRow}
            onClose={() => handleActiveResourceChange(undefined)}
          />
        ) : null
      }
      hasDividers
      isFlush
      onOpenChange={(isOpen) => {
        if (!isOpen) onClose();
      }}
    >
      <Box variant={BoxVariant.BASE} height="100%">
        {renderBody()}
      </Box>
    </Modal>
  );
};

const CreatePipelineModal: FC<CreatePipelineModalProps> = ({ isOpen, onClose }) => {
  const session = useOverlaySession(isOpen);

  if (!session) {
    return null;
  }

  return (
    <CreatePipelineModalProvider key={session} isOpen={isOpen}>
      <CreatePipelineModalContent isOpen={isOpen} onClose={onClose} />
    </CreatePipelineModalProvider>
  );
};

export default CreatePipelineModal;
