import type { FC } from "react";

import { create } from "@bufbuild/protobuf";

import ConfirmDialog from "@galaxy-io/dls/modal/ConfirmDialog";
import { ToastVariant } from "@galaxy-io/dls/toast/Toast";
import { useToast } from "@galaxy-io/dls/toast/useToast";

import {
  CreatePipelineNotifierRequestSchema,
  DeletePipelineNotifierRequestSchema,
  ListPipelineNotifiersRequestSchema,
  type Notifier,
  UpdatePipelineNotifierRequestSchema,
} from "@/gen/ingestion/v1/notifiers_pb";

import PipelineNotifierTable from "@/pages/pipelines/components/notifier/PipelineNotifierTable";
import type {
  PipelineNotifier,
  PipelineNotifierState,
} from "@/pages/pipelines/components/notifier/types";
import {
  mapNotifierToPipelineNotifierState,
  mapPipelineNotifierStateToInput,
} from "@/pages/pipelines/components/notifier/utils";

import { usePipelineParams } from "@/module/hooks";

import {
  useCreatePipelineNotifierMutation,
  useDeletePipelineNotifierMutation,
  useSuspenseListPipelineNotifiersQuery,
  useUpdatePipelineNotifierMutation,
} from "@/api/queries/notifiers";

import { useConfirm } from "@/hooks/useConfirm";

import { getErrorMessage } from "@/utils/errors";

const mapNotifierToRow = (notifier: Notifier): PipelineNotifier => ({
  ...mapNotifierToPipelineNotifierState(notifier),
  id: notifier.id,
});

const PipelineSettingsPageNotifications: FC = () => {
  const { toast: showToast } = useToast();
  const { id: pipelineId } = usePipelineParams();

  const { data } = useSuspenseListPipelineNotifiersQuery({
    input: create(ListPipelineNotifiersRequestSchema, { pipelineId }),
  });
  const rows = data.notifiers.map(mapNotifierToRow);

  const { mutate: createNotifier, isPending: isCreating } = useCreatePipelineNotifierMutation();
  const { mutate: updateNotifier, isPending: isUpdating } = useUpdatePipelineNotifierMutation();
  const { mutate: deleteNotifier, isPending: isDeleting } = useDeletePipelineNotifierMutation();

  const { handleOpen, isOpen, target, handleClose, handleConfirm } = useConfirm<PipelineNotifier>({
    entityLabel: "Notifier",
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
            variant: ToastVariant.SUCCESS,
          });
          onSuccess();
        },
        onError: (error) => {
          showToast({
            header: "Create failed",
            description: getErrorMessage(error, "Failed to create notifier"),
            variant: ToastVariant.ERROR,
          });
        },
      },
    );
  };

  const updateRow = (
    row: PipelineNotifier,
    state: PipelineNotifierState,
    successHeader: string,
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
          showToast({ header: successHeader, variant: ToastVariant.SUCCESS });
          onSuccess?.();
        },
        onError: (error) => {
          showToast({
            header: "Update failed",
            description: getErrorMessage(error, "Failed to update notifier"),
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
    updateRow(row, state, "Notifier saved", onSuccess);
  };

  const handleToggleEnabled = (row: PipelineNotifier, isEnabled: boolean) => {
    updateRow(row, { ...row, isEnabled }, isEnabled ? "Notifier enabled" : "Notifier disabled");
  };

  return (
    <>
      <PipelineNotifierTable
        rows={rows}
        isOpenInitial={rows.length > 0}
        isSaving={isCreating || isUpdating || isDeleting}
        onCreate={handleCreate}
        onUpdate={handleUpdate}
        onDelete={handleOpen}
        onToggleEnabled={handleToggleEnabled}
      />
      <ConfirmDialog
        isOpen={isOpen}
        onOpenChange={(isOpen) => {
          if (!isOpen) handleClose();
        }}
        onConfirm={handleConfirm}
        header="Delete notifier?"
        description="This deletes the notifier. It cannot be undone."
        confirmValue={target?.name}
        label="Delete notifier"
        isDestructive
      />
    </>
  );
};

export default PipelineSettingsPageNotifications;
