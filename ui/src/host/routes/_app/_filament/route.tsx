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
    productUrls={HOST_RUNTIME_CONFIG.products}
  />
);

export const Route = createFileRoute("/_app/_filament")({
  ...filamentLayoutRouteOptions,
  component: AppFilamentLayout,
});
