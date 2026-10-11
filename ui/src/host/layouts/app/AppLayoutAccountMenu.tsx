import type { FC } from "react";

import { useRouteContext } from "@tanstack/react-router";

import AppLayoutAccountMenuContent from "@/host/layouts/app/AppLayoutAccountMenuContent";

const AppLayoutAccountMenu: FC = () => {
  const { session } = useRouteContext({ from: "/_app" });

  if (!session.isAuthenticated) {
    return null;
  }

  return <AppLayoutAccountMenuContent />;
};

export default AppLayoutAccountMenu;
