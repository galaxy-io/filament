import { type PropsWithChildren, type ReactNode, useEffect, useState } from "react";

import { styled } from "@linaria/react";
import {
  CheckCircleIcon,
  type Icon as PhosphorIcon,
  WarningCircleIcon,
  XCircleIcon,
} from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import TextInput from "@galaxy-io/dls/inputs/TextInput";
import Divider from "@galaxy-io/dls/layout/Divider";
import Flex, { AlignItems, FlexDirection, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import Modal from "@galaxy-io/dls/modal/Modal";
import Code from "@galaxy-io/dls/text/Code";
import Selectable from "@galaxy-io/dls/text/Selectable";
import Text, { TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import { t } from "@galaxy-io/dls/theme/tokens/t";
import Widget, { WidgetVariant } from "@galaxy-io/dls/widget/Widget";

import BaseHeader from "@/layouts/components/BaseHeader";

export const DIALOG_WIDTH = 640;

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

const DialogWrapper = styled.div`
  display: flex;
  flex-direction: column;
  width: ${DIALOG_WIDTH}px;
  max-width: calc(100vw - 32px);
  background-color: ${t.color.background.primary};
  border: 0.5px solid ${t.color.border.primary};
  border-radius: 8px;
  overflow: hidden;
`;

const BodyWrapper = styled.div`
  display: flex;
  flex-direction: column;
  gap: 16px;
  min-height: 0;
  padding: 16px;
  background-color: ${t.color.background.base};
`;

interface DialogState {
  inputValue: string;
}

const DEFAULT_STATE: DialogState = {
  inputValue: "",
};

const DIALOG_VARIANT_TO_ICON_MAP: Record<DialogVariant, PhosphorIcon> = {
  [DialogVariant.SUCCESS]: CheckCircleIcon,
  [DialogVariant.WARNING]: WarningCircleIcon,
  [DialogVariant.ERROR]: XCircleIcon,
};

const DIALOG_VARIANT_TO_ICON_VARIANT_MAP: Record<DialogVariant, IconVariant> = {
  [DialogVariant.SUCCESS]: IconVariant.SUCCESS,
  [DialogVariant.WARNING]: IconVariant.WARNING,
  [DialogVariant.ERROR]: IconVariant.ERROR,
};

const DIALOG_VARIANT_TO_WIDGET_VARIANT_MAP: Record<DialogVariant, WidgetVariant> = {
  // @dls-migrate widget.WidgetVariant.SUCCESS: Removed: use a neutral card with an `Alert`, a labeled `Beacon` or a `Chip` inside.
  [DialogVariant.SUCCESS]: WidgetVariant.SUCCESS,
  // @dls-migrate widget.WidgetVariant.WARNING: Removed: use a neutral card with an `Alert`, a labeled `Beacon` or a `Chip` inside.
  [DialogVariant.WARNING]: WidgetVariant.WARNING,
  // @dls-migrate widget.WidgetVariant.ERROR: Removed: use a neutral card with an `Alert`, a labeled `Beacon` or a `Chip` inside.
  [DialogVariant.ERROR]: WidgetVariant.ERROR,
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
    <Modal /* @dls-migrate modal.ariaLabel: The dialog needs a name: give it a `header` (often the title from the old `Widget`) or an `ariaLabel`. */
      isOpen={open}
      onOpenChange={(isOpen) => {
        if (!isOpen) handleClose();
      }}
    >
      <DialogWrapper>
        <Flex alignItems={AlignItems.START} padding={16}>
          <BaseHeader title={title} description={description} onClose={handleClose} />
        </Flex>
        <Divider />
        <BodyWrapper>
          {body &&
            (variant ? (
              <Widget
                variant={
                  DIALOG_VARIANT_TO_WIDGET_VARIANT_MAP[variant]
                } /* @dls-migrate widget.fillWidth: Grow the card with a `FlexItem` or a `Grid` track. */
                fillWidth
              >
                <Flex alignItems={AlignItems.START} gap={12} fillWidth>
                  <FlexItem
                    grow={0}
                    shrink={
                      0
                    } /* @dls-migrate flexitem.display: A FlexItem is not a flex container: use `<Flex grow={1}>` or nest a `Flex`. */
                    display="flex"
                  >
                    <Icon
                      component={DIALOG_VARIANT_TO_ICON_MAP[variant]}
                      size={18}
                      variant={DIALOG_VARIANT_TO_ICON_VARIANT_MAP[variant]}
                    />
                  </FlexItem>
                  <Flex
                    alignItems={AlignItems.START}
                    direction={FlexDirection.COLUMN}
                    gap={4}
                    minWidth={0}
                  >
                    {bodyTitle && <Text weight={TextWeight.MEDIUM}>{bodyTitle}</Text>}
                    <Text isProse variant={TextVariant.SECONDARY}>
                      {body}
                    </Text>
                  </Flex>
                </Flex>
              </Widget>
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
        </BodyWrapper>
        {renderedFooter && (
          <>
            <Divider />
            <Flex
              alignItems={AlignItems.CENTER}
              justifyContent={JustifyContent.END}
              padding={16}
              gap={8}
              fillWidth
            >
              {renderedFooter}
            </Flex>
          </>
        )}
      </DialogWrapper>
    </Modal>
  );
};

export default Dialog;
