import type { FC } from "react";

import { createFileRoute } from "@tanstack/react-router";

import FilamentLayout from "@/module/FilamentLayout";
import { filamentLayoutRouteOptions } from "@/module/routes";

import AppLayoutAccountMenu from "@/host/layouts/app/AppLayoutAccountMenu";

const AppFilamentLayout: FC = () => <FilamentLayout sidebarFooter={<AppLayoutAccountMenu />} />;

export const Route = createFileRoute("/_app/_filament")({
  ...filamentLayoutRouteOptions,
  component: AppFilamentLayout,
});
