import { useEffect, useState } from "react";

import { styled } from "@linaria/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, {
  AlignItems,
  FlexDirection,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import TextInput from "@galaxy-io/dls/inputs/TextInput";
import Code from "@galaxy-io/dls/text/Code";
import Paragraph from "@galaxy-io/dls/text/Paragraph";
import Selectable from "@galaxy-io/dls/text/Selectable";
import Text, { TextVariant } from "@galaxy-io/dls/text/Text";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import BaseHeader from "@/layouts/components/BaseHeader";

export interface DeleteConfirmDialogProps {
  open: boolean;
  onClose: () => void;
  onConfirm: () => void;
  title: string;
  body: string;
  confirmationPhrase: string;
  confirmLabel?: string;
  isPending?: boolean;
}

const DeleteConfirmDialogWrapper = withTheme(styled.div<PropsWithTheme>`
  display: flex;
  flex-direction: column;
  width: 520px;
  background-color: ${({ theme }) => theme.color.background.primary};
  border: 0.5px solid ${({ theme }) => theme.color.border.primary};
  border-radius: 8px;
  overflow: hidden;
`);

const DeleteConfirmDialogBodyWrapper = withTheme(styled.div<PropsWithTheme>`
  display: flex;
  flex-direction: column;
  gap: 16px;
  padding: 16px;
  background-color: ${({ theme }) => theme.color.background.base};
`);

interface DeleteConfirmDialogState {
  inputValue: string;
}

const DEFAULT_STATE: DeleteConfirmDialogState = {
  inputValue: "",
};

const DeleteConfirmDialog = ({
  open,
  onClose,
  onConfirm,
  title,
  body,
  confirmationPhrase,
  confirmLabel = "Delete",
  isPending = false,
}: DeleteConfirmDialogProps) => {
  const [state, setState] = useState<DeleteConfirmDialogState>(DEFAULT_STATE);

  const handleInputChange = (value: string) => {
    setState((prev) => ({ ...prev, inputValue: value }));
  };

  useEffect(() => {
    if (open) {
      setState(DEFAULT_STATE);
    }
  }, [open]);

  const isConfirmDisabled = state.inputValue !== confirmationPhrase || isPending;

  return (
    <DeleteConfirmDialogWrapper>
      <FlexWrapper padding={"16px"}>
        <BaseHeader title={title} onClose={onClose} />
      </FlexWrapper>

      <FlexItem grow={0} shrink={0}>
        <HorizontalDivider />
      </FlexItem>

      <DeleteConfirmDialogBodyWrapper>
        <Paragraph variant={TextVariant.SECONDARY}>{body}</Paragraph>
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
          />
        </FlexWrapper>
      </DeleteConfirmDialogBodyWrapper>

      <FlexItem grow={0} shrink={0}>
        <HorizontalDivider />
      </FlexItem>

      <FlexWrapper
        justifyContent={JustifyContent.END}
        alignItems={AlignItems.CENTER}
        padding={"16px"}
        gap={8}
        fillWidth
      >
        <Button
          size={ButtonSize.LARGE}
          label="Cancel"
          variant={ButtonVariant.SECONDARY}
          onClick={onClose}
          isDisabled={isPending}
        />
        <Button
          size={ButtonSize.LARGE}
          label={confirmLabel}
          variant={ButtonVariant.ERROR}
          onClick={onConfirm}
          isDisabled={isConfirmDisabled}
          isLoading={isPending}
        />
      </FlexWrapper>
    </DeleteConfirmDialogWrapper>
  );
};

export default DeleteConfirmDialog;
