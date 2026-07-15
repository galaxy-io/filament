import { useMemo } from "react";

import { PipelineGroup, PipelineListItem } from "@/pages/pipelines/types";
import { getPipelineGroup, toPipelineListItem } from "@/pages/pipelines/utils";

import { useListPipelinesQuery } from "@/api/queries/pipelines";

/**
 * Pipeline list items for the /pipelines view.
 */
export const usePipelineListItems = (): {
  items: PipelineListItem[];
  isLoading: boolean;
  isError: boolean;
} => {
  const { data, isLoading, isError } = useListPipelinesQuery();

  const items = useMemo(() => {
    if (data?.pipelines.length) {
      return data.pipelines.map(toPipelineListItem);
    }
    return [];
  }, [data]);

  return {
    items,
    isLoading,
    isError,
  };
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
