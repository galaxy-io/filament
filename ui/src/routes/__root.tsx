import { styled } from "@linaria/react";
import { BugIcon } from "@phosphor-icons/react";
import { createRootRoute, Outlet, useRouter } from "@tanstack/react-router";

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
  return (
    <ToastProvider>
      <OverlayProvider>
        <RootComponentWrapper>
          <Outlet />
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
    if (authConfig.issuer) {
      initOidc(authConfig);
    }
    return { authConfig };
  },
  errorComponent: RootErrorComponent,
  component: RootComponent,
});
