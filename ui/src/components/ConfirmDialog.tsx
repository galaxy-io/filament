import { styled } from "@linaria/react";
import { WarningCircleIcon } from "@phosphor-icons/react";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, {
  AlignItems,
  FlexDirection,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import Paragraph from "@galaxy-io/dls/text/Paragraph";
import Text, { TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import BaseHeader from "@/layouts/components/BaseHeader";

interface ConfirmDialogProps {
  title: string;
  description?: string;
  bodyTitle?: string;
  body: string;
  confirmLabel: string;
  onClose: () => void;
  onConfirm: () => void;
  isPending?: boolean;
  confirmVariant?: ButtonVariant;
}

const ConfirmDialogWrapper = withTheme(styled.div<PropsWithTheme>`
  display: flex;
  flex-direction: column;
  width: 540px;
  max-width: calc(100vw - 32px);
  background-color: ${({ theme }) => theme.color.background.primary};
  border: 0.5px solid ${({ theme }) => theme.color.border.primary};
  border-radius: 8px;
  overflow: hidden;
`);

const ConfirmDialogBody = withTheme(styled.div<PropsWithTheme>`
  padding: 16px;
  background-color: ${({ theme }) => theme.color.background.base};
`);

const WarningPanel = withTheme(styled.div<PropsWithTheme>`
  display: flex;
  align-items: flex-start;
  gap: 12px;
  padding: 12px;
  background-color: ${({ theme }) => theme.color.background.secondary};
  border: 0.5px solid ${({ theme }) => theme.color.border.primary};
  border-radius: 6px;
`);

const ConfirmDialog = ({
  title,
  description,
  bodyTitle,
  body,
  confirmLabel,
  onClose,
  onConfirm,
  isPending = false,
  confirmVariant = ButtonVariant.PRIMARY,
}: ConfirmDialogProps) => (
  <ConfirmDialogWrapper>
    <FlexWrapper padding="16px">
      <BaseHeader title={title} description={description} onClose={onClose} />
    </FlexWrapper>
    <FlexItem grow={0} shrink={0}>
      <HorizontalDivider />
    </FlexItem>
    <ConfirmDialogBody>
      <WarningPanel>
        <FlexItem grow={0} shrink={0} display="flex">
          <Icon component={WarningCircleIcon} size={18} variant={IconVariant.ERROR} />
        </FlexItem>
        <FlexWrapper direction={FlexDirection.COLUMN} gap={4} minWidth={0}>
          {bodyTitle && <Text weight={TextWeight.MEDIUM}>{bodyTitle}</Text>}
          <Paragraph variant={TextVariant.SECONDARY}>{body}</Paragraph>
        </FlexWrapper>
      </WarningPanel>
    </ConfirmDialogBody>
    <FlexItem grow={0} shrink={0}>
      <HorizontalDivider />
    </FlexItem>
    <FlexWrapper
      justifyContent={JustifyContent.END}
      alignItems={AlignItems.CENTER}
      padding="16px"
      gap={8}
      fillWidth
    >
      <Button
        label="Cancel"
        variant={ButtonVariant.SECONDARY}
        onClick={onClose}
        isDisabled={isPending}
      />
      <Button
        label={confirmLabel}
        variant={confirmVariant}
        onClick={onConfirm}
        isLoading={isPending}
      />
    </FlexWrapper>
  </ConfirmDialogWrapper>
);

export default ConfirmDialog;
