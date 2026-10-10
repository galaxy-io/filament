import type { FC } from "react";

import { TransportProvider } from "@connectrpc/connect-query";
import { createConnectTransport } from "@connectrpc/connect-web";
import {
  createRootRoute,
  createRoute,
  createRouter,
  Link,
  lazyRouteComponent,
  linkOptions,
  Outlet,
} from "@tanstack/react-router";

import FilamentLayout from "@galaxy-io/filament/FilamentLayout";
import {
  filamentLayoutRouteOptions,
  filamentNotFoundRouteOptions,
  pipelinesRouteOptions,
  sinksRouteOptions,
  sourcesRouteOptions,
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

const pipelinesRoute = createRoute({
  getParentRoute: () => mountRoute,
  path: "pipelines",
  ...pipelinesRouteOptions,
  component: lazyRouteComponent(() => import("@galaxy-io/filament/pages/PipelinesPage")),
});

const sourcesRoute = createRoute({
  getParentRoute: () => mountRoute,
  path: "sources",
  ...sourcesRouteOptions,
  component: lazyRouteComponent(() => import("@galaxy-io/filament/pages/SourcesPage")),
});

const sinksRoute = createRoute({
  getParentRoute: () => mountRoute,
  path: "sinks",
  ...sinksRouteOptions,
  component: lazyRouteComponent(() => import("@galaxy-io/filament/pages/SinksPage")),
});

const routeTree = rootRoute.addChildren([
  homeRoute,
  mountRoute.addChildren([pipelinesRoute, sourcesRoute, sinksRoute, notFoundRoute]),
]);

export const router = createRouter({ routeTree });

export const unknownSearchKeyLink = linkOptions({
  to: `/${MOUNT_PATH}/pipelines`,
  // @ts-expect-error a module route rejects search keys its schema does not declare
  search: { nope: 1 },
});

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}
