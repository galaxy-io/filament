import type { FC, PropsWithChildren, ReactNode } from "react";

import { styled } from "@linaria/react";
import type { Icon as PhosphorIcon } from "@phosphor-icons/react";

import Button, { ButtonSize } from "@galaxy-io/dls/buttons/Button";
import Alert, { AlertVariant } from "@galaxy-io/dls/feedback/Alert";
import Box from "@galaxy-io/dls/layout/Box";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import { t } from "@galaxy-io/dls/theme/tokens/t";

import IconTile from "@/components/IconTile";

import AuthLayout from "@/host/layouts/auth/AuthLayout";

const Form = styled.form`
  width: 100%;

  display: flex;
  flex-direction: column;
  gap: ${t.space[24]};
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

export default AuthForm;
