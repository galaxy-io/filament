import { CreatePipelineModalActionType } from "@/pages/pipelines/components/create/actions";
import {
  useCreatePipelineModalDispatch,
  useCreatePipelineModalState,
} from "@/pages/pipelines/components/create/CreatePipelineModalProvider";
import PipelineNotifierTable from "@/pages/pipelines/components/notifier/PipelineNotifierTable";
import type {
  PipelineNotifier,
  PipelineNotifierState,
} from "@/pages/pipelines/components/notifier/types";

const CreatePipelineModalDeliveryNotifications = () => {
  const { notifiers } = useCreatePipelineModalState();
  const dispatch = useCreatePipelineModalDispatch();

  const handleCreate = (state: PipelineNotifierState, onSuccess: () => void) => {
    dispatch({
      type: CreatePipelineModalActionType.ADD_NOTIFIER,
      payload: { ...state, id: crypto.randomUUID() },
    });
    onSuccess();
  };

  const handleUpdate = (
    notifier: PipelineNotifier,
    state: PipelineNotifierState,
    onSuccess: () => void,
  ) => {
    dispatch({
      type: CreatePipelineModalActionType.UPDATE_NOTIFIER,
      payload: { id: notifier.id, partial: state },
    });
    onSuccess();
  };

  const handleToggleEnabled = (notifier: PipelineNotifier, isEnabled: boolean) => {
    dispatch({
      type: CreatePipelineModalActionType.UPDATE_NOTIFIER,
      payload: { id: notifier.id, partial: { isEnabled } },
    });
  };

  const handleDelete = (notifier: PipelineNotifier) => {
    dispatch({ type: CreatePipelineModalActionType.REMOVE_NOTIFIER, payload: notifier.id });
  };

  return (
    <PipelineNotifierTable
      rows={notifiers}
      onCreate={handleCreate}
      onUpdate={handleUpdate}
      onDelete={handleDelete}
      onToggleEnabled={handleToggleEnabled}
    />
  );
};

export default CreatePipelineModalDeliveryNotifications;
