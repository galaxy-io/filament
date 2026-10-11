import { type FC, useState } from "react";

import { create } from "@bufbuild/protobuf";
import { useTransport } from "@connectrpc/connect-query";
import { LinkBreakIcon } from "@phosphor-icons/react";
import { useQueryClient } from "@tanstack/react-query";
import { useNavigate, useParams } from "@tanstack/react-router";

import { InputSize } from "@galaxy-io/dls/inputs/Input";
import PasswordInput from "@galaxy-io/dls/inputs/PasswordInput";
import { ToastVariant } from "@galaxy-io/dls/toast/Toast";
import { useToast } from "@galaxy-io/dls/toast/useToast";

import { type AcceptInviteRequest, AcceptInviteRequestSchema } from "@/gen/auth/v1/session_pb";

import { decodeInviteToken } from "@/pages/settings/utils";

import AuthForm from "@/host/layouts/auth/AuthForm";

import { createGetSessionQueryOptions, useAcceptInviteMutation } from "@/api/queries/auth";

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

const InvitePage: FC = () => {
  const { token } = useParams({ from: "/invite/$token" });
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const transport = useTransport();
  const { toast } = useToast();
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
        onSubmit={() => void navigate({ to: "/login" })}
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
      create(AcceptInviteRequestSchema, {
        userId: invite.userId,
        code: invite.code,
        password: state.password,
      }),
      {
        onSuccess: () => {
          queryClient.removeQueries({
            queryKey: createGetSessionQueryOptions({ transport }).queryKey,
          });
          toast({ header: "Invite accepted", variant: ToastVariant.SUCCESS });
          void navigate({ to: "/login", replace: true });
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
