import type { PropsWithChildren, ReactNode } from "react";

import { styled } from "@linaria/react";
import { type Icon as PhosphorIcon, WarningIcon } from "@phosphor-icons/react";
import { Link, type LinkProps } from "@tanstack/react-router";

import Button, { ButtonSize } from "@galaxy-io/dls/buttons/Button";
import FlexWrapper, {
  AlignItems,
  FlexDirection,
  FlexGap,
} from "@galaxy-io/dls/containers/FlexWrapper";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import AuthLayout from "@/layouts/auth/AuthLayout";

const Form = styled.form`
  width: 100%;

  display: flex;
  flex-direction: column;
  gap: 24px;
`;

const IconWrapper = withTheme(styled.div<PropsWithTheme>`
  width: 28px;
  height: 28px;

  display: flex;
  align-items: center;
  justify-content: center;

  background-color: ${({ theme }) => theme.color.background.primary};
  border: 0.5px solid ${({ theme }) => theme.color.border.primary};
  border-radius: 6px;
`);

const ErrorWrapper = withTheme(styled.div<PropsWithTheme>`
  width: 100%;

  display: flex;
  align-items: center;
  gap: 8px;

  padding: 8px 12px;

  background-color: ${({ theme }) => theme.color.background.error};
  border: 0.5px solid ${({ theme }) => theme.color.border.error};
  border-radius: 5px;
`);

const FooterLink = styled(Link)`
  cursor: pointer;
  text-decoration: none;

  &:hover {
    text-decoration: underline;
  }
`;

export interface AuthFormProps {
  title: string;
  subtitle: string;
  icon?: PhosphorIcon;
  error?: string;
  submitLabel: string;
  isPending: boolean;
  onSubmit: () => void;
  footer?: ReactNode;
}

const AuthForm = ({
  title,
  subtitle,
  icon,
  error,
  submitLabel,
  isPending,
  onSubmit,
  footer,
  children,
}: PropsWithChildren<AuthFormProps>) => {
  const handleSubmit = () => {
    if (isPending) {
      return;
    }
    onSubmit();
  };

  const handleFormSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    handleSubmit();
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key !== "Enter" || !(e.target instanceof HTMLInputElement)) {
      return;
    }
    e.preventDefault();
    handleSubmit();
  };

  return (
    <AuthLayout>
      <Form onSubmit={handleFormSubmit} onKeyDown={handleKeyDown}>
        <FlexWrapper direction={FlexDirection.COLUMN} gap={FlexGap.MEDIUM} fillWidth>
          {icon && (
            <IconWrapper>
              <Icon component={icon} size={14} variant={IconVariant.SECONDARY} />
            </IconWrapper>
          )}
          <FlexWrapper direction={FlexDirection.COLUMN} gap={FlexGap.SMALL} fillWidth>
            <Text size={TextSize.HEADING_MD} weight={TextWeight.MEDIUM}>
              {title}
            </Text>
            <Text size={TextSize.BODY_MD} variant={TextVariant.SECONDARY}>
              {subtitle}
            </Text>
          </FlexWrapper>
        </FlexWrapper>
        <FlexWrapper direction={FlexDirection.COLUMN} gap={FlexGap.LARGE} fillWidth>
          {children && (
            <FlexWrapper direction={FlexDirection.COLUMN} gap={FlexGap.MEDIUM} fillWidth>
              {children}
            </FlexWrapper>
          )}
          {error && (
            <ErrorWrapper>
              <Icon component={WarningIcon} size={14} variant={IconVariant.ERROR} />
              <Text size={TextSize.BODY_SM} variant={TextVariant.ERROR}>
                {error}
              </Text>
            </ErrorWrapper>
          )}
          <Button
            label={submitLabel}
            size={ButtonSize.LARGE}
            onClick={handleSubmit}
            isLoading={isPending}
            fillWidth
          />
        </FlexWrapper>
        {footer}
      </Form>
    </AuthLayout>
  );
};

interface AuthFormFooterProps {
  prompt: string;
  to: LinkProps["to"];
  label: string;
}

export const AuthFormFooter = ({ prompt, to, label }: AuthFormFooterProps) => (
  <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.SMALL} fillWidth>
    <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY}>
      {prompt}
    </Text>
    <FooterLink to={to}>
      <Text size={TextSize.BODY_SM}>{label}</Text>
    </FooterLink>
  </FlexWrapper>
);

export default AuthForm;
