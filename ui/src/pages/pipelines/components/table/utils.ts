import type { TableSort } from "@galaxy-io/dls/table/types";

import { SortBy, SortOrder } from "@/gen/ingestion/v1/sorting_pb";

import { DEFAULT_LIST_SORT_BY, DEFAULT_LIST_SORT_ORDER, type ListSearchParams } from "@/api/utils";

export type PipelinesTableSorting = TableSort | null;

export type PipelinesTableSortingChange = (sorting: PipelinesTableSorting) => void;

export const PIPELINES_TABLE_COLUMN_ID_PIPELINE = "pipeline";

const PIPELINES_TABLE_COLUMN_ID_TO_SORT_BY_MAP: Record<string, SortBy | undefined> = {
  [PIPELINES_TABLE_COLUMN_ID_PIPELINE]: SortBy.NAME,
};

const PIPELINES_TABLE_SORT_BY_TO_COLUMN_ID_MAP: Partial<Record<SortBy, string>> = {
  [SortBy.NAME]: PIPELINES_TABLE_COLUMN_ID_PIPELINE,
};

export const createPipelinesTableSorting = ({
  sortBy,
  sortOrder,
}: ListSearchParams): PipelinesTableSorting => {
  const id = PIPELINES_TABLE_SORT_BY_TO_COLUMN_ID_MAP[sortBy ?? DEFAULT_LIST_SORT_BY];
  if (!id) {
    return null;
  }
  return { columnId: id, isDescending: (sortOrder ?? DEFAULT_LIST_SORT_ORDER) === SortOrder.DESC };
};

export const createPipelinesTableSortSearch = (
  sorting: PipelinesTableSorting,
): Pick<ListSearchParams, "sortBy" | "sortOrder"> => {
  const sortBy = sorting ? PIPELINES_TABLE_COLUMN_ID_TO_SORT_BY_MAP[sorting.columnId] : undefined;
  if (!sorting || !sortBy) {
    return { sortBy: undefined, sortOrder: undefined };
  }
  const sortOrder = sorting.isDescending ? SortOrder.DESC : SortOrder.ASC;
  if (sortBy === DEFAULT_LIST_SORT_BY && sortOrder === DEFAULT_LIST_SORT_ORDER) {
    return { sortBy: undefined, sortOrder: undefined };
  }
  return { sortBy, sortOrder };
};
