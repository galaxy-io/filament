import { createFileRoute } from "@tanstack/react-router";

import FilamentLayout from "@/module/FilamentLayout";
import { filamentLayoutRouteOptions } from "@/module/routes";

export const Route = createFileRoute("/_app/_filament")({
  ...filamentLayoutRouteOptions,
  component: FilamentLayout,
});
