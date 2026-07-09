import { useMemo } from "react";

import { PipelineGroup, PipelineListItem } from "@/pages/pipelines/types";
import { MOCK_PIPELINE_ITEMS } from "@/pages/pipelines/constants";
import { getPipelineGroup, toPipelineListItem } from "@/pages/pipelines/utils";

import { useListPipelinesQuery } from "@/api/queries/pipelines";

/**
 * Pipeline list items for the /pipelines view.
 *
 * Reads persisted pipelines from the API when the server is reachable and
 * falls back to design mocks while the backend list metadata (runs, volume,
 * schedules) is still being built out.
 */
export const usePipelineListItems = (): {
  items: PipelineListItem[];
  isLoading: boolean;
  isMockData: boolean;
} => {
  const { data, isLoading } = useListPipelinesQuery();

  const hasServerPipelines = !!data?.pipelines.length;

  const items = useMemo(() => {
    if (hasServerPipelines) {
      return data.pipelines.map(toPipelineListItem);
    }
    return MOCK_PIPELINE_ITEMS;
  }, [hasServerPipelines, data]);

  return { items, isLoading: isLoading && !hasServerPipelines, isMockData: !hasServerPipelines };
};

export const groupPipelineItems = (
  items: PipelineListItem[],
): Record<PipelineGroup, PipelineListItem[]> => {
  const groups: Record<PipelineGroup, PipelineListItem[]> = {
    [PipelineGroup.ACTIVE]: [],
    [PipelineGroup.NEEDS_ATTENTION]: [],
    [PipelineGroup.PAUSED]: [],
  };
  for (const item of items) {
    groups[getPipelineGroup(item)].push(item);
  }
  return groups;
};
