import { useMemo } from "react";

import { create, fromJsonString, toJsonString } from "@bufbuild/protobuf";
import { keepPreviousData } from "@tanstack/react-query";
import { useParams } from "@tanstack/react-router";

import { useDebouncedValue } from "@galaxy-io/dls/inputs/hooks";

import {
  type EdgeValidation,
  ValidatePipelineRequestSchema,
  type ValidatePipelineResponse,
} from "@/gen/ingestion/v1/capabilities_pb";
import type { ValidationError } from "@/gen/ingestion/v1/connectors_pb";
import { GetPipelineRequestSchema } from "@/gen/ingestion/v1/pipelines_pb";

import { PIPELINE_CANVAS_VALIDATION_DEBOUNCE_MS } from "@/pages/pipelines/canvas/constants";
import {
  getCanvasEdgeKey,
  mapCanvasStateToVersionRequest,
} from "@/pages/pipelines/canvas/graph/serialize";
import { usePipelineCanvasState } from "@/pages/pipelines/canvas/providers/canvas/PipelineCanvasProvider";
import type { CanvasEdge } from "@/pages/pipelines/canvas/types";
import { groupTransformIssuesByStep } from "@/pages/pipelines/components/transform/grammar/paths";
import { getPipelineValidationErrors } from "@/pages/pipelines/utils";

import { useValidatePipelineQuery } from "@/api/queries/capabilities";
import { useSuspenseGetPipelineQuery } from "@/api/queries/pipelines";

export interface PipelineCanvasValidationIssue {
  // Set when a resource's transformation no longer compiles; the builder
  // holds the step-level messages.
  resource?: string;
  // The canvas edge carrying that resource, so the issue can open its panel.
  edgeId?: CanvasEdge["id"];
  // Steps that failed to compile on that resource.
  invalidSteps?: number;
  message: string;
}

export interface PipelineCanvasValidation {
  validation: ValidatePipelineResponse | undefined;
  invalidEdgeIds: Set<CanvasEdge["id"]>;
  issues: PipelineCanvasValidationIssue[];
  isPending: boolean;
  // The request itself failed, so nothing is known about the canvas.
  isError: boolean;
}

// Transform issues are keyed by their path in the definition; every other
// edge error names the lever it concerns.
const isTransformIssue = (error: ValidationError): boolean =>
  error.field === "transform" || error.field.startsWith("resources[");

const getEdgeValidationKey = (edge: EdgeValidation) =>
  `${edge.fromNode}|${edge.resource}|${edge.toNode}`;

const getCanvasValidationIssues = (
  validation: ValidatePipelineResponse | undefined,
  edgeIdsByKey: Map<string, CanvasEdge["id"]>,
): PipelineCanvasValidationIssue[] => {
  if (!validation) return [];
  const transformIssues = validation.edges
    .filter((edge) => edge.errors.some(isTransformIssue))
    .map((edge) => ({
      resource: edge.resource,
      edgeId: edgeIdsByKey.get(getEdgeValidationKey(edge)),
      invalidSteps: groupTransformIssuesByStep(edge.errors.filter(isTransformIssue)).size,
      message: "Invalid transformation",
    }));
  // Graph-level problems collapse to one row; the canvas itself shows what
  // is missing.
  const graphIssues = validation.errors.length > 0 ? [{ message: "Invalid pipeline graph" }] : [];
  const rest = getPipelineValidationErrors({
    ...validation,
    errors: [],
    edges: validation.edges.map((edge) => ({
      ...edge,
      errors: edge.errors.filter((error) => !isTransformIssue(error)),
    })),
  }).map((message) => ({ message }));
  return [...transformIssues, ...graphIssues, ...rest];
};

// Validates the canvas as it would be saved, so Save and the edge styling
// agree with the server on what is invalid. The request is keyed by content,
// so moving nodes never refetches.
export const usePipelineCanvasValidation = (): PipelineCanvasValidation => {
  const { id } = useParams({ from: "/_app/pipelines/$id" });
  const { data: pipelineData } = useSuspenseGetPipelineQuery({
    input: create(GetPipelineRequestSchema, { id, includeVersions: true }),
  });
  const currentVersion = pipelineData.pipeline?.currentVersion;
  const state = usePipelineCanvasState();

  const requestJson = useMemo(
    () =>
      toJsonString(
        ValidatePipelineRequestSchema,
        create(ValidatePipelineRequestSchema, {
          graph: mapCanvasStateToVersionRequest(state, id, currentVersion).graph,
        }),
      ),
    [state, id, currentVersion],
  );
  const debouncedJson = useDebouncedValue(requestJson, PIPELINE_CANVAS_VALIDATION_DEBOUNCE_MS);
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
    for (const edge of data?.edges ?? []) {
      const edgeId = edgeIdsByKey.get(getEdgeValidationKey(edge));
      if (edgeId !== undefined && edge.errors.some(isTransformIssue)) invalidEdgeIds.add(edgeId);
    }
    return {
      validation: data,
      invalidEdgeIds,
      issues: getCanvasValidationIssues(data, edgeIdsByKey),
      isPending: isPending || !isSettled,
      isError,
    };
  }, [data, isPending, isError, isSettled, state.edges]);
};
