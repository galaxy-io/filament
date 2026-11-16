import type { FC } from "react";

import { createFileRoute } from "@tanstack/react-router";

import FilamentLayout from "@/module/FilamentLayout";
import { filamentLayoutRouteOptions } from "@/module/routes";

import AppLayoutAccountMenu from "@/host/layouts/app/AppLayoutAccountMenu";
import { getHostRuntimeConfig } from "@/host/utils";

const HOST_RUNTIME_CONFIG = getHostRuntimeConfig();

const AppFilamentLayout: FC = () => (
  <FilamentLayout
    sidebarFooter={<AppLayoutAccountMenu />}
    alchemyUrl={HOST_RUNTIME_CONFIG.alchemyUrl}
  />
);

export const Route = createFileRoute("/_app/_filament")({
  ...filamentLayoutRouteOptions,
  component: AppFilamentLayout,
});
