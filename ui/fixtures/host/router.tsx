import type { FC } from "react";

import { TransportProvider } from "@connectrpc/connect-query";
import { createConnectTransport } from "@connectrpc/connect-web";
import {
  createRootRoute,
  createRoute,
  createRouter,
  Link,
  lazyRouteComponent,
  Outlet,
} from "@tanstack/react-router";

import FilamentLayout from "@galaxy-io/filament/FilamentLayout";
import {
  filamentLayoutRouteOptions,
  filamentNotFoundRouteOptions,
} from "@galaxy-io/filament/routes";

const MOUNT_PATH = "ingest";

const filamentTransport = createConnectTransport({ baseUrl: "" });

const HomePage: FC = () => <Link to={`/${MOUNT_PATH}`}>Open the module</Link>;

const MountLayout: FC = () => (
  <TransportProvider transport={filamentTransport}>
    <FilamentLayout />
  </TransportProvider>
);

const rootRoute = createRootRoute({ component: Outlet });

const homeRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/",
  component: HomePage,
});

const mountRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: MOUNT_PATH,
  ...filamentLayoutRouteOptions,
  component: MountLayout,
});

const notFoundRoute = createRoute({
  getParentRoute: () => mountRoute,
  path: "$",
  ...filamentNotFoundRouteOptions,
  component: lazyRouteComponent(() => import("@galaxy-io/filament/pages/NotFoundPage")),
});

const routeTree = rootRoute.addChildren([homeRoute, mountRoute.addChildren([notFoundRoute])]);

export const router = createRouter({ routeTree });

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}
