import type { FC, PropsWithChildren, ReactNode } from "react";

import { styled } from "@linaria/react";
import type { Icon as PhosphorIcon } from "@phosphor-icons/react";
import type { LinkProps } from "@tanstack/react-router";

import Button, { ButtonSize } from "@galaxy-io/dls/buttons/Button";
import Alert, { AlertVariant } from "@galaxy-io/dls/feedback/Alert";
import Box from "@galaxy-io/dls/layout/Box";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import Link, { LinkUnderline } from "@galaxy-io/dls/links/Link";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";

import IconTile from "@/components/IconTile";
import RouterLink from "@/components/RouterLink";

import AuthLayout from "@/host/layouts/auth/AuthLayout";

const Form = styled.form`
  width: 100%;

  display: flex;
  flex-direction: column;
  gap: 24px;
`;

interface AuthFormProps {
  title: string;
  subtitle: string;
  icon?: PhosphorIcon;
  error?: string;
  submitLabel: string;
  isPending: boolean;
  onSubmit: () => void;
  footer?: ReactNode;
}

const AuthForm: FC<PropsWithChildren<AuthFormProps>> = ({
  title,
  subtitle,
  icon,
  error,
  submitLabel,
  isPending,
  onSubmit,
  footer,
  children,
}) => {
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
            <Box fillWidth>
              <Alert variant={AlertVariant.ERROR}>{error}</Alert>
            </Box>
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

export const AuthFormFooter: FC<AuthFormFooterProps> = ({ prompt, to, label }) => (
  <Flex alignItems={AlignItems.CENTER} gap={8} fillWidth>
    <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY}>
      {prompt}
    </Text>
    <Text size={TextSize.BODY_SM}>
      <Link href={to} as={RouterLink} underline={LinkUnderline.HOVER}>
        {label}
      </Link>
    </Text>
  </Flex>
);

export default AuthForm;
