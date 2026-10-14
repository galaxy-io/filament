import type { FC } from "react";

import {
  useCreatePipelineModalActions,
  useCreatePipelineModalState,
} from "@/pages/pipelines/components/create/CreatePipelineModalProvider";
import PipelineNotifierTable from "@/pages/pipelines/components/notifier/PipelineNotifierTable";
import type {
  PipelineNotifier,
  PipelineNotifierState,
} from "@/pages/pipelines/components/notifier/types";

const CreatePipelineModalDeliveryNotifications: FC = () => {
  const { notifiers } = useCreatePipelineModalState();
  const { addNotifier, removeNotifier, updateNotifier } = useCreatePipelineModalActions();

  const handleCreate = (state: PipelineNotifierState, onSuccess: () => void) => {
    addNotifier({ ...state, id: crypto.randomUUID() });
    onSuccess();
  };

  const handleUpdate = (
    notifier: PipelineNotifier,
    state: PipelineNotifierState,
    onSuccess: () => void,
  ) => {
    updateNotifier({ id: notifier.id, partial: state });
    onSuccess();
  };

  const handleToggleEnabled = (notifier: PipelineNotifier, isEnabled: boolean) => {
    updateNotifier({ id: notifier.id, partial: { isEnabled } });
  };

  const handleDelete = (notifier: PipelineNotifier) => {
    removeNotifier(notifier.id);
  };

  return (
    <PipelineNotifierTable
      rows={notifiers}
      isOpenInitial={notifiers.length > 0}
      onCreate={handleCreate}
      onUpdate={handleUpdate}
      onDelete={handleDelete}
      onToggleEnabled={handleToggleEnabled}
    />
  );
};

export default CreatePipelineModalDeliveryNotifications;
