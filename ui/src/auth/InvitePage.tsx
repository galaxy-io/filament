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

import { useAcceptInviteMutation } from "@/api/mutations/auth";

import { decodeInviteToken } from "@/auth/inviteToken";
import { Card, Page } from "@/auth/LoginPage";
import { getErrorMessage } from "@/utils/errors";

// InvitePage redeems an invite link (/invite?userId=..&code=..): the invited
// teammate picks a password and then signs in through the normal flow.
const InvitePage = () => {
  // The link is /invite/<token> where token packs "userId:code".
  const token = window.location.pathname.replace(/^\/invite\/?/, "");
  const invite = decodeInviteToken(token);
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [error, setError] = useState<string | undefined>(undefined);
  const { mutate: acceptInvite, isPending } = useAcceptInviteMutation();

  const handleSubmit = () => {
    if (isPending || !invite) {
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
    setError(undefined);
    acceptInvite(
      { userId: invite.userId, code: invite.code, password },
      {
        onSuccess: () => {
          window.location.replace("/");
        },
        onError: (err) => {
          setError(getErrorMessage(err, "Something went wrong, please try again"));
        },
      },
    );
  };

  if (!invite) {
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
            label={isPending ? "Joining..." : "Join organization"}
            onClick={handleSubmit}
            isDisabled={isPending}
            fillWidth
          />
        </FlexWrapper>
      </Card>
    </Page>
  );
};

export default InvitePage;
