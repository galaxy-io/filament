import { createFileRoute, Outlet } from "@tanstack/react-router";

import MainLayout from "@/layouts/main/MainLayout";

export const Route = createFileRoute("/_app/_main")({
  component: () => (
    <MainLayout>
      <Outlet />
    </MainLayout>
  ),
});
