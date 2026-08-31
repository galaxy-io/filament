import { useState } from "react";

import { ToastVariant } from "@galaxy-io/dls/toast/Toast";
import { useToast } from "@galaxy-io/dls/toast/useToast";

import { getErrorMessage } from "@/utils/errors";

interface UseDeleteConfirmInput {
  entityLabel: string;
  entityName: string;
  onDelete: ({
    onSuccess,
    onError,
  }: {
    onSuccess: () => void;
    onError: (error: unknown) => void;
  }) => void;
  onDeleted: () => void;
}

interface UseDeleteConfirmState {
  isOpen: boolean;
}

const DEFAULT_STATE: UseDeleteConfirmState = {
  isOpen: false,
};

export const useDeleteConfirm = ({
  entityLabel,
  entityName,
  onDelete,
  onDeleted,
}: UseDeleteConfirmInput) => {
  const { showToast } = useToast();

  const [state, setState] = useState<UseDeleteConfirmState>(DEFAULT_STATE);

  const handleOpen = () => setState({ isOpen: true });
  const handleClose = () => setState({ isOpen: false });

  const handleConfirm = () => {
    onDelete({
      onSuccess: () => {
        showToast({
          header: `${entityLabel} deleted`,
          subheader: `${entityName} has been deleted successfully.`,
          variant: ToastVariant.SUCCESS,
        });
        handleClose();
        onDeleted();
      },
      onError: (error) => {
        showToast({
          header: "Delete failed",
          subheader: getErrorMessage(error, `Failed to delete ${entityLabel.toLowerCase()}`),
          variant: ToastVariant.ERROR,
        });
        handleClose();
      },
    });
  };

  return { isOpen: state.isOpen, handleOpen, handleClose, handleConfirm };
};
