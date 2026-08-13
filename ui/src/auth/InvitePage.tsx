import { useState } from "react";

import Button from "@galaxy-io/dls/buttons/Button";
import FlexWrapper, {
  AlignItems,
  FlexDirection,
  FlexGap,
} from "@galaxy-io/dls/containers/FlexWrapper";
import GalaxyLogomark from "@galaxy-io/dls/icons/GalaxyLogomark";
import PasswordInput from "@galaxy-io/dls/inputs/PasswordInput";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";

import FilamentWordmark from "@/components/FilamentWordmark";

import { API_URL } from "@/constants";

import { Card, Page } from "@/auth/LoginPage";

// InvitePage redeems an invite link (/invite?userId=..&code=..): the invited
// teammate picks a password and then signs in through the normal flow.
const InvitePage = () => {
  // The link is /invite/<token> where token packs "userId:code".
  const token = window.location.pathname.replace(/^\/invite\/?/, "");
  let userId = "";
  let code = "";
  try {
    [userId = "", code = ""] = atob(token).split(":");
  } catch {
    // fall through to the redirect below
  }
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | undefined>(undefined);

  const handleSubmit = async () => {
    if (pending) {
      return;
    }
    if (password === "") {
      setError("Choose a password");
      return;
    }
    if (password !== confirm) {
      setError("Passwords do not match");
      return;
    }
    setPending(true);
    setError(undefined);
    try {
      const res = await fetch(`${API_URL}/auth/invite/accept`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ userId, code, password }),
      });
      if (!res.ok) {
        setError(await res.text());
        return;
      }
      window.location.replace("/");
    } catch {
      setError("Something went wrong, please try again");
    } finally {
      setPending(false);
    }
  };

  if (!userId || !code) {
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
            You have been invited — set a password to join
          </Text>
        </FlexWrapper>
        <FlexWrapper direction={FlexDirection.COLUMN} gap={FlexGap.MEDIUM} fillWidth>
          <PasswordInput
            label="Password"
            value={password}
            onChange={setPassword}
            fillWidth
            autoFocus
          />
          <PasswordInput label="Confirm password" value={confirm} onChange={setConfirm} fillWidth />
          {error && (
            <Text size={TextSize.CAPTION} variant={TextVariant.ERROR}>
              {error}
            </Text>
          )}
          <Button
            label={pending ? "Joining..." : "Join organization"}
            onClick={() => void handleSubmit()}
            isDisabled={pending}
            fillWidth
          />
        </FlexWrapper>
      </Card>
    </Page>
  );
};

export default InvitePage;
