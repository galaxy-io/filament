import { useState } from "react";

import PasswordInput from "@galaxy-io/dls/inputs/PasswordInput";

import { useAcceptInviteMutation } from "@/api/mutations/auth";

import AuthCard from "@/auth/AuthCard";
import { decodeInviteToken } from "@/auth/inviteToken";
import { getErrorMessage } from "@/utils/errors";

// InvitePage redeems an invite link: the invited teammate picks a password
// and then signs in through the normal flow.
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
    <AuthCard
      subtitle="You have been invited — set a password to join"
      error={error}
      submitLabel="Join organization"
      pendingLabel="Joining..."
      isPending={isPending}
      onSubmit={handleSubmit}
    >
      <PasswordInput label="Password" value={password} onChange={setPassword} fillWidth autoFocus />
      <PasswordInput label="Confirm password" value={confirm} onChange={setConfirm} fillWidth />
    </AuthCard>
  );
};

export default InvitePage;
