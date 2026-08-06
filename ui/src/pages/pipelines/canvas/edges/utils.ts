import type { SelectInputOption } from "@galaxy-io/dls/inputs/SelectInput";

import type {
  CandidateValue,
  EdgeValidation,
  Requirement,
} from "@/gen/ingestion/v1/capabilities_pb";
import { RequirementKind } from "@/gen/ingestion/v1/capabilities_pb";
import type { IngestionType } from "@/gen/ingestion/v1/common_pb";

import {
  INGESTION_TYPE_TO_LABEL_MAP,
  PIPELINE_CANVAS_EDGE_ALL_INGESTION_TYPES,
} from "@/pages/pipelines/canvas/edges/constants";
import type { CanvasEdge } from "@/pages/pipelines/canvas/types";
import { getPipelineCanvasEdgeData } from "@/pages/pipelines/canvas/utils";

const UNRANKED_CANDIDATE_RANK = Number.MAX_SAFE_INTEGER;

const getResourceValidation = (validation: EdgeValidation | undefined, resource: string) =>
  validation?.resources.find((entry) => entry.resource === resource);

// The server answers per table where it can, per source/sink pair otherwise.
// Before the first verdict lands every type is offered rather than none, so a
// mid-debounce select is never empty.
export const getIngestionTypeOptions = (
  validation: EdgeValidation | undefined,
  resource: string,
  ingestionType: IngestionType,
): SelectInputOption[] => {
  const supported =
    getResourceValidation(validation, resource)?.supportedIngestionTypes ??
    validation?.supportedIngestionTypes;

  const types = supported?.length
    ? PIPELINE_CANVAS_EDGE_ALL_INGESTION_TYPES.filter(
        (candidate) => supported.includes(candidate) || candidate === ingestionType,
      )
    : PIPELINE_CANVAS_EDGE_ALL_INGESTION_TYPES;

  return types.map((candidate) => ({
    id: String(candidate),
    label: INGESTION_TYPE_TO_LABEL_MAP[candidate],
    value: candidate,
  }));
};

export const getCursorRequirement = (
  validation: EdgeValidation | undefined,
  resource: string,
): Requirement | undefined =>
  getResourceValidation(validation, resource)?.requirements.find(
    (requirement) => requirement.kind === RequirementKind.CURSOR_COLUMN,
  );

export const getBlockingMessages = (validation: EdgeValidation | undefined): string[] => [
  ...(validation?.errors ?? []).map((error) => error.message),
  ...(validation?.requirements ?? [])
    .filter((requirement) => requirement.blocking)
    .map((requirement) => requirement.message),
  ...(validation?.resources ?? []).flatMap((entry) =>
    entry.requirements.filter((requirement) => requirement.blocking).map((r) => r.message),
  ),
];

export const sortCursorCandidates = (candidates: CandidateValue[]): CandidateValue[] =>
  [...candidates].sort((a, b) => {
    if (a.recommended !== b.recommended) return a.recommended ? -1 : 1;
    const rankA = a.rank || UNRANKED_CANDIDATE_RANK;
    const rankB = b.rank || UNRANKED_CANDIDATE_RANK;
    if (rankA !== rankB) return rankA - rankB;
    return a.value.localeCompare(b.value);
  });

export const getEdgeCursor = (edge: CanvasEdge, resource: string) =>
  getPipelineCanvasEdgeData(edge).cursors.find((cursor) => cursor.resource === resource) ?? null;
