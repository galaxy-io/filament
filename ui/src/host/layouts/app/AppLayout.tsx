import type { FC } from "react";

import { Outlet } from "@tanstack/react-router";

import SettingsPage from "@/pages/settings/SettingsPage";

const AppLayout: FC = () => (
  <>
    <Outlet />
    <SettingsPage />
  </>
);

export default AppLayout;
