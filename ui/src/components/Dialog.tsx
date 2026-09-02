import { type PropsWithChildren, type ReactNode, useEffect, useState } from "react";

import { styled } from "@linaria/react";
import {
  CheckCircleIcon,
  type Icon as PhosphorIcon,
  WarningCircleIcon,
  XCircleIcon,
} from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, {
  AlignItems,
  FlexDirection,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import TextInput from "@galaxy-io/dls/inputs/TextInput";
import Modal from "@galaxy-io/dls/modal/Modal";
import Code from "@galaxy-io/dls/text/Code";
import Paragraph from "@galaxy-io/dls/text/Paragraph";
import Selectable from "@galaxy-io/dls/text/Selectable";
import Text, { TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";
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

const DialogWrapper = withTheme(styled.div<PropsWithTheme>`
  display: flex;
  flex-direction: column;
  width: ${DIALOG_WIDTH}px;
  max-width: calc(100vw - 32px);
  background-color: ${({ theme }) => theme.color.background.primary};
  border: 0.5px solid ${({ theme }) => theme.color.border.primary};
  border-radius: 8px;
  overflow: hidden;
`);

const BodyWrapper = withTheme(styled.div<PropsWithTheme>`
  display: flex;
  flex-direction: column;
  gap: 16px;
  min-height: 0;
  padding: 16px;
  background-color: ${({ theme }) => theme.color.background.base};
`);

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
  [DialogVariant.SUCCESS]: WidgetVariant.SUCCESS,
  [DialogVariant.WARNING]: WidgetVariant.WARNING,
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
    <Modal open={open} onClose={handleClose}>
      <DialogWrapper>
        <FlexWrapper padding="16px">
          <BaseHeader title={title} description={description} onClose={handleClose} />
        </FlexWrapper>
        <HorizontalDivider />
        <BodyWrapper>
          {body &&
            (variant ? (
              <Widget variant={DIALOG_VARIANT_TO_WIDGET_VARIANT_MAP[variant]} fillWidth noHover>
                <FlexWrapper alignItems={AlignItems.START} gap={12} fillWidth>
                  <FlexItem grow={0} shrink={0} display="flex">
                    <Icon
                      component={DIALOG_VARIANT_TO_ICON_MAP[variant]}
                      size={18}
                      variant={DIALOG_VARIANT_TO_ICON_VARIANT_MAP[variant]}
                    />
                  </FlexItem>
                  <FlexWrapper direction={FlexDirection.COLUMN} gap={4} minWidth={0}>
                    {bodyTitle && <Text weight={TextWeight.MEDIUM}>{bodyTitle}</Text>}
                    <Paragraph variant={TextVariant.SECONDARY}>{body}</Paragraph>
                  </FlexWrapper>
                </FlexWrapper>
              </Widget>
            ) : (
              <Paragraph variant={TextVariant.SECONDARY}>{body}</Paragraph>
            ))}
          {children}
          {confirmationPhrase !== undefined && (
            <FlexWrapper direction={FlexDirection.COLUMN} gap={8} fillWidth>
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
            </FlexWrapper>
          )}
        </BodyWrapper>
        {renderedFooter && (
          <>
            <HorizontalDivider />
            <FlexWrapper
              alignItems={AlignItems.CENTER}
              justifyContent={JustifyContent.END}
              padding="16px"
              gap={8}
              fillWidth
            >
              {renderedFooter}
            </FlexWrapper>
          </>
        )}
      </DialogWrapper>
    </Modal>
  );
};

export default Dialog;
