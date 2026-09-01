import { BugIcon } from "@phosphor-icons/react";
import { createFileRoute, redirect, useNavigate } from "@tanstack/react-router";

import Button from "@galaxy-io/dls/buttons/Button";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";

import ErrorLayout from "@/layouts/ErrorLayout";

import { completeSignIn } from "@/auth/oidc";

const AuthCallbackErrorComponent = ({ error }: { error: Error }) => {
  const navigate = useNavigate();

  return (
    <ErrorLayout
      icon={<Icon component={BugIcon} size={24} variant={IconVariant.ERROR} />}
      header="Sign-in failed"
      message="Please try again"
      error={error}
      actions={<Button label="Sign in again" onClick={() => void navigate({ to: "/" })} />}
    />
  );
};

export const Route = createFileRoute("/auth/callback")({
  pendingMs: 0,
  pendingMinMs: 0,
  beforeLoad: ({ context }) => {
    if (!context.authConfig.issuer) {
      throw redirect({ to: "/" });
    }
  },
  loader: async () => {
    const user = await completeSignIn();
    const returnTo = (user?.state as { returnTo?: string } | undefined)?.returnTo;
    if (returnTo?.startsWith("/") && !returnTo.startsWith("//")) {
      throw redirect({ href: returnTo, replace: true });
    }
    throw redirect({ to: "/", replace: true });
  },
  errorComponent: AuthCallbackErrorComponent,
});
