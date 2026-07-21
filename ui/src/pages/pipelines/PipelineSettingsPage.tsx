import { useEffect, useMemo, useState } from "react";

import { create } from "@bufbuild/protobuf";
import { styled } from "@linaria/react";
import { useNavigate, useParams } from "@tanstack/react-router";

import Button from "@galaxy-io/dls/buttons/Button";
import FlexWrapper, {
  AlignItems,
  FlexDirection,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import Wrapper from "@galaxy-io/dls/containers/Wrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import { InputSize } from "@galaxy-io/dls/inputs/Input";
import TextAreaInput, { TextAreaSize } from "@galaxy-io/dls/inputs/TextAreaInput";
import TextInput from "@galaxy-io/dls/inputs/TextInput";
import ToggleInput from "@galaxy-io/dls/inputs/ToggleInput";
import Modal from "@galaxy-io/dls/modal/Modal";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";
import Widget from "@galaxy-io/dls/widget/Widget";

import BaseHeader from "@/layouts/components/BaseHeader";
import { BaseHeaderSize } from "@/layouts/components/types";

import { ToastVariant } from "@/providers/toast/Toast";
import { useToast } from "@/providers/toast/useToast";

import {
  useDeletePipelineMutation,
  useGetPipelineQuery,
  useUpdatePipelineMutation,
} from "@/api/queries/pipelines";

import {
  DeletePipelineRequestSchema,
  GetPipelineRequestSchema,
  UpdatePipelineRequestSchema,
} from "@/gen/ingestion/v1/pipelines_pb";

import DangerZone from "@/components/DangerZone";
import DeleteConfirmDialog from "@/components/DeleteConfirmDialog";

const PageWrapper = withTheme(styled.div<PropsWithTheme>`
  width: 100%;
  height: 100%;

  display: flex;
  flex-direction: column;

  overflow: hidden;

  background-color: ${({ theme }) => theme.color.background.base};
`);

interface PipelineSettingsPageState {
  name: string;
  description: string;
  scheduleEnabled: boolean;
  notificationsEnabled: boolean;
  errorAlertsEnabled: boolean;
}

const DEFAULT_STATE: PipelineSettingsPageState = {
  name: "",
  description: "",
  scheduleEnabled: false,
  notificationsEnabled: true,
  errorAlertsEnabled: true,
};

const PipelineSettingsPage = () => {
  const { id } = useParams({ from: "/pipelines/$id" });

  const navigate = useNavigate();

  const { data } = useGetPipelineQuery({
    input: create(GetPipelineRequestSchema, { id }),
  });
  const { mutate: deletePipeline, isPending: isDeleting } = useDeletePipelineMutation();
  const { mutate: updatePipeline, isPending: isSaving } = useUpdatePipelineMutation();

  const { showToast } = useToast();

  const [state, setState] = useState<PipelineSettingsPageState>(DEFAULT_STATE);
  const [isDeleteModalOpen, setIsDeleteModalOpen] = useState(false);

  const handleOpenDeleteModal = () => setIsDeleteModalOpen(true);
  const handleCloseDeleteModal = () => setIsDeleteModalOpen(false);

  const handleConfirmDelete = () => {
    deletePipeline(create(DeletePipelineRequestSchema, { id }), {
      onSuccess: () => {
        showToast({
          header: "Pipeline deleted",
          subheader: `${state.name} has been deleted successfully.`,
          variant: ToastVariant.SUCCESS,
        });
        navigate({ to: "/pipelines" });
      },
      onError: (error) => {
        showToast({
          header: "Delete failed",
          subheader: error instanceof Error ? error.message : "Failed to delete pipeline",
          variant: ToastVariant.ERROR,
        });
        setIsDeleteModalOpen(false);
      },
    });
  };

  useEffect(() => {
    const { pipeline } = data ?? {};
    if (pipeline) {
      setState((prev) => ({ ...prev, name: pipeline.name ?? "" }));
    }
  }, [data]);

  const handleNameChange = (name: string) => {
    setState((prev) => ({ ...prev, name }));
  };

  const handleDescriptionChange = (description: string) => {
    setState((prev) => ({ ...prev, description }));
  };

  const handleScheduleEnabledChange = (scheduleEnabled: boolean) => {
    setState((prev) => ({ ...prev, scheduleEnabled }));
  };

  const handleNotificationsEnabledChange = (notificationsEnabled: boolean) => {
    setState((prev) => ({ ...prev, notificationsEnabled }));
  };

  const handleErrorAlertsEnabledChange = (errorAlertsEnabled: boolean) => {
    setState((prev) => ({ ...prev, errorAlertsEnabled }));
  };

  const hasChanges = useMemo(() => {
    return state.name !== (data?.pipeline?.name ?? "");
  }, [state.name, data?.pipeline?.name]);

  const canSave = hasChanges && state.name.trim().length > 0;

  const handleSave = () => {
    const pipeline = data?.pipeline;
    if (!pipeline) return;

    const request = create(UpdatePipelineRequestSchema, {
      pipeline: { ...pipeline, name: state.name.trim() },
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
          subheader: error instanceof Error ? error.message : "Failed to save pipeline",
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
        <Widget header="General" noHover fillWidth>
          <FlexWrapper direction={FlexDirection.COLUMN} gap={16} fillWidth>
            <TextInput
              value={state.name}
              onChange={handleNameChange}
              size={InputSize.LARGE}
              placeholder="Pipeline name"
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

        <Widget header="Schedule" noHover fillWidth>
          <FlexWrapper fillWidth alignItems={AlignItems.CENTER} gap={16}>
            <FlexWrapper direction={FlexDirection.COLUMN} gap={6} fillWidth>
              <Text>Enable scheduling</Text>
              <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY}>
                Run this pipeline on a recurring schedule.
              </Text>
            </FlexWrapper>
            <ToggleInput value={state.scheduleEnabled} onChange={handleScheduleEnabledChange} />
          </FlexWrapper>
        </Widget>

        <Widget header="Notifications" noHover fillWidth>
          <FlexWrapper
            direction={FlexDirection.COLUMN}
            alignItems={AlignItems.CENTER}
            gap={16}
            fillWidth
          >
            <FlexWrapper alignItems={AlignItems.CENTER} fillWidth>
              <FlexWrapper direction={FlexDirection.COLUMN} gap={6} fillWidth>
                <Text>Pipeline notifications</Text>
                <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY}>
                  Receive notifications when runs complete.
                </Text>
              </FlexWrapper>
              <ToggleInput
                value={state.notificationsEnabled}
                onChange={handleNotificationsEnabledChange}
              />
            </FlexWrapper>

            <FlexWrapper alignItems={AlignItems.CENTER} fillWidth>
              <FlexWrapper direction={FlexDirection.COLUMN} gap={6} fillWidth>
                <Text>Error alerts</Text>
                <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY}>
                  Get notified immediately when errors occur.
                </Text>
              </FlexWrapper>
              <ToggleInput
                value={state.errorAlertsEnabled}
                onChange={handleErrorAlertsEnabledChange}
              />
            </FlexWrapper>
          </FlexWrapper>
        </Widget>

        <DangerZone
          title="Delete pipeline"
          description="This will permanently delete this pipeline and all of its data."
          buttonLabel="Delete pipeline"
          onAction={handleOpenDeleteModal}
        />
      </FlexWrapper>

      <Modal open={isDeleteModalOpen} onClose={handleCloseDeleteModal}>
        <DeleteConfirmDialog
          open={isDeleteModalOpen}
          onClose={handleCloseDeleteModal}
          onConfirm={handleConfirmDelete}
          title="Delete pipeline"
          body="This will permanently delete this pipeline and all associated data."
          confirmationPhrase={state.name || ""}
          confirmLabel="Delete pipeline"
          isPending={isDeleting}
        />
      </Modal>
    </PageWrapper>
  );
};

export default PipelineSettingsPage;
