import { useState } from "react";

import { create } from "@bufbuild/protobuf";
import { styled } from "@linaria/react";
import { useNavigate } from "@tanstack/react-router";

import Button from "@galaxy-io/dls/buttons/Button";
import FlexWrapper, {
  AlignItems,
  FlexDirection,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import Wrapper from "@galaxy-io/dls/containers/Wrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import CopyInput from "@galaxy-io/dls/inputs/CopyInput";
import { InputSize } from "@galaxy-io/dls/inputs/Input";
import TextAreaInput, { TextAreaSize } from "@galaxy-io/dls/inputs/TextAreaInput";
import TextInput from "@galaxy-io/dls/inputs/TextInput";
import Modal from "@galaxy-io/dls/modal/Modal";
import Text, { TextWeight } from "@galaxy-io/dls/text/Text";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";
import { ToastVariant } from "@galaxy-io/dls/toast/Toast";
import { useToast } from "@galaxy-io/dls/toast/useToast";
import Widget from "@galaxy-io/dls/widget/Widget";

import {
  DeletePipelineRequestSchema,
  type Pipeline,
  UpdatePipelineRequestSchema,
} from "@/gen/ingestion/v1/pipelines_pb";

import DangerZone from "@/components/DangerZone";
import DeleteConfirmDialog from "@/components/DeleteConfirmDialog";

import BaseHeader, { BaseHeaderSize } from "@/layouts/components/BaseHeader";

import { formatPipelineName } from "@/pages/pipelines/utils";

import { useDeletePipelineMutation, useUpdatePipelineMutation } from "@/api/queries/pipelines";

import { useDeleteConfirm } from "@/hooks/useDeleteConfirm";

import { getErrorMessage } from "@/utils/errors";

const PageWrapper = withTheme(styled.div<PropsWithTheme>`
  width: 100%;
  height: 100%;

  display: flex;
  flex-direction: column;

  overflow: hidden;

  background-color: ${({ theme }) => theme.color.background.base};
`);

interface PipelineSettingsFormState {
  name: Pipeline["name"];
  description: Pipeline["description"];
}

const DEFAULT_STATE: PipelineSettingsFormState = {
  name: "",
  description: "",
};

interface PipelineSettingsFormProps {
  pipeline: Pipeline;
}

const PipelineSettingsForm = ({ pipeline }: PipelineSettingsFormProps) => {
  const navigate = useNavigate();

  const { mutate: deletePipeline, isPending: isDeleting } = useDeletePipelineMutation();
  const { mutate: updatePipeline, isPending: isSaving } = useUpdatePipelineMutation();

  const { showToast } = useToast();

  const [state, setState] = useState<PipelineSettingsFormState>(() => ({
    ...DEFAULT_STATE,
    name: pipeline.name,
    description: pipeline.description,
  }));

  const { handleOpen, isOpen, handleClose, handleConfirm } = useDeleteConfirm({
    entityLabel: "Pipeline",
    entityName: state.name,
    onDelete: ({ onSuccess, onError }) =>
      deletePipeline(create(DeletePipelineRequestSchema, { id: pipeline.id }), {
        onSuccess,
        onError,
      }),
    onDeleted: () => navigate({ to: "/pipelines" }),
  });

  const handleNameChange = (name: string) => {
    setState((prev) => ({ ...prev, name }));
  };

  const handleDescriptionChange = (description: string) => {
    setState((prev) => ({ ...prev, description }));
  };

  const hasChanges =
    state.name.trim() !== pipeline.name || state.description.trim() !== pipeline.description;

  const canSave = hasChanges && state.name.trim().length > 0;

  const handleSave = () => {
    const request = create(UpdatePipelineRequestSchema, {
      pipeline: {
        ...pipeline,
        name: state.name.trim(),
        description: state.description.trim(),
      },
    });

    updatePipeline(request, {
      onSuccess: () => {
        showToast({
          header: "Pipeline saved",
          subheader: "Your pipeline has been saved successfully.",
          variant: ToastVariant.SUCCESS,
        });
      },
      onError: (error) => {
        showToast({
          header: "Save failed",
          subheader: getErrorMessage(error, "Failed to save pipeline"),
          variant: ToastVariant.ERROR,
        });
      },
    });
  };

  return (
    <PageWrapper>
      <Wrapper padding={"16px"} fillWidth>
        <BaseHeader
          size={BaseHeaderSize.LARGE}
          title="Settings"
          description="Configure your pipeline settings and preferences."
        />
      </Wrapper>
      <HorizontalDivider />
      <FlexWrapper
        direction={FlexDirection.COLUMN}
        gap={16}
        padding={"16px"}
        minWidth={400}
        maxWidth={600}
      >
        <Widget noHover fillWidth>
          <FlexWrapper
            fillWidth
            alignItems={AlignItems.CENTER}
            justifyContent={JustifyContent.SPACE_BETWEEN}
          >
            <Text weight={TextWeight.MEDIUM}>Pipeline ID</Text>
            <CopyInput value={pipeline.id} size={InputSize.SMALL} width={272} isMonospace />
          </FlexWrapper>
        </Widget>
        <Widget header="General" noHover fillWidth>
          <FlexWrapper direction={FlexDirection.COLUMN} gap={16} fillWidth>
            <TextInput
              value={state.name}
              onChange={handleNameChange}
              size={InputSize.LARGE}
              placeholder={formatPipelineName(pipeline)}
              label="Name"
              fillWidth
            />
            <TextAreaInput
              value={state.description}
              onChange={handleDescriptionChange}
              size={TextAreaSize.LARGE}
              placeholder="Optional description"
              label="Description"
              fillWidth
            />
            <FlexWrapper justifyContent={JustifyContent.END} fillWidth>
              <Button
                label="Save"
                isDisabled={!canSave}
                isLoading={isSaving}
                onClick={handleSave}
              />
            </FlexWrapper>
          </FlexWrapper>
        </Widget>

        <DangerZone
          title="Delete pipeline"
          description="This will permanently delete this pipeline and all of its data."
          onDelete={handleOpen}
        />
      </FlexWrapper>

      <Modal open={isOpen} onClose={handleClose}>
        <DeleteConfirmDialog
          open={isOpen}
          onClose={handleClose}
          onConfirm={handleConfirm}
          title="Delete pipeline"
          body="This will permanently delete this pipeline and all associated data."
          confirmationPhrase={formatPipelineName(pipeline)}
          confirmLabel="Delete pipeline"
          isPending={isDeleting}
        />
      </Modal>
    </PageWrapper>
  );
};

export default PipelineSettingsForm;
