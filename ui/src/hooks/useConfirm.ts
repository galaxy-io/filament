import { useState } from "react";

import { ToastVariant } from "@galaxy-io/dls/toast/Toast";
import { useToast } from "@galaxy-io/dls/toast/useToast";

import { getErrorMessage } from "@/utils/errors";

interface UseConfirmCallbacks {
  onSuccess: () => void;
  onError: (error: unknown) => void;
}

interface UseConfirmMessages<T> {
  successHeader: string;
  successSubheader: (target: T) => string;
  errorHeader: string;
  errorFallback: string;
}

interface UseConfirmInput<T> {
  entityLabel: string;
  entityName: (target: T) => string;
  messages?: Partial<UseConfirmMessages<T>>;
  onConfirm: (target: T, callbacks: UseConfirmCallbacks) => void;
  onConfirmed?: (target: T) => void;
}

interface UseConfirmState<T> {
  target: T | undefined;
}

const DEFAULT_STATE = {
  target: undefined,
};

export const useConfirm = <T>({
  entityLabel,
  entityName,
  messages,
  onConfirm,
  onConfirmed,
}: UseConfirmInput<T>) => {
  const { showToast } = useToast();

  const [state, setState] = useState<UseConfirmState<T>>(DEFAULT_STATE);

  const handleOpen = (target: T) => setState({ target });
  const handleClose = () => setState(DEFAULT_STATE);

  const handleConfirm = () => {
    const { target } = state;
    if (target === undefined) return;

    onConfirm(target, {
      onSuccess: () => {
        showToast({
          header: messages?.successHeader ?? `${entityLabel} deleted`,
          subheader:
            messages?.successSubheader?.(target) ??
            `${entityName(target)} has been deleted successfully.`,
          variant: ToastVariant.SUCCESS,
        });
        handleClose();
        onConfirmed?.(target);
      },
      onError: (error) => {
        showToast({
          header: messages?.errorHeader ?? "Delete failed",
          subheader: getErrorMessage(
            error,
            messages?.errorFallback ?? `Failed to delete ${entityLabel.toLowerCase()}`,
          ),
          variant: ToastVariant.ERROR,
        });
        handleClose();
      },
    });
  };

  return {
    isOpen: state.target !== undefined,
    target: state.target,
    handleOpen,
    handleClose,
    handleConfirm,
  };
};
