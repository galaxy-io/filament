import { create } from "@bufbuild/protobuf";
import { useParams } from "@tanstack/react-router";

import Accordion from "@galaxy-io/dls/accordion/Accordion";
import { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import { ToastVariant } from "@galaxy-io/dls/toast/Toast";
import { useToast } from "@galaxy-io/dls/toast/useToast";

import {
  CreatePipelineNotifierRequestSchema,
  DeletePipelineNotifierRequestSchema,
  ListPipelineNotifiersRequestSchema,
  type Notifier,
  UpdatePipelineNotifierRequestSchema,
} from "@/gen/ingestion/v1/notifiers_pb";

import Dialog from "@/components/Dialog";

import PipelineNotifierTable from "@/pages/pipelines/components/notifier/PipelineNotifierTable";
import type {
  PipelineNotifier,
  PipelineNotifierState,
} from "@/pages/pipelines/components/notifier/types";
import {
  mapNotifierToPipelineNotifierState,
  mapPipelineNotifierStateToInput,
} from "@/pages/pipelines/components/notifier/utils";

import {
  useCreatePipelineNotifierMutation,
  useDeletePipelineNotifierMutation,
  useListPipelineNotifiersQuery,
  useUpdatePipelineNotifierMutation,
} from "@/api/queries/notifiers";

import { useConfirm } from "@/hooks/useConfirm";

import { getErrorMessage } from "@/utils/errors";

interface PipelineSettingsPageNotifierToast {
  header: string;
  subheader: string;
}

const mapNotifierToRow = (notifier: Notifier): PipelineNotifier => ({
  ...mapNotifierToPipelineNotifierState(notifier),
  id: notifier.id,
});

const PipelineSettingsPageNotifications = () => {
  const { showToast } = useToast();
  const { id: pipelineId } = useParams({ from: "/_app/pipelines/$id" });

  const { data, isLoading } = useListPipelineNotifiersQuery({
    input: create(ListPipelineNotifiersRequestSchema, { pipelineId }),
  });
  const rows = (data?.notifiers ?? []).map(mapNotifierToRow);

  const { mutate: createNotifier, isPending: isCreating } = useCreatePipelineNotifierMutation();
  const { mutate: updateNotifier, isPending: isUpdating } = useUpdatePipelineNotifierMutation();
  const { mutate: deleteNotifier, isPending: isDeleting } = useDeletePipelineNotifierMutation();

  const { handleOpen, isOpen, target, handleClose, handleConfirm } = useConfirm<PipelineNotifier>({
    entityLabel: "Notifier",
    entityName: (row) => row.name,
    onConfirm: (row, { onSuccess, onError }) =>
      deleteNotifier(
        create(DeletePipelineNotifierRequestSchema, { pipelineId, notifierId: row.id }),
        { onSuccess, onError },
      ),
  });

  const handleCreate = (state: PipelineNotifierState, onSuccess: () => void) => {
    createNotifier(
      create(CreatePipelineNotifierRequestSchema, {
        pipelineId,
        notifier: mapPipelineNotifierStateToInput(state),
      }),
      {
        onSuccess: () => {
          showToast({
            header: "Notifier created",
            subheader: `${state.name} will now send notifications.`,
            variant: ToastVariant.SUCCESS,
          });
          onSuccess();
        },
        onError: (error) => {
          showToast({
            header: "Create failed",
            subheader: getErrorMessage(error, "Failed to create notifier"),
            variant: ToastVariant.ERROR,
          });
        },
      },
    );
  };

  const updateRow = (
    row: PipelineNotifier,
    state: PipelineNotifierState,
    toast: PipelineSettingsPageNotifierToast,
    onSuccess?: () => void,
  ) => {
    updateNotifier(
      create(UpdatePipelineNotifierRequestSchema, {
        pipelineId,
        notifierId: row.id,
        notifier: mapPipelineNotifierStateToInput(state),
      }),
      {
        onSuccess: () => {
          showToast({ ...toast, variant: ToastVariant.SUCCESS });
          onSuccess?.();
        },
        onError: (error) => {
          showToast({
            header: "Update failed",
            subheader: getErrorMessage(error, "Failed to update notifier"),
            variant: ToastVariant.ERROR,
          });
        },
      },
    );
  };

  const handleUpdate = (
    row: PipelineNotifier,
    state: PipelineNotifierState,
    onSuccess: () => void,
  ) => {
    updateRow(
      row,
      state,
      { header: "Notifier saved", subheader: `${state.name} has been saved.` },
      onSuccess,
    );
  };

  const handleToggleEnabled = (row: PipelineNotifier, isEnabled: boolean) => {
    updateRow(
      row,
      { ...row, isEnabled },
      isEnabled
        ? { header: "Notifier enabled", subheader: `${row.name} will now send notifications.` }
        : {
            header: "Notifier disabled",
            subheader: `${row.name} will no longer send notifications.`,
          },
    );
  };

  return (
    <Accordion header="Notifications" isOpenInitial>
      <PipelineNotifierTable
        rows={rows}
        isLoading={isLoading}
        isSaving={isCreating || isUpdating || isDeleting}
        onCreate={handleCreate}
        onUpdate={handleUpdate}
        onDelete={handleOpen}
        onToggleEnabled={handleToggleEnabled}
      />
      <Dialog
        open={isOpen}
        onClose={handleClose}
        onConfirm={handleConfirm}
        title="Delete notifier"
        body="This is a destructive action and cannot be undone."
        confirmationPhrase={target?.name}
        confirmLabel="Delete notifier"
        confirmVariant={ButtonVariant.ERROR}
        isPending={isDeleting}
      />
    </Accordion>
  );
};

export default PipelineSettingsPageNotifications;
