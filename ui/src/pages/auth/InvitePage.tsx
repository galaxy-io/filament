import { useState } from "react";

import { LinkBreakIcon } from "@phosphor-icons/react";
import { useNavigate, useParams } from "@tanstack/react-router";

import { InputSize } from "@galaxy-io/dls/inputs/Input";
import PasswordInput from "@galaxy-io/dls/inputs/PasswordInput";

import type { AcceptInviteRequest } from "@/gen/auth/v1/session_pb";

import AuthForm from "@/layouts/auth/AuthForm";

import { useAcceptInviteMutation } from "@/api/mutations/auth";

import { decodeInviteToken } from "@/auth/utils";
import { getErrorMessage } from "@/utils/errors";

interface InvitePageState {
  password: AcceptInviteRequest["password"];
  confirm: AcceptInviteRequest["password"];
  error: string | undefined;
}

const DEFAULT_STATE: InvitePageState = {
  password: "",
  confirm: "",
  error: undefined,
};

const InvitePage = () => {
  const { token } = useParams({ from: "/invite/$token" });
  const navigate = useNavigate();
  const [state, setState] = useState<InvitePageState>(DEFAULT_STATE);
  const { mutate: acceptInvite, isPending } = useAcceptInviteMutation();
  const invite = decodeInviteToken(token);

  if (!invite) {
    return (
      <AuthForm
        icon={LinkBreakIcon}
        title="This invite is not valid"
        subtitle="The link has expired or has already been used. Ask an admin to send a new one."
        submitLabel="Go to sign in"
        isPending={false}
        onSubmit={() => void navigate({ to: "/" })}
      />
    );
  }

  const handleSubmit = () => {
    if (state.password === "") {
      setState((prev) => ({ ...prev, error: "Choose a password" }));
      return;
    }
    if (state.password !== state.confirm) {
      setState((prev) => ({ ...prev, error: "Passwords do not match" }));
      return;
    }
    setState((prev) => ({ ...prev, error: undefined }));
    acceptInvite(
      { userId: invite.userId, code: invite.code, password: state.password },
      {
        onSuccess: () => {
          void navigate({ to: "/", replace: true });
        },
        onError: (err) => {
          setState((prev) => ({
            ...prev,
            error: getErrorMessage(err, "Something went wrong, please try again"),
          }));
        },
      },
    );
  };

  return (
    <AuthForm
      title="Set your password"
      subtitle="Choose a password to finish joining your team."
      error={state.error}
      submitLabel="Join organization"
      isPending={isPending}
      onSubmit={handleSubmit}
    >
      <PasswordInput
        label="Password"
        value={state.password}
        onChange={(password) => setState((prev) => ({ ...prev, password }))}
        size={InputSize.LARGE}
        fillWidth
        autoFocus
      />
      <PasswordInput
        label="Confirm password"
        value={state.confirm}
        onChange={(confirm) => setState((prev) => ({ ...prev, confirm }))}
        size={InputSize.LARGE}
        fillWidth
      />
    </AuthForm>
  );
};

export default InvitePage;
