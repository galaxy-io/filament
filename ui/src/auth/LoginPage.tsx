import { useState } from "react";

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

import { API_URL } from "@/constants";

export const Page = withTheme(styled.div<PropsWithTheme>`
  position: relative;
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background-color: ${({ theme }) => theme.color.background.primary};
`);

export const LinkText = styled.span`
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
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | undefined>(undefined);

  const handleSubmit = async () => {
    if (pending) {
      return;
    }
    if (loginName === "" || password === "") {
      setError("Enter your email and password");
      return;
    }
    setPending(true);
    setError(undefined);
    try {
      const res = await fetch(`${API_URL}/auth/login`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ authRequestId, loginName, password }),
      });
      if (res.status === 401) {
        setError("Incorrect email or password");
        return;
      }
      if (!res.ok) {
        setError("Sign-in failed, please try again");
        return;
      }
      const { callbackUrl } = (await res.json()) as { callbackUrl: string };
      window.location.href = callbackUrl;
    } catch {
      setError("Sign-in failed, please try again");
    } finally {
      setPending(false);
    }
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
            void handleSubmit();
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
            label={pending ? "Signing in..." : "Sign in"}
            onClick={() => void handleSubmit()}
            isDisabled={pending}
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
          <LinkText
            onClick={() => {
              window.location.href = "/register";
            }}
          >
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
