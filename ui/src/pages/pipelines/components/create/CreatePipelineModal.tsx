import { styled } from "@linaria/react";
import { match } from "ts-pattern";

import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

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

const ModalWrapper = withTheme(styled.div<PropsWithTheme>`
  display: flex;

  height: ${CREATE_PIPELINE_MODAL_HEIGHT}px;
  width: ${CREATE_PIPELINE_MODAL_WIDTH}px;

  background-color: ${({ theme }) => theme.color.background.primary};
  border: 0.5px solid ${({ theme }) => theme.color.border.primary};
  border-radius: 8px;
  overflow: hidden;
`);

const MainWrapper = styled.div`
  display: flex;
  flex-direction: column;
  flex: 1;
  min-width: 0;
`;

const BodyWrapper = withTheme(styled.div<PropsWithTheme<{ $isPadded: boolean }>>`
  display: flex;
  flex-direction: column;

  flex: 1;
  min-height: 0;
  width: 100%;
  overflow-y: auto;
  background-color: ${({ theme }) => theme.color.background.base};
  padding: ${({ $isPadded }) => ($isPadded ? "16px" : "0")};
`);

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
          <FlexWrapper padding="12px 16px" fillWidth>
            <BaseHeader title={CREATE_PIPELINE_MODAL_STEP_TO_TITLE_MAP[step]} onClose={onClose} />
          </FlexWrapper>
        </FlexItem>
        <FlexItem grow={0} shrink={0}>
          <HorizontalDivider />
        </FlexItem>

        <BodyWrapper $isPadded={CREATE_PIPELINE_MODAL_STEP_TO_IS_PADDED_MAP[step]}>
          {renderBody()}
        </BodyWrapper>

        <FlexItem grow={0} shrink={0}>
          <HorizontalDivider />
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
