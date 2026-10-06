import { type PropsWithChildren, type ReactNode, useEffect, useState } from "react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Alert, { AlertVariant } from "@galaxy-io/dls/feedback/Alert";
import TextInput from "@galaxy-io/dls/inputs/TextInput";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import Modal, { ModalSize } from "@galaxy-io/dls/modal/Modal";
import Code from "@galaxy-io/dls/text/Code";
import Selectable from "@galaxy-io/dls/text/Selectable";
import Text, { TextVariant } from "@galaxy-io/dls/text/Text";

export enum DialogVariant {
  SUCCESS = "SUCCESS",
  WARNING = "WARNING",
  ERROR = "ERROR",
}

export interface DialogProps {
  open: boolean;
  onClose: () => void;
  title: string;
  description?: string;
  /** Message rendered above `children`. */
  body?: string;
  /** Bold lead-in, shown inside the variant panel. */
  bodyTitle?: string;
  /** Omit for a plain paragraph; set to box `body` in a semantic panel. */
  variant?: DialogVariant;
  /** When set, the confirm button unlocks only once this phrase is typed back. */
  confirmationPhrase?: string;
  /** Custom footer. Takes precedence over the Cancel/confirm pair. */
  footer?: ReactNode;
  /** Set to get the standard Cancel/confirm footer. */
  onConfirm?: () => void;
  confirmLabel?: string;
  confirmVariant?: ButtonVariant;
  cancelLabel?: string;
  isConfirmDisabled?: boolean;
  isPending?: boolean;
}

interface DialogState {
  inputValue: string;
}

const DEFAULT_STATE: DialogState = {
  inputValue: "",
};

const DIALOG_VARIANT_TO_ALERT_VARIANT_MAP: Record<DialogVariant, AlertVariant> = {
  [DialogVariant.SUCCESS]: AlertVariant.SUCCESS,
  [DialogVariant.WARNING]: AlertVariant.WARNING,
  [DialogVariant.ERROR]: AlertVariant.ERROR,
};

const normalizeArrows = (value: string) => value.replace(/->/g, "→");

const Dialog = ({
  open,
  onClose,
  title,
  description,
  body,
  bodyTitle,
  variant,
  confirmationPhrase,
  footer,
  onConfirm,
  confirmLabel = "Confirm",
  confirmVariant = ButtonVariant.PRIMARY,
  cancelLabel = "Cancel",
  isConfirmDisabled = false,
  isPending = false,
  children,
}: PropsWithChildren<DialogProps>) => {
  const [state, setState] = useState<DialogState>(DEFAULT_STATE);

  const handleInputChange = (inputValue: string) => {
    setState((prev) => ({ ...prev, inputValue }));
  };

  useEffect(() => {
    if (open) {
      setState(DEFAULT_STATE);
    }
  }, [open]);

  const handleClose = () => {
    if (!isPending) onClose();
  };

  const isConfirmBlocked =
    isPending ||
    isConfirmDisabled ||
    (confirmationPhrase !== undefined &&
      normalizeArrows(state.inputValue) !== normalizeArrows(confirmationPhrase));

  const renderedFooter =
    footer ??
    (onConfirm && (
      <>
        <Button
          size={ButtonSize.LARGE}
          label={cancelLabel}
          variant={ButtonVariant.SECONDARY}
          onClick={onClose}
          isDisabled={isPending}
        />
        <Button
          size={ButtonSize.LARGE}
          label={confirmLabel}
          variant={confirmVariant}
          onClick={onConfirm}
          isDisabled={isConfirmBlocked}
          isLoading={isPending}
        />
      </>
    ));

  return (
    <Modal
      header={title}
      size={ModalSize.MEDIUM}
      isOpen={open}
      isDismissable={!isPending}
      onOpenChange={(isOpen) => {
        if (!isOpen) handleClose();
      }}
      footer={renderedFooter}
    >
      <Flex alignItems={AlignItems.STRETCH} direction={FlexDirection.COLUMN} gap={16}>
        {description && (
          <Text isProse variant={TextVariant.SECONDARY}>
            {description}
          </Text>
        )}
        {body &&
          (variant ? (
            <Alert variant={DIALOG_VARIANT_TO_ALERT_VARIANT_MAP[variant]} header={bodyTitle}>
              {body}
            </Alert>
          ) : (
            <Text isProse variant={TextVariant.SECONDARY}>
              {body}
            </Text>
          ))}
        {children}
        {confirmationPhrase !== undefined && (
          <Flex alignItems={AlignItems.START} direction={FlexDirection.COLUMN} gap={8} fillWidth>
            <Text variant={TextVariant.PRIMARY}>
              Type{" "}
              <Code>
                <Selectable>{confirmationPhrase}</Selectable>
              </Code>{" "}
              to confirm:
            </Text>
            <TextInput
              value={state.inputValue}
              onChange={handleInputChange}
              placeholder={confirmationPhrase}
              fillWidth
              autoFocus
            />
          </Flex>
        )}
      </Flex>
    </Modal>
  );
};

export default Dialog;
