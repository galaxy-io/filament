import { useMemo } from "react";

import { create, fromJsonString, toJsonString } from "@bufbuild/protobuf";
import { keepPreviousData } from "@tanstack/react-query";

import { useDebouncedValue } from "@galaxy-io/dls/hooks/useDebouncedValue";

import {
  type EdgeValidation,
  ValidatePipelineRequestSchema,
  type ValidatePipelineResponse,
} from "@/gen/ingestion/v1/capabilities_pb";
import type { ValidationError } from "@/gen/ingestion/v1/connectors_pb";

import {
  getCanvasEdgeKey,
  mapCanvasStateToVersionRequest,
} from "@/pages/pipelines/canvas/graph/serialize";
import { usePipelineCanvasState } from "@/pages/pipelines/canvas/providers/canvas/PipelineCanvasProvider";
import type { CanvasEdge } from "@/pages/pipelines/canvas/types";
import { groupTransformIssuesByStep } from "@/pages/pipelines/components/transform/grammar/paths";
import { getEdgeBlockingRequirements, getEdgeValidationErrors } from "@/pages/pipelines/utils";

import { usePipelineParams } from "@/module/hooks";

import { useValidatePipelineQuery } from "@/api/queries/capabilities";
import { createGetPipelineInput, useSuspenseGetPipelineQuery } from "@/api/queries/pipelines";

import { VALIDATION_DEBOUNCE_MS } from "@/constants";

export enum PipelineCanvasValidationIssueKind {
  TRANSFORM = "transform",
  EDGE = "edge",
  GRAPH = "graph",
}

export interface PipelineCanvasValidationIssue {
  kind: PipelineCanvasValidationIssueKind;
  resource?: string;
  edgeId?: CanvasEdge["id"];
  invalidSteps?: number;
  message: string;
}

interface PipelineCanvasValidation {
  validation: ValidatePipelineResponse | undefined;
  invalidEdgeIds: Set<CanvasEdge["id"]>;
  edgeValidationByEdgeId: Map<CanvasEdge["id"], EdgeValidation>;
  issues: PipelineCanvasValidationIssue[];
  isPending: boolean;
  isError: boolean;
}

const isTransformIssue = (error: ValidationError): boolean =>
  error.field === "transform" || error.field.startsWith("resources[");

const getEdgeValidationKey = (edge: EdgeValidation) =>
  `${edge.fromNode}|${edge.resource}|${edge.toNode}`;

const getCanvasValidationIssues = (
  validation: ValidatePipelineResponse | undefined,
  edgeIdsByKey: Map<string, CanvasEdge["id"]>,
): PipelineCanvasValidationIssue[] => {
  if (!validation) return [];
  const edgeIssues = validation.edges.flatMap<PipelineCanvasValidationIssue>((edge) => {
    const edgeId = edgeIdsByKey.get(getEdgeValidationKey(edge));
    const transformErrors = edge.errors.filter(isTransformIssue);
    return [
      ...(transformErrors.length > 0
        ? [
            {
              kind: PipelineCanvasValidationIssueKind.TRANSFORM,
              resource: edge.resource,
              edgeId,
              invalidSteps: groupTransformIssuesByStep(transformErrors).size,
              message: "Invalid transformation",
            },
          ]
        : []),
      ...edge.errors
        .filter((error) => !isTransformIssue(error))
        .map((error) => ({
          kind: PipelineCanvasValidationIssueKind.EDGE,
          resource: edge.resource,
          edgeId,
          message: error.message,
        })),
      ...getEdgeBlockingRequirements(edge).map((requirement) => ({
        kind: PipelineCanvasValidationIssueKind.EDGE,
        resource: requirement.resource || edge.resource,
        edgeId,
        message: requirement.message,
      })),
    ];
  });
  const graphIssues: PipelineCanvasValidationIssue[] =
    validation.errors.length > 0
      ? [{ kind: PipelineCanvasValidationIssueKind.GRAPH, message: "Invalid pipeline graph" }]
      : [];
  return [...edgeIssues, ...graphIssues];
};

export const usePipelineCanvasValidation = (): PipelineCanvasValidation => {
  const { id } = usePipelineParams();
  const { data: pipelineData } = useSuspenseGetPipelineQuery({
    input: createGetPipelineInput(id),
  });
  const currentVersion = pipelineData.pipeline?.currentVersion;
  const executionMode = pipelineData.pipeline?.executionMode;
  const state = usePipelineCanvasState();

  const requestJson = useMemo(
    () =>
      toJsonString(
        ValidatePipelineRequestSchema,
        create(ValidatePipelineRequestSchema, {
          executionMode,
          graph: mapCanvasStateToVersionRequest(state, id, currentVersion, executionMode).graph,
        }),
      ),
    [state, id, currentVersion, executionMode],
  );
  const debouncedJson = useDebouncedValue(requestJson, VALIDATION_DEBOUNCE_MS);
  const input = useMemo(
    () => fromJsonString(ValidatePipelineRequestSchema, debouncedJson),
    [debouncedJson],
  );
  const { data, isPending, isError, isPlaceholderData } = useValidatePipelineQuery({
    input,
    options: { placeholderData: keepPreviousData },
  });
  const isSettled = debouncedJson === requestJson && !isPlaceholderData;

  return useMemo(() => {
    const edgeIdsByKey = new Map(state.edges.map((edge) => [getCanvasEdgeKey(edge), edge.id]));
    const invalidEdgeIds = new Set<CanvasEdge["id"]>();
    const edgeValidationByEdgeId = new Map<CanvasEdge["id"], EdgeValidation>();
    for (const edge of data?.edges ?? []) {
      const edgeId = edgeIdsByKey.get(getEdgeValidationKey(edge));
      if (edgeId === undefined) continue;
      edgeValidationByEdgeId.set(edgeId, edge);
      if (getEdgeValidationErrors(edge).length > 0) invalidEdgeIds.add(edgeId);
    }
    return {
      validation: data,
      invalidEdgeIds,
      edgeValidationByEdgeId,
      issues: getCanvasValidationIssues(data, edgeIdsByKey),
      isPending: isPending || !isSettled,
      isError,
    };
  }, [data, isPending, isError, isSettled, state.edges]);
};
