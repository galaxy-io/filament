import { BugIcon } from "@phosphor-icons/react";
import { createFileRoute, redirect, useNavigate } from "@tanstack/react-router";
import { hasAuthParams, useAuth } from "react-oidc-context";

import Button from "@galaxy-io/dls/buttons/Button";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";

import ErrorLayout from "@/layouts/ErrorLayout";
import PendingLayout from "@/layouts/PendingLayout";

const AuthCallbackPage = () => {
  const navigate = useNavigate();
  const { error } = useAuth();

  if (!error) {
    return <PendingLayout />;
  }

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
  beforeLoad: ({ context }) => {
    if (!context.userManager || !hasAuthParams()) {
      throw redirect({ to: "/" });
    }
  },
  component: AuthCallbackPage,
});
