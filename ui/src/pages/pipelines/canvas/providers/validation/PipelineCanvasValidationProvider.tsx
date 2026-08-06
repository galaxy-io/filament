import {
  createContext,
  type PropsWithChildren,
  useContext,
  useMemo,
  useState,
  useSyncExternalStore,
} from "react";

import { keepPreviousData } from "@tanstack/react-query";

import type { EdgeValidation } from "@/gen/ingestion/v1/capabilities_pb";
import type { PipelineVersion } from "@/gen/ingestion/v1/pipelines_pb";

import {
  buildValidatePipelineRequest,
  getCanvasEdgeKey,
  getProtoEdgeKey,
} from "@/pages/pipelines/canvas/graph/serialize";
import {
  getPipelineCanvasValidationTick,
  subscribePipelineCanvasValidation,
} from "@/pages/pipelines/canvas/graph/validationClock";
import {
  usePipelineCanvasReadOnly,
  usePipelineCanvasState,
} from "@/pages/pipelines/canvas/providers/canvas/PipelineCanvasProvider";
import type { CanvasEdge } from "@/pages/pipelines/canvas/types";

import { useValidatePipelineQuery } from "@/api/queries/pipelines";

interface PipelineCanvasValidation {
  getEdgeValidation: (edge: CanvasEdge) => EdgeValidation | undefined;
  isStale: boolean;
}

const PipelineCanvasValidationContext = createContext<PipelineCanvasValidation | null>(null);
PipelineCanvasValidationContext.displayName = "PipelineCanvasValidationContext";

export const usePipelineCanvasValidation = () => {
  const validation = useContext(PipelineCanvasValidationContext);
  if (!validation) {
    throw new Error(
      "usePipelineCanvasValidation must be used within PipelineCanvasValidationProvider",
    );
  }
  return validation;
};

interface PipelineCanvasValidationProviderProps {
  baseVersion: PipelineVersion | undefined;
}

const PipelineCanvasValidationProvider = ({
  baseVersion,
  children,
}: PropsWithChildren<PipelineCanvasValidationProviderProps>) => {
  const state = usePipelineCanvasState();
  const isReadOnly = usePipelineCanvasReadOnly();

  const tick = useSyncExternalStore(
    subscribePipelineCanvasValidation,
    getPipelineCanvasValidationTick,
  );

  const [committed, setCommitted] = useState(() => ({
    tick,
    request: buildValidatePipelineRequest(state, baseVersion),
  }));
  if (committed.tick !== tick) {
    setCommitted({ tick, request: buildValidatePipelineRequest(state, baseVersion) });
  }

  const { data, isPlaceholderData } = useValidatePipelineQuery({
    input: committed.request,
    options: {
      enabled: !isReadOnly && committed.request.edges.length > 0,
      retry: false,
      networkMode: "always",
      refetchOnWindowFocus: false,
      placeholderData: keepPreviousData,
    },
  });

  const value = useMemo(() => {
    const byKey = new Map((data?.edges ?? []).map((edge) => [getProtoEdgeKey(edge), edge]));
    return {
      getEdgeValidation: (edge: CanvasEdge) => byKey.get(getCanvasEdgeKey(edge)),
      isStale: isPlaceholderData,
    };
  }, [data?.edges, isPlaceholderData]);

  return (
    <PipelineCanvasValidationContext.Provider value={value}>
      {children}
    </PipelineCanvasValidationContext.Provider>
  );
};

export default PipelineCanvasValidationProvider;
