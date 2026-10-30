import type { FC } from "react";

import { Outlet } from "@tanstack/react-router";

import AppLayoutCommandPalette from "@/host/layouts/app/AppLayoutCommandPalette";

const AppLayout: FC = () => (
  <>
    <Outlet />
    <AppLayoutCommandPalette />
  </>
);

export default AppLayout;
