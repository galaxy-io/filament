import { styled } from "@linaria/react";
import type { Icon as PhosphorIcon } from "@phosphor-icons/react";
import {
  CheckCircleIcon,
  SpinnerGapIcon,
  WarningIcon,
  XIcon,
} from "@phosphor-icons/react";
import { match } from "ts-pattern";

import Button, {
  ButtonProps,
  ButtonSize,
  ButtonVariant,
} from "@galaxy-io/dls/buttons/Button";
import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, {
  AlignItems,
  FlexDirection,
} from "@galaxy-io/dls/containers/FlexWrapper";
import Icon, { IconVariant, IconWeight } from "@galaxy-io/dls/icons/Icon";
import Text, { TextVariant } from "@galaxy-io/dls/text/Text";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

export enum ToastVariant {
  PRIMARY = "PRIMARY",
  SUCCESS = "SUCCESS",
  WARNING = "WARNING",
  ERROR = "ERROR",
}

export interface ToastProps {
  variant?: ToastVariant;
  icon?: PhosphorIcon;
  header: string | React.ReactNode;
  subheader: string | React.ReactNode;
  actions?: ButtonProps[];
  onClose?: () => void;
}

const StyledToast = withTheme(styled.div<
  PropsWithTheme<{ $variant: ToastVariant; $hasActions: boolean }>
>`
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: ${({ $hasActions }) => ($hasActions ? "12px 12px 8px 12px" : "12px")};

  min-width: 360px;
  max-width: 420px;

  background-color: ${({ theme }) => theme.color.background.tertiary};
  border: 0.5px solid
    ${({ theme, $variant }) =>
      match($variant)
        .with(ToastVariant.PRIMARY, () => theme.color.border.primary)
        .with(ToastVariant.SUCCESS, () => theme.color.border.success)
        .with(ToastVariant.WARNING, () => theme.color.border.primary)
        .with(ToastVariant.ERROR, () => theme.color.border.error)
        .exhaustive()};

  border-radius: 5px;
`);

const Toast = ({
  variant = ToastVariant.PRIMARY,
  icon,
  header,
  subheader,
  actions,
  onClose,
}: ToastProps) => {
  const renderIcon = () => {
    if (icon) {
      return <Icon component={icon} size={16} />;
    }

    return match(variant)
      .with(ToastVariant.PRIMARY, () => <Icon component={SpinnerGapIcon} size={16} />)
      .with(ToastVariant.SUCCESS, () => (
        <Icon
          component={CheckCircleIcon}
          variant={IconVariant.SUCCESS}
          size={16}
          weight={IconWeight.FILL}
        />
      ))
      .with(ToastVariant.WARNING, () => (
        <Icon
          component={WarningIcon}
          variant={IconVariant.WARNING}
          size={16}
          weight={IconWeight.FILL}
        />
      ))
      .with(ToastVariant.ERROR, () => (
        <Icon
          component={WarningIcon}
          variant={IconVariant.ERROR}
          size={16}
          weight={IconWeight.FILL}
        />
      ))
      .exhaustive();
  };

  const hasActions = (actions && actions.length > 0) ?? false;

  return (
    <StyledToast $variant={variant} $hasActions={hasActions}>
      <FlexWrapper alignItems={AlignItems.START} gap={8}>
        <FlexWrapper width={20} height={20}>
          {renderIcon()}
        </FlexWrapper>
        <FlexItem grow={1}>
          <FlexWrapper direction={FlexDirection.COLUMN} gap={4}>
            {typeof header === "string" ? <Text>{header}</Text> : header}
            {typeof subheader === "string" ? (
              <Text variant={TextVariant.TERTIARY}>{subheader}</Text>
            ) : (
              subheader
            )}
          </FlexWrapper>
        </FlexItem>
        {onClose && (
          <Button
            icon={XIcon}
            variant={ButtonVariant.TERTIARY}
            size={ButtonSize.SMALL}
            onClick={onClose}
          />
        )}
      </FlexWrapper>
      {hasActions && (
        <FlexWrapper alignItems={AlignItems.CENTER} gap={8}>
          {actions?.map((action, index) => (
            <Button
              key={index}
              label={action.label}
              variant={action.variant ?? ButtonVariant.TERTIARY}
              size={ButtonSize.SMALL}
              onClick={action.onClick}
            />
          ))}
        </FlexWrapper>
      )}
    </StyledToast>
  );
};

export default Toast;
