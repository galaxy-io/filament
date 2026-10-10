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
  redirect,
} from "@tanstack/react-router";

import FilamentLayout from "@galaxy-io/filament/FilamentLayout";
import { FilamentPath } from "@galaxy-io/filament/paths";
import {
  filamentLayoutRouteOptions,
  filamentNotFoundRouteOptions,
  pipelineCanvasRouteOptions,
  pipelineHistoryRouteOptions,
  pipelineRouteOptions,
  pipelineSettingsRouteOptions,
  pipelinesRouteOptions,
  sinksRouteOptions,
  sourcesRouteOptions,
} from "@galaxy-io/filament/routes";

const MOUNT_PATH = "ingest";

const filamentTransport = createConnectTransport({ baseUrl: "" });

const HomePage: FC = () => (
  <>
    <Link to={`/${MOUNT_PATH}`}>Open the module</Link>
    <Link from={`/${MOUNT_PATH}`} to={FilamentPath.PIPELINE_CANVAS} params={{ id: "p" }}>
      Open a pipeline canvas
    </Link>
  </>
);

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

const pipelineRoute = createRoute({
  getParentRoute: () => mountRoute,
  path: "pipelines/$id",
  ...pipelineRouteOptions,
  component: lazyRouteComponent(() => import("@galaxy-io/filament/pages/PipelinePage")),
});

const pipelineIndexRoute = createRoute({
  getParentRoute: () => pipelineRoute,
  path: "/",
  beforeLoad: ({ params }) => {
    throw redirect({ to: `/${MOUNT_PATH}/pipelines/$id/canvas`, params });
  },
});

const pipelineCanvasRoute = createRoute({
  getParentRoute: () => pipelineRoute,
  path: "canvas",
  ...pipelineCanvasRouteOptions,
  component: lazyRouteComponent(() => import("@galaxy-io/filament/pages/PipelineCanvasPage")),
});

const pipelineHistoryRoute = createRoute({
  getParentRoute: () => pipelineRoute,
  path: "history",
  ...pipelineHistoryRouteOptions,
  component: lazyRouteComponent(() => import("@galaxy-io/filament/pages/PipelineHistoryPage")),
});

const pipelineSettingsRoute = createRoute({
  getParentRoute: () => pipelineRoute,
  path: "settings",
  ...pipelineSettingsRouteOptions,
  component: lazyRouteComponent(() => import("@galaxy-io/filament/pages/PipelineSettingsPage")),
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
  mountRoute.addChildren([
    pipelinesRoute,
    pipelineRoute.addChildren([
      pipelineIndexRoute,
      pipelineCanvasRoute,
      pipelineHistoryRoute,
      pipelineSettingsRoute,
    ]),
    sourcesRoute,
    sinksRoute,
    notFoundRoute,
  ]),
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
