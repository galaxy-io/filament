import { createFileRoute, Outlet } from "@tanstack/react-router";

import MainLayout from "@/layouts/main/MainLayout";

import { filamentLayoutRouteOptions } from "@/module/routes";

export const Route = createFileRoute("/_app/_filament")({
  ...filamentLayoutRouteOptions,
  component: () => (
    <MainLayout>
      <Outlet />
    </MainLayout>
  ),
});
