import type { FC } from "react";

import type { ErrorComponentProps } from "@tanstack/react-router";

import PipelineNotFoundPage from "@/pages/pipelines/PipelineNotFoundPage";

import FilamentErrorComponent from "@/module/FilamentErrorComponent";
import FilamentPendingComponent from "@/module/FilamentPendingComponent";
import FilamentPipelineErrorComponent from "@/module/FilamentPipelineErrorComponent";
import {
  connectionsSearchSchema,
  filamentLayoutSearchSchema,
  observabilitySearchSchema,
  pipelineCanvasSearchSchema,
  pipelineHistorySearchSchema,
  pipelineSearchSchema,
  pipelinesSearchSchema,
} from "@/module/schemas";

interface FilamentRouteComponents {
  errorComponent: FC<ErrorComponentProps>;
  pendingComponent: FC;
}

const createFilamentRouteOptions = <TOptions extends object>(
  options: TOptions,
): FilamentRouteComponents & TOptions => ({
  errorComponent: FilamentErrorComponent,
  pendingComponent: FilamentPendingComponent,
  ...options,
});

const connectionsRouteOptions = {
  validateSearch: connectionsSearchSchema,
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
});

export const sourcesRouteOptions = createFilamentRouteOptions(connectionsRouteOptions);

export const sinksRouteOptions = createFilamentRouteOptions(connectionsRouteOptions);

export const pipelineRouteOptions = createFilamentRouteOptions({
  validateSearch: pipelineSearchSchema,
  errorComponent: FilamentPipelineErrorComponent,
  notFoundComponent: PipelineNotFoundPage,
});

export const pipelineCanvasRouteOptions = createFilamentRouteOptions({
  validateSearch: pipelineCanvasSearchSchema,
});

export const pipelineHistoryRouteOptions = createFilamentRouteOptions({
  validateSearch: pipelineHistorySearchSchema,
});

export const pipelineSettingsRouteOptions = createFilamentRouteOptions({});
