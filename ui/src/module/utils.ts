import { createFilamentFlowSearch } from "@/module/paths";
import type { FilamentLayoutSearch } from "@/module/schemas";

export const createFilamentFlowCloseSearch = ({
  returnFlow,
  sourceId,
  sinkIds,
}: FilamentLayoutSearch) =>
  returnFlow
    ? createFilamentFlowSearch(returnFlow, { sourceId, sinkIds })
    : createFilamentFlowSearch();
