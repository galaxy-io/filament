import { type FC, useState } from "react";

import { match } from "ts-pattern";

import Box, { BoxVariant } from "@galaxy-io/dls/layout/Box";
import Flex, { FlexDirection } from "@galaxy-io/dls/layout/Flex";
import PendingLayout from "@galaxy-io/dls/layout/PendingLayout";
import ScrollArea from "@galaxy-io/dls/layout/ScrollArea";
import Modal from "@galaxy-io/dls/modal/Modal";

import { PIPELINE_CREATE_TITLE } from "@/components/pipelines/constants";

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
  type CreatePipelineModalSeedSearch,
  CreatePipelineModalStep,
} from "@/pages/pipelines/components/create/types";
import {
  getCreatePipelineModalSeedIds,
  mapConnectionsToCreatePipelineModalSeed,
} from "@/pages/pipelines/components/create/utils";
import { usePipelineResourceSinkFields } from "@/pages/pipelines/components/resource/hooks/usePipelineResourceSinkFields";

import { useGetConnectionQueries } from "@/api/queries/connections";

import { useOverlaySession } from "@/hooks/useOverlaySession";

interface CreatePipelineModalProps {
  isOpen: boolean;
  sourceId?: CreatePipelineModalSeedSearch["sourceId"];
  sinkIds?: CreatePipelineModalSeedSearch["sinkIds"];
  onClose: () => void;
}

interface CreatePipelineModalContentProps {
  isOpen: boolean;
  onClose: () => void;
}

interface CreatePipelineModalContentState {
  activeResource?: CreatePipelineModalResourceRow["name"];
}

const DEFAULT_CONTENT_STATE: CreatePipelineModalContentState = {};

const CreatePipelineModalContent: FC<CreatePipelineModalContentProps> = ({ isOpen, onClose }) => {
  const { step, rowsBySink, activeSinkId, sinkConnections, nodeConfigs } =
    useCreatePipelineModalState();
  const hasSinkFields =
    usePipelineResourceSinkFields(
      sinkConnections.find((connection) => connection.id === activeSinkId),
      nodeConfigs[activeSinkId] ?? {},
    ).fields.length > 0;
  const [localState, setLocalState] =
    useState<CreatePipelineModalContentState>(DEFAULT_CONTENT_STATE);

  const activeRow =
    step === CreatePipelineModalStep.RESOURCES
      ? rowsBySink[activeSinkId]?.find(
          (row) =>
            row.name === localState.activeResource && (row.isColumnSelectable || hasSinkFields),
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
      header={PIPELINE_CREATE_TITLE}
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

const CreatePipelineModalSession: FC<CreatePipelineModalProps> = ({
  isOpen,
  sourceId,
  sinkIds,
  onClose,
}) => {
  const [seedSearch] = useState<CreatePipelineModalSeedSearch>(() => ({ sourceId, sinkIds }));
  const { connections, isLoading } = useGetConnectionQueries(
    getCreatePipelineModalSeedIds(seedSearch),
    { retry: false },
  );

  if (isLoading) {
    return (
      <Modal
        isOpen={isOpen}
        isFullscreen
        header={PIPELINE_CREATE_TITLE}
        onOpenChange={(isOpen) => {
          if (!isOpen) onClose();
        }}
      >
        <PendingLayout />
      </Modal>
    );
  }

  return (
    <CreatePipelineModalProvider
      isOpen={isOpen}
      seed={mapConnectionsToCreatePipelineModalSeed(seedSearch, connections)}
    >
      <CreatePipelineModalContent isOpen={isOpen} onClose={onClose} />
    </CreatePipelineModalProvider>
  );
};

const CreatePipelineModal: FC<CreatePipelineModalProps> = (props) => {
  const session = useOverlaySession(props.isOpen);

  if (!session) {
    return null;
  }

  return <CreatePipelineModalSession key={session} {...props} />;
};

export default CreatePipelineModal;
