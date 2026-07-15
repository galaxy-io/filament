import { useMemo } from "react";

import { PipelineGroup, PipelineResource } from "@/pages/pipelines/types";
import { getPipelineGroup, toPipelineResource } from "@/pages/pipelines/utils";

import { useListPipelinesQuery } from "@/api/queries/pipelines";

/**
 * Pipeline list items for the /pipelines view.
 */
export const usePipelineListItems = (): {
  items: PipelineResource[];
  isLoading: boolean;
  isError: boolean;
} => {
  const { data, isLoading, isError } = useListPipelinesQuery();

  const items = useMemo(() => {
    if (data?.pipelines.length) {
      return data.pipelines.map(toPipelineResource);
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
  items: PipelineResource[],
): Record<PipelineGroup, PipelineResource[]> => {
  const groups: Record<PipelineGroup, PipelineResource[]> = {
    [PipelineGroup.ACTIVE]: [],
    [PipelineGroup.NEEDS_ATTENTION]: [],
    [PipelineGroup.PAUSED]: [],
  };
  for (const item of items) {
    groups[getPipelineGroup(item)].push(item);
  }
  return groups;
};
