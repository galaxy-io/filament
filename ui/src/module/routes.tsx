import type { FC } from "react";

import { Code, ConnectError } from "@connectrpc/connect";
import type { ErrorComponentProps } from "@tanstack/react-router";

import PipelineNotFoundPage from "@/pages/pipelines/PipelineNotFoundPage";

import FilamentErrorComponent from "@/module/FilamentErrorComponent";
import FilamentPendingComponent from "@/module/FilamentPendingComponent";
import {
  type ConnectionsSearch,
  connectionsSearchSchema,
  filamentLayoutSearchSchema,
  observabilitySearchSchema,
  type PipelinesSearch,
  pipelineCanvasSearchSchema,
  pipelineHistorySearchSchema,
  pipelineSearchSchema,
  pipelinesSearchSchema,
} from "@/module/schemas";

const createFilamentRouteOptions = <TOptions extends object>(options: TOptions) => ({
  errorComponent: FilamentErrorComponent,
  pendingComponent: FilamentPendingComponent,
  ...options,
});

const PipelineErrorComponent: FC<ErrorComponentProps> = ({ error }) =>
  error instanceof ConnectError && error.code === Code.NotFound ? (
    <PipelineNotFoundPage />
  ) : (
    <FilamentErrorComponent error={error} />
  );

const connectionsRouteOptions = {
  validateSearch: connectionsSearchSchema,
  remountDeps: ({ search }: { search: ConnectionsSearch }) => ({ q: search.q }),
};

export const filamentLayoutRouteOptions = createFilamentRouteOptions({
  validateSearch: filamentLayoutSearchSchema,
  staticData: { filament: true },
});

export const filamentNotFoundRouteOptions = createFilamentRouteOptions({});

export const observabilityRouteOptions = createFilamentRouteOptions({
  validateSearch: observabilitySearchSchema,
});

export const pipelinesRouteOptions = createFilamentRouteOptions({
  validateSearch: pipelinesSearchSchema,
  remountDeps: ({ search }: { search: PipelinesSearch }) => ({
    q: search.q,
    sortBy: search.sortBy,
    sortOrder: search.sortOrder,
  }),
});

export const sourcesRouteOptions = createFilamentRouteOptions(connectionsRouteOptions);

export const sinksRouteOptions = createFilamentRouteOptions(connectionsRouteOptions);

export const pipelineRouteOptions = createFilamentRouteOptions({
  validateSearch: pipelineSearchSchema,
  errorComponent: PipelineErrorComponent,
  notFoundComponent: PipelineNotFoundPage,
});

export const pipelineCanvasRouteOptions = createFilamentRouteOptions({
  validateSearch: pipelineCanvasSearchSchema,
});

export const pipelineHistoryRouteOptions = createFilamentRouteOptions({
  validateSearch: pipelineHistorySearchSchema,
});

export const pipelineSettingsRouteOptions = createFilamentRouteOptions({});
