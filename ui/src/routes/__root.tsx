import { useCallback } from "react";

import { styled } from "@linaria/react";
import { BugIcon } from "@phosphor-icons/react";
import { createRootRoute, Outlet, useRouter } from "@tanstack/react-router";
import type { User } from "oidc-client-ts";
import { AuthProvider } from "react-oidc-context";

import Button from "@galaxy-io/dls/buttons/Button";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import { OverlayProvider } from "@galaxy-io/dls/overlay/OverlayProvider";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";
import { ToastProvider } from "@galaxy-io/dls/toast/ToastProvider";

import ErrorLayout from "@/layouts/ErrorLayout";

import { createGetAuthConfigQueryOptions } from "@/api/queries/auth";
import { queryClient } from "@/api/queryClient";
import { transport } from "@/api/transport";

import { initOidc } from "@/auth/oidc";
import { AppSessionProvider } from "@/auth/session";
import { resolveReturnTo } from "@/auth/utils";

const RootComponentWrapper = withTheme(styled.div<PropsWithTheme>`
  display: flex;
  flex-direction: column;

  height: 100vh;
  width: 100vw;
  background-color: ${({ theme }) => theme.color.background.base};

  ::selection {
    background-color: ${({ theme }) => theme.color.background.blue};
  }
`);

const RootErrorComponent = ({ error }: { error: Error }) => {
  const router = useRouter();

  return (
    <ErrorLayout
      icon={<Icon component={BugIcon} size={24} variant={IconVariant.ERROR} />}
      header="Could not reach the server"
      message="Please try again later"
      error={error}
      actions={<Button label="Retry" onClick={() => void router.invalidate()} />}
    />
  );
};

const RootComponent = () => {
  const { userManager } = Route.useRouteContext();
  const router = useRouter();

  const handleSigninCallback = useCallback(
    (user: User | undefined) => {
      void router.navigate({ href: resolveReturnTo(user), replace: true });
    },
    [router],
  );

  return (
    <ToastProvider>
      <OverlayProvider>
        <RootComponentWrapper>
          {userManager ? (
            <AuthProvider userManager={userManager} onSigninCallback={handleSigninCallback}>
              <AppSessionProvider>
                <Outlet />
              </AppSessionProvider>
            </AuthProvider>
          ) : (
            <Outlet />
          )}
        </RootComponentWrapper>
      </OverlayProvider>
    </ToastProvider>
  );
};

export const Route = createRootRoute({
  beforeLoad: async () => {
    const authConfig = await queryClient.ensureQueryData(
      createGetAuthConfigQueryOptions({ transport }),
    );
    const userManager = authConfig.issuer ? initOidc(authConfig) : null;
    return { authConfig, userManager };
  },
  errorComponent: RootErrorComponent,
  component: RootComponent,
});
