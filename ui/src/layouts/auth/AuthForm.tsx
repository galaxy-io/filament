import type { PropsWithChildren, ReactNode } from "react";

import { styled } from "@linaria/react";
import { type Icon as PhosphorIcon, WarningIcon } from "@phosphor-icons/react";
import { Link, type LinkProps } from "@tanstack/react-router";

import Button, { ButtonSize } from "@galaxy-io/dls/buttons/Button";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import { t } from "@galaxy-io/dls/theme/tokens/t";

import IconTile from "@/components/IconTile";

import AuthLayout from "@/layouts/auth/AuthLayout";

const Form = styled.form`
  width: 100%;

  display: flex;
  flex-direction: column;
  gap: 24px;
`;

const ErrorWrapper = styled.div`
  width: 100%;

  display: flex;
  align-items: center;
  gap: 8px;

  padding: 8px 12px;

  background-color: ${t.color.background.error};
  border: 0.5px solid ${t.color.border.error};
  border-radius: 5px;
`;

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
        <Flex alignItems={AlignItems.START} direction={FlexDirection.COLUMN} gap={12} fillWidth>
          {icon && <IconTile icon={icon} size={28} />}
          <Flex alignItems={AlignItems.START} direction={FlexDirection.COLUMN} gap={8} fillWidth>
            <Text size={TextSize.HEADING_MD} weight={TextWeight.MEDIUM}>
              {title}
            </Text>
            <Text size={TextSize.BODY_MD} variant={TextVariant.SECONDARY}>
              {subtitle}
            </Text>
          </Flex>
        </Flex>
        <Flex alignItems={AlignItems.START} direction={FlexDirection.COLUMN} gap={16} fillWidth>
          {children && (
            <Flex alignItems={AlignItems.START} direction={FlexDirection.COLUMN} gap={12} fillWidth>
              {children}
            </Flex>
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
        </Flex>
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
  <Flex alignItems={AlignItems.CENTER} gap={8} fillWidth>
    <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY}>
      {prompt}
    </Text>
    <FooterLink to={to}>
      <Text size={TextSize.BODY_SM}>{label}</Text>
    </FooterLink>
  </Flex>
);

export default AuthForm;
