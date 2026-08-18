import type { PropsWithChildren, ReactNode } from "react";

import { styled } from "@linaria/react";

import Button from "@galaxy-io/dls/buttons/Button";
import FlexWrapper, {
  AlignItems,
  FlexDirection,
  FlexGap,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import GalaxyFilamentWordmark from "@galaxy-io/dls/icons/GalaxyFilamentWordmark";
import GalaxyLogomark from "@galaxy-io/dls/icons/GalaxyLogomark";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

const Page = withTheme(styled.div<PropsWithTheme>`
  position: relative;
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background-color: ${({ theme }) => theme.color.background.primary};
`);

const Card = withTheme(styled.div<PropsWithTheme>`
  width: 360px;
  padding: 32px;
  display: flex;
  flex-direction: column;
  gap: 24px;
  background-color: ${({ theme }) => theme.color.background.tertiary};
  border: 0.5px solid ${({ theme }) => theme.color.border.primary};
  border-radius: 8px;
`);

const LinkText = styled.a`
  cursor: pointer;
  text-decoration: none;

  &:hover {
    text-decoration: underline;
  }
`;

export interface AuthCardProps {
  /** Sits under the wordmark, describing what this screen is for. */
  subtitle: string;
  /** Inline failure, cleared and set by the page's own submit handler. */
  error?: string;
  submitLabel: string;
  pendingLabel: string;
  isPending: boolean;
  onSubmit: () => void;
  /** Optional cross-link rendered below the button. */
  footer?: ReactNode;
}

// AuthCard is the shell every signed-out screen shares: the brand lockup, a
// field stack, the inline error, and a full-width submit that Enter also
// triggers. Pages supply their fields as children and own their submit.
const AuthCard = ({
  subtitle,
  error,
  submitLabel,
  pendingLabel,
  isPending,
  onSubmit,
  footer,
  children,
}: PropsWithChildren<AuthCardProps>) => {
  return (
    <Page>
      <Card
        onKeyDown={(e: React.KeyboardEvent) => {
          if (e.key === "Enter") {
            onSubmit();
          }
        }}
      >
        <FlexWrapper
          direction={FlexDirection.COLUMN}
          alignItems={AlignItems.CENTER}
          gap={FlexGap.MEDIUM}
        >
          <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.MEDIUM}>
            <GalaxyLogomark height={14} />
            <GalaxyFilamentWordmark height={22} />
          </FlexWrapper>
          <Text size={TextSize.BODY_MD} variant={TextVariant.SECONDARY}>
            {subtitle}
          </Text>
        </FlexWrapper>
        <FlexWrapper direction={FlexDirection.COLUMN} gap={FlexGap.MEDIUM} fillWidth>
          {children}
          {error && (
            <Text size={TextSize.CAPTION} variant={TextVariant.ERROR}>
              {error}
            </Text>
          )}
          <Button
            label={isPending ? pendingLabel : submitLabel}
            onClick={onSubmit}
            isDisabled={isPending}
            fillWidth
          />
        </FlexWrapper>
        {footer}
      </Card>
    </Page>
  );
};

// AuthCardFooter centers a prompt beside a link, the shape every
// cross-navigation between these screens takes.
export const AuthCardFooter = ({
  prompt,
  href,
  label,
}: {
  prompt: string;
  href: string;
  label: string;
}) => (
  <FlexWrapper
    alignItems={AlignItems.CENTER}
    justifyContent={JustifyContent.CENTER}
    gap={FlexGap.SMALL}
    fillWidth
  >
    <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY}>
      {prompt}
    </Text>
    <LinkText href={href}>
      <Text size={TextSize.BODY_SM} variant={TextVariant.BLUE}>
        {label}
      </Text>
    </LinkText>
  </FlexWrapper>
);

export default AuthCard;
