import { useEffect, useState } from "react";

import { styled } from "@linaria/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, { FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import TextInput from "@galaxy-io/dls/inputs/TextInput";
import Code from "@galaxy-io/dls/text/Code";
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

const Wrapper = withTheme(styled.div<PropsWithTheme>`
  display: flex;
  flex-direction: column;
  width: 520px;
  background-color: ${({ theme }) => theme.color.background.primary};
  border: 0.5px solid ${({ theme }) => theme.color.border.primary};
  border-radius: 8px;
  overflow: hidden;
`);

const HeaderWrapper = styled.div`
  padding: 16px;
`;

const BodyWrapper = withTheme(styled.div<PropsWithTheme>`
  display: flex;
  flex-direction: column;
  gap: 16px;
  padding: 16px;
  background-color: ${({ theme }) => theme.color.background.base};
`);

const FooterWrapper = styled.div`
  display: flex;
  justify-content: flex-end;
  align-items: center;
  gap: 8px;
  padding: 16px;
`;

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
  const [inputValue, setInputValue] = useState("");

  useEffect(() => {
    if (open) {
      setInputValue("");
    }
  }, [open]);

  const isConfirmDisabled = inputValue !== confirmationPhrase || isPending;

  return (
    <Wrapper>
      <HeaderWrapper>
        <BaseHeader title={title} onClose={onClose} />
      </HeaderWrapper>

      <FlexItem grow={0} shrink={0}>
        <HorizontalDivider />
      </FlexItem>

      <BodyWrapper>
        <Text variant={TextVariant.SECONDARY}>{body}</Text>
        <FlexWrapper direction={FlexDirection.COLUMN} gap={8} fillWidth>
          <Text variant={TextVariant.PRIMARY}>
            Type{" "}
            <Code>
              <Selectable>{confirmationPhrase}</Selectable>
            </Code>{" "}
            to confirm:
          </Text>
          <TextInput
            value={inputValue}
            onChange={setInputValue}
            placeholder={confirmationPhrase}
            fillWidth
          />
        </FlexWrapper>
      </BodyWrapper>

      <FlexItem grow={0} shrink={0}>
        <HorizontalDivider />
      </FlexItem>

      <FooterWrapper>
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
      </FooterWrapper>
    </Wrapper>
  );
};

export default DeleteConfirmDialog;
