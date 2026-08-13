import { useState } from "react";

import { Code, ConnectError } from "@connectrpc/connect";
import { styled } from "@linaria/react";

import Button from "@galaxy-io/dls/buttons/Button";
import FlexWrapper, {
  AlignItems,
  FlexDirection,
  FlexGap,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import GalaxyLogomark from "@galaxy-io/dls/icons/GalaxyLogomark";
import PasswordInput from "@galaxy-io/dls/inputs/PasswordInput";
import TextInput from "@galaxy-io/dls/inputs/TextInput";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import FilamentWordmark from "@/components/FilamentWordmark";

import { useLoginMutation } from "@/api/mutations/auth";

export const Page = withTheme(styled.div<PropsWithTheme>`
  position: relative;
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background-color: ${({ theme }) => theme.color.background.primary};
`);

export const LinkText = styled.a`
  cursor: pointer;

  &:hover {
    text-decoration: underline;
  }
`;

export const Card = withTheme(styled.div<PropsWithTheme>`
  width: 360px;
  padding: 32px;
  display: flex;
  flex-direction: column;
  gap: 24px;
  background-color: ${({ theme }) => theme.color.background.tertiary};
  border: 0.5px solid ${({ theme }) => theme.color.border.primary};
  border-radius: 8px;
`);

// LoginPage is filament's own credential screen. Zitadel's authorize endpoint
// redirects here with ?authRequest=<id>; the server proxy verifies the
// credentials and finalizes the request, returning the OIDC callback URL that
// completes the login.
const LoginPage = () => {
  const authRequestId = new URLSearchParams(window.location.search).get("authRequest") ?? "";
  const [loginName, setLoginName] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | undefined>(undefined);
  const { mutate: login, isPending } = useLoginMutation();

  const handleSubmit = () => {
    if (isPending) {
      return;
    }
    if (loginName === "" || password === "") {
      setError("Enter your email and password");
      return;
    }
    setError(undefined);
    login(
      { authRequestId, loginName, password },
      {
        onSuccess: ({ callbackUrl }) => {
          window.location.href = callbackUrl;
        },
        onError: (err) => {
          setError(
            ConnectError.from(err).code === Code.Unauthenticated
              ? "Incorrect email or password"
              : "Sign-in failed, please try again",
          );
        },
      },
    );
  };

  if (!authRequestId) {
    // Reached directly instead of via the authorize redirect; restart the
    // flow from the app so Zitadel issues a fresh auth request.
    window.location.replace("/");
    return null;
  }

  return (
    <Page>
      <Card
        onKeyDown={(e: React.KeyboardEvent) => {
          if (e.key === "Enter") {
            handleSubmit();
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
            <FilamentWordmark height={22} />
          </FlexWrapper>
          <Text size={TextSize.BODY_MD} variant={TextVariant.SECONDARY}>
            Sign in with your organization account
          </Text>
        </FlexWrapper>
        <FlexWrapper direction={FlexDirection.COLUMN} gap={FlexGap.MEDIUM} fillWidth>
          <TextInput
            label="Email"
            value={loginName}
            onChange={setLoginName}
            placeholder="you@company.com"
            fillWidth
            autoFocus
          />
          <PasswordInput label="Password" value={password} onChange={setPassword} fillWidth />
          {error && (
            <Text size={TextSize.CAPTION} variant={TextVariant.ERROR}>
              {error}
            </Text>
          )}
          <Button
            label={isPending ? "Signing in..." : "Sign in"}
            onClick={handleSubmit}
            isDisabled={isPending}
            fillWidth
          />
        </FlexWrapper>
        <FlexWrapper
          alignItems={AlignItems.CENTER}
          justifyContent={JustifyContent.CENTER}
          gap={FlexGap.SMALL}
          fillWidth
        >
          <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY}>
            New to filament?
          </Text>
          <LinkText href="/register">
            <Text size={TextSize.BODY_SM} variant={TextVariant.BLUE}>
              Create an organization
            </Text>
          </LinkText>
        </FlexWrapper>
      </Card>
    </Page>
  );
};

export default LoginPage;
