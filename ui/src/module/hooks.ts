import { useCallback } from "react";

import {
  type UseNavigateResult,
  useMatches,
  useMatchRoute,
  useNavigate,
  useParams,
  useSearch,
} from "@tanstack/react-router";

import type { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import { OBSERVABILITY_RUNS_DEFAULT_STATUSES } from "@/pages/observability/components/runs/constants";
import {
  OBSERVABILITY_DEFAULT_RUNS_VIEW,
  OBSERVABILITY_DEFAULT_TIMEFRAME,
} from "@/pages/observability/constants";

import type { FilamentPath } from "@/module/paths";
import {
  connectionsSearchSchema,
  type FilamentLayoutSearch,
  filamentLayoutSearchSchema,
  listSearchParamsSchema,
  observabilitySearchSchema,
  pipelineCanvasSearchSchema,
  pipelineHistorySearchSchema,
  pipelineParamsSchema,
  pipelineSearchSchema,
  pipelinesSearchSchema,
  teamSearchSchema,
} from "@/module/schemas";
import type { Flow } from "@/module/types";

const ROOT_PATH = "/";

interface FilamentSearchUpdateOptions {
  replace?: boolean;
}

interface FilamentMatchRouteOptions {
  params?: Record<string, string>;
  fuzzy?: boolean;
}

export const useFilamentMountedBase = () =>
  useMatches({
    select: (matches): string | undefined =>
      matches.find((match) => match.staticData.filament)?.fullPath,
  });

export const useFilamentBase = (fallback?: string) =>
  (useFilamentMountedBase() ?? fallback ?? ROOT_PATH) as typeof ROOT_PATH;

export const useFilamentNavigate = (fallback?: string): UseNavigateResult<typeof ROOT_PATH> => {
  const base = useFilamentBase(fallback);
  return useNavigate({ from: base });
};

export const useFilamentSearchUpdate = <TSearch extends object>() => {
  const navigate = useNavigate();
  return useCallback(
    (update: (prev: TSearch) => TSearch, { replace = false }: FilamentSearchUpdateOptions = {}) =>
      navigate({ to: ".", replace, search: update as never }),
    [navigate],
  );
};

export const useFilamentFlowOpen = () => {
  const updateSearch = useFilamentSearchUpdate<FilamentLayoutSearch>();
  return useCallback(
    (flow: Flow, connectorKind?: ConnectorKind) =>
      void updateSearch((prev) => ({ ...prev, connectionId: undefined, flow, connectorKind })),
    [updateSearch],
  );
};

export const useFilamentMatchRoute = () => {
  const base = useFilamentBase();
  const matchRoute = useMatchRoute();
  return useCallback(
    (to: FilamentPath, { params, fuzzy = false }: FilamentMatchRouteOptions = {}) =>
      matchRoute({ from: base, to, params, fuzzy } as Parameters<typeof matchRoute>[0]) !== false,
    [base, matchRoute],
  );
};

export const useFilamentLayoutSearch = () =>
  useSearch({
    strict: false,
    structuralSharing: true,
    select: (search) => filamentLayoutSearchSchema.parse(search),
  });

export const useTeamSearch = () =>
  useSearch({
    strict: false,
    structuralSharing: true,
    select: (search) => teamSearchSchema.parse(search),
  });

export const useListSearch = () =>
  useSearch({
    strict: false,
    structuralSharing: true,
    select: (search) => listSearchParamsSchema.parse(search),
  });

export const useObservabilitySearch = () =>
  useSearch({
    strict: false,
    structuralSharing: true,
    select: (search) => {
      const parsed = observabilitySearchSchema.parse(search);
      return {
        ...parsed,
        timeframe: parsed.timeframe ?? OBSERVABILITY_DEFAULT_TIMEFRAME,
        runs: parsed.runs ?? OBSERVABILITY_DEFAULT_RUNS_VIEW,
        statuses: parsed.statuses ?? OBSERVABILITY_RUNS_DEFAULT_STATUSES,
      };
    },
  });

export const usePipelinesSearch = () =>
  useSearch({
    strict: false,
    structuralSharing: true,
    select: (search) => pipelinesSearchSchema.parse(search),
  });

export const useConnectionsSearch = () =>
  useSearch({
    strict: false,
    structuralSharing: true,
    select: (search) => connectionsSearchSchema.parse(search),
  });

export const usePipelineSearch = () =>
  useSearch({
    strict: false,
    structuralSharing: true,
    select: (search) => pipelineSearchSchema.parse(search),
  });

export const usePipelineCanvasSearch = () =>
  useSearch({
    strict: false,
    structuralSharing: true,
    select: (search) => pipelineCanvasSearchSchema.parse(search),
  });

export const usePipelineHistorySearch = () =>
  useSearch({
    strict: false,
    structuralSharing: true,
    select: (search) => pipelineHistorySearchSchema.parse(search),
  });

export const usePipelineParams = () =>
  useParams({ strict: false, select: (params) => pipelineParamsSchema.parse(params) });
