import { useMemo } from "react";

import { ExecutionMode, ReplicationMode } from "@/gen/ingestion/v1/common_pb";

import { getCanvasEdgeResource } from "@/pages/pipelines/canvas/graph/serialize";
import { usePipelineCanvasConnections } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasConnections";
import {
  PipelineCanvasValidationIssueKind,
  usePipelineCanvasValidation,
} from "@/pages/pipelines/canvas/hooks/usePipelineCanvasValidation";
import { usePipelineCanvasState } from "@/pages/pipelines/canvas/providers/canvas/PipelineCanvasProvider";
import type { PipelineCanvasRoute } from "@/pages/pipelines/canvas/routes/types";
import { getPipelineCanvasRouteGroupKey } from "@/pages/pipelines/canvas/routes/utils";
import type { CanvasEdge, CanvasNode } from "@/pages/pipelines/canvas/types";
import { getCanvasEdgeResourceLabel, getTransformStepCount } from "@/pages/pipelines/canvas/utils";
import { usePipelineExecutionMode } from "@/pages/pipelines/hooks/usePipelineExecutionMode";

import { isSearchMatch } from "@/utils/search";

interface PipelineCanvasRoutesOptions {
  search: string;
  sinkIds: CanvasNode["id"][];
}

type PipelineCanvasRouteDraft = Omit<PipelineCanvasRoute, "groupIndex" | "groupSize">;

const compareGroups = (
  left: PipelineCanvasRouteDraft[],
  right: PipelineCanvasRouteDraft[],
): number => {
  const leftIsInvalid = left.some((route) => route.isInvalid);
  const rightIsInvalid = right.some((route) => route.isInvalid);
  if (leftIsInvalid !== rightIsInvalid) return leftIsInvalid ? -1 : 1;
  return left[0].resourceLabel.localeCompare(right[0].resourceLabel);
};

const compareSinks = (left: PipelineCanvasRouteDraft, right: PipelineCanvasRouteDraft): number =>
  (left.sinkConnection?.name ?? left.edge.target).localeCompare(
    right.sinkConnection?.name ?? right.edge.target,
  );

export const usePipelineCanvasRoutes = ({ search, sinkIds }: PipelineCanvasRoutesOptions) => {
  const { edges } = usePipelineCanvasState();
  const connectionByNodeId = usePipelineCanvasConnections();
  const { issues, invalidEdgeIds, edgeValidationByEdgeId } = usePipelineCanvasValidation();
  const isContinuous = usePipelineExecutionMode() === ExecutionMode.CONTINUOUS;

  const routes = useMemo<PipelineCanvasRoute[]>(() => {
    const issuesByEdgeId = new Map<CanvasEdge["id"], string[]>();
    for (const issue of issues) {
      if (issue.edgeId === undefined || issue.kind !== PipelineCanvasValidationIssueKind.TRANSFORM)
        continue;
      issuesByEdgeId.set(issue.edgeId, [
        ...(issuesByEdgeId.get(issue.edgeId) ?? []),
        issue.message,
      ]);
    }

    const drafts = edges
      .filter((edge) => sinkIds.length === 0 || sinkIds.includes(edge.target))
      .map<PipelineCanvasRouteDraft>((edge) => {
        const resource = getCanvasEdgeResource(edge);
        const { label, isNamedResource } = getCanvasEdgeResourceLabel(resource, 0);
        const sourceConnection = connectionByNodeId.get(edge.source);
        return {
          edge,
          resource,
          resourceLabel: label,
          isNamedResource,
          sourceConnection,
          sinkConnection: connectionByNodeId.get(edge.target),
          groupKey: getPipelineCanvasRouteGroupKey(edge.source, resource),
          hasReadLevers: sourceConnection?.replication !== ReplicationMode.CDC && !isContinuous,
          transformStepCount: getTransformStepCount(edge.data?.transform, resource),
          issues: issuesByEdgeId.get(edge.id) ?? [],
          isInvalid: invalidEdgeIds.has(edge.id),
          verdict: edgeValidationByEdgeId.get(edge.id),
        };
      })
      .filter((draft) =>
        isSearchMatch(
          search,
          draft.sourceConnection?.name ?? "",
          draft.resourceLabel,
          draft.sinkConnection?.name ?? "",
        ),
      );

    const groups = new Map<PipelineCanvasRoute["groupKey"], PipelineCanvasRouteDraft[]>();
    for (const draft of drafts) {
      groups.set(draft.groupKey, [...(groups.get(draft.groupKey) ?? []), draft]);
    }

    return [...groups.values()]
      .map((group) => group.sort(compareSinks))
      .sort(compareGroups)
      .flatMap((group) =>
        group.map((draft, groupIndex) => ({ ...draft, groupIndex, groupSize: group.length })),
      );
  }, [
    edges,
    connectionByNodeId,
    issues,
    invalidEdgeIds,
    edgeValidationByEdgeId,
    isContinuous,
    search,
    sinkIds,
  ]);

  return { routes, hasRoutes: edges.length > 0 };
};
