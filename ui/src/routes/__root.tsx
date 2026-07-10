import { styled } from "@linaria/react";
import { createRootRoute, Outlet } from "@tanstack/react-router";

import { OverlayProvider } from "@galaxy-io/dls/overlay/OverlayProvider";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

export const Route = createRootRoute({
  component: RootComponent,
});

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

function RootComponent() {
  return (
    <OverlayProvider>
      <RootComponentWrapper>
        <Outlet />
      </RootComponentWrapper>
    </OverlayProvider>
  );
}
